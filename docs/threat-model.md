# Modelo de ameaças

## Escopo e ativos

O sistema inclui clientes, API de controlo, PostgreSQL, gateway-agent, gateways WireGuard, CI/CD e contas cloud. Os ativos prioritários são: chaves privadas dos dispositivos e gateways, credenciais e tokens, disponibilidade do túnel, integridade das configurações, dados pessoais e confidencialidade dos destinos do utilizador.

O modelo assume que a Internet, Wi-Fi local, dispositivo perdido e fornecedor de alojamento podem ser hostis. WireGuard protege entre cliente e gateway; não protege tráfego já saído do gateway nem corrige uma aplicação cliente ou API comprometida.

## Fronteiras de confiança

1. UI sem privilégios ↔ serviço/extensão VPN privilegiado.
2. Cliente ↔ API HTTPS.
3. Cliente ↔ gateway WireGuard UDP.
4. API/scheduler ↔ gateway-agent.
5. Serviços ↔ PostgreSQL/gestor de segredos.
6. CI/CD ↔ lojas, assinatura de código e infraestrutura cloud.
7. Gateway ↔ Internet pública e resolvedor DNS.

## Ameaças e controlos obrigatórios

| ID | Cenário | Impacto | Controlos e prova esperada |
|---|---|---|---|
| T01 | Roubo da chave privada do cliente | Personificação do dispositivo | Keystore/Keychain/DPAPI, chave não exportável quando possível, revogação; teste comprova ausência na API e logs |
| T02 | API entrega endpoint ou chave de servidor falsos | Interceção/indisponibilidade | TLS, autenticação forte, resposta vinculada à conta, auditoria e rotação controlada; testes de alteração e replay |
| T03 | IDOR permite gerir dispositivo alheio | Sequestro de sessão | Autorização por proprietário em todas as queries, IDs opacos; testes negativos entre duas contas |
| T04 | Token roubado/reutilizado | Controlo da conta | Access token curto, refresh rotativo em hash, deteção de reuse, revogação por família; teste de replay |
| T05 | Fuga durante ligação/reconexão | Exposição de IP, DNS ou IPv6 | Rotas atómicas, kill switch, DNS no túnel, cobertura IPv4/IPv6; captura externa em cada transição |
| T06 | IPC local aceita utilizador não autorizado | Alteração de rede/privilégios | ACL por utilizador, protocolo autenticado, comandos allowlist, serviço sem shell; teste com conta Windows secundária |
| T07 | Gateway comprometido | Observação de tráfego e movimento lateral | Imagem mínima, default-deny, sem DB, gestão isolada, credenciais temporárias, reconstrução automática |
| T08 | Agent recebe comando forjado | Peer malicioso no gateway | mTLS/workload identity, operações assinadas, nonce/idempotência, allowlist de campos e reconciliação |
| T09 | Logs/telemetria revelam atividade | Violação de privacidade | Sem DNS/destinos/conteúdo, schema allowlist, retenção curta, acessos auditados; testes automáticos de redaction |
| T10 | Dependência ou pipeline comprometido | Release maliciosa | Dependências fixadas, SBOM, provenance, runners isolados, revisão e assinatura em hardware/serviço protegido |
| T11 | Atualização falsa ou downgrade | Execução de código | Manifesto assinado, TLS, versão mínima, rollback apenas assinado; testes de assinatura inválida/downgrade |
| T12 | Abuso/DoS da API ou UDP | Indisponibilidade/custo | Rate limits por identidade/IP, quotas, proteção upstream, limites por conta e capacidade observável |
| T13 | Colisão ou atribuição IP incorreta | Tráfego entregue ao peer errado | Lease transacional único, reconciliação, `AllowedIPs` /32 e /128; teste concorrente de alocação |
| T14 | Backup expõe base de dados | Dados pessoais comprometidos | Cifra com chave separada, acesso mínimo, retenção e restore auditado; exercício periódico de restauro |
| T15 | Operador cloud observa saída | Correlação do utilizador | Minimização de metadados, regiões transparentes e política honesta; não prometer anonimato absoluto |

## Dados permitidos e proibidos

Permitidos, com finalidade e retenção definidas: ID interno, email normalizado, chave pública, tipo/versão do dispositivo, região escolhida, estado da subscrição, evento administrativo e métricas agregadas de capacidade.

Proibidos por defeito: chave privada, configuração completa contendo segredo, URLs, domínios DNS, conteúdo, histórico de navegação, payload de pacotes e captura de tráfego. IP de origem só pode ser registado para segurança com truncagem, acesso restrito e expiração explícita.

## Critérios de saída de segurança

- Threat model revisto em cada alteração de fronteira ou dado recolhido.
- Nenhum segredo em Git, imagem, logs, crash reports ou artefactos CI.
- Testes de fuga passam em todas as plataformas e mudanças de rede.
- Revogação e rotação funcionam mesmo com um gateway indisponível.
- Pentest independente sem achados críticos/altos abertos antes de lançamento público.
- Incidente simulado cobre contenção, rotação, comunicação e recuperação.

Risco residual aceite pelo proprietário deve ficar num ADR com responsável, prazo e compensações; não pode ser aceite informalmente num issue.
