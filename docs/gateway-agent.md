# Gateway-agent: protocolo e limites

O primeiro contrato do agent está em `services/api/internal/agent/`. O plano de controlo envia snapshots de peers assinados com Ed25519; o agent valida identidade do gateway, geração, expiração, chaves WireGuard, endereços host IPv4/IPv6 e unicidade antes de calcular um diff. O transporte HTTP exige uma URL HTTPS e deve receber um `http.Client` configurado por `ClientTLSConfig`.

`cmd/gateway-agent` é o entrypoint executável. Requer `CONTROL_API_URL`, `GATEWAY_ID`, `WG_INTERFACE`, `SNAPSHOT_VERIFY_KEY` (chave Ed25519 pública em Base64), `AGENT_TLS_CERT_FILE`, `AGENT_TLS_KEY_FILE`, `AGENT_TLS_CA_FILE` e `AGENT_TLS_SERVER_NAME`. Usa TLS 1.3, verifica cada snapshot antes de chamar `wg` e termina em SIGTERM/SIGINT. A imagem Docker é o target `gateway-agent`; execute-a apenas com acesso controlado ao namespace de rede do gateway, nunca no host de desenvolvimento.

Snapshots devem ser duráveis em `gateway_snapshots`, uma outbox com geração única, tentativas limitadas e estados `queued`/`in_flight`/`acked`/`failed`. A ligação deve usar certificados de cliente e servidor verificados com TLS 1.3; `InsecureSkipVerify` não é permitido. O agent pode repetir mensagens, mas só confirma uma geração depois de reler o estado aplicado.

## Formato

Um `Snapshot` contém `gatewayId`, `generation`, `expiresAt`, peers e assinatura. Cada peer contém apenas chave pública e os dois endereços atribuídos. A assinatura cobre o JSON sem o próprio campo `signature`, evitando ambiguidade. A chave pública de assinatura deve ser provisionada fora da API de utilizadores e rodada por procedimento administrativo.

## Fluxo seguro

1. Agent autentica-se por mTLS/workload identity numa ligação de saída.
2. Faz `GET /v1/agent/gateways/{gatewayId}/snapshots/next`; `204` significa que não há trabalho.
3. Recebe bytes do snapshot e verifica assinatura, gateway e prazo.
4. Recusa gerações menores ou iguais à última aplicada.
5. Calcula operações determinísticas `add`, `update` e `remove`.
6. Um executor WireGuard separado traduz apenas essas operações tipadas para a biblioteca/serviço oficial.
7. Confirma em `/ack` após sucesso ou agenda `/retry` após falha.

O protocolo, o worker e o executor tipado estão em `snapshot.go`, `worker.go` e `wireguard.go`. `Worker.Run` faz polling contínuo, respeita cancelamento e aplica backoff limitado; o worker só confirma após aplicar toda a geração e devolve falhas à outbox para retry. O pacote não abre sockets nem altera interfaces por si; o `ProcessRunner` invoca apenas o binário fixo `wg` com argumentos validados, sem shell. A aplicação real ainda precisa de ACLs de serviço, limites de recursos, logs redigidos e uma estratégia atómica de aplicação antes de ser ligada a gateways reais.

## Invariantes

- Não aceitar peers duplicados por chave, IPv4 ou IPv6.
- Não aceitar prefixos que não sejam host (`/32` ou `/128`).
- Não aplicar snapshot expirado ou destinado a outro gateway.
- Não incluir endereços no comando `remove`.
- Não permitir que um erro parcial seja confirmado como geração observada.
- Não enviar tráfego, DNS, destinos ou contadores individuais para o plano de controlo.

Os testes em `snapshot_test.go`, `worker_test.go` e `wireguard_test.go` cobrem assinatura, adulteração, expiração, duplicação, diff determinístico, retries, rejeição de nomes tipo shell, argumentos fixos e paragem após falha. Ainda falta um teste de integração do executor contra uma VM Linux descartável.
