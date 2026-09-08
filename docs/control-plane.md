# Plano de controlo e reconciliação

## Princípios

O PostgreSQL representa o **estado desejado**; cada gateway-agent reporta o **estado observado**. A API nunca executa `wg` diretamente nem espera que todos os gateways estejam online para responder. Operações são transacionais, idempotentes e sempre limitadas ao proprietário do dispositivo.

O esquema inicial está em `services/api/migrations/0001_initial.sql`. Chaves privadas, destinos, consultas DNS e conteúdo não têm coluna no modelo.

Estado de implementação: a migração é testada em PostgreSQL 17 e o adapter em `services/api/internal/postgres/` usa `pgx` v5.10.0. Já existem operações reais para criar utilizadores internos, registar/listar dispositivos por proprietário, pedir revogação, alocar leases/assignments dual-stack e materializar snapshots assinados na outbox. Ligações remotas com TLS desativado são rejeitadas antes do dial. Argon2id, sessões opacas, handlers de autenticação, configuração/runtime, executor tipado, worker e endpoints mTLS do agent estão implementados; falta operação automatizada de revogação e rotação de certificados.

## Fluxo de registo e configuração

1. O cliente gera o par WireGuard no armazenamento seguro da plataforma.
2. `POST /devices` envia nome, plataforma e chave pública.
3. `POST /devices/{id}/configuration` bloqueia o dispositivo e a região numa transação.
4. O serviço valida propriedade, estado ativo, quota e `Idempotency-Key`.
5. Aloca IPv4 /32 e IPv6 /128 únicos dentro dos pools da região.
6. Escolhe um gateway `ready` com capacidade e cria `peer_assignments(pending)`.
7. Um reconciliador assina a visão atual com a chave fora da base e chama `EnqueueCurrentGatewaySnapshot`.
8. Commit ocorre antes de devolver endpoint, chave pública do gateway, endereços, DNS e rotas.
9. O `ack` chama `ConfirmGatewaySnapshot`: promove assignments, conclui remoções, liberta leases e marca dispositivos revogados.

Para uma publicação administrativa explícita, use `cmd/snapshot-publisher` com `DATABASE_URL`, `GATEWAY_ID`, `SNAPSHOT_GENERATION` e `SNAPSHOT_SIGNING_KEY_FILE`. O ficheiro deve conter apenas a chave Ed25519 em Base64, ser montado por um gestor de secrets e nunca ser incluído na imagem ou nos logs. Em staging/produção, execute `cmd/reconciler` como uma única réplica por base de dados; ele publica automaticamente snapshots para gateways `ready` com atribuições ainda não observadas. Configure `RECONCILE_INTERVAL` (por exemplo, `15s`) e supervise o processo pelo seu código de saída.
8. O agent aplica o peer e avança `observed_generation`; o scheduler marca a atribuição `active`.

Se uma chave idempotente for repetida com o mesmo request hash, devolve a resposta guardada. Se o conteúdo divergir, responde `409`. A chave original nunca é guardada: apenas um hash com expiração.

## Reconciliação do gateway

O agent inicia ligação mTLS de saída ao plano de controlo, evitando uma porta administrativa pública. A cada ciclo:

- Envia `gateway_id`, geração observada, capacidade, versão e health.
- Recebe snapshot assinado de peers desejados: chave pública, /32, /128 e geração.
- Valida schema, assinatura, gateway e monotonicidade da geração.
- Calcula diff; adiciona/atualiza antes de remover quando não cria colisão.
- Aplica uma configuração atómica e relê o estado do kernel.
- Confirma hash/generation observados, sem enviar contadores por peer para analytics.

Comandos arbitrários, paths, shell fragments, DNS ou regras `nftables` nunca fazem parte deste protocolo. O agent aceita somente operações tipadas sobre peers.

## Revogação

`DELETE /devices/{id}` muda o dispositivo para `revocation_pending` e a atribuição para `removal_pending` na mesma transação. Todos os agents relevantes recebem a remoção. Quando confirmada, a atribuição fica `removed`, o lease recebe `released_at` e o dispositivo fica `revoked`.

O endereço libertado entra numa quarentena antes de reutilização. Se um gateway estiver offline, a revogação permanece pendente e esse gateway não pode voltar a `ready` até reconciliar um snapshot completo. A UI deve distinguir “revogação pedida” de “revogação confirmada”.

## Concorrência e invariantes

- Uma chave pública pertence a exatamente um dispositivo.
- Um dispositivo tem no máximo um lease e uma atribuição vivos.
- IPv4 e IPv6 ativos são únicos por região.
- `observed_generation` nunca supera `desired_generation`.
- Um gateway `draining` não recebe novas atribuições.
- Queries de utilizador incluem `user_id` derivado do token, nunca do body.
- `404` é usado para recurso inexistente ou de outro proprietário, reduzindo enumeração.
- Revogação, alocação e consumo de quota usam transação e row locks.

As constraints SQL são defesa adicional, não substituem validação e autorização na aplicação.

## Falhas e recuperação

| Falha | Comportamento |
|---|---|
| API cai após commit | Retry idempotente devolve a mesma atribuição |
| Agent offline | Estado permanece pending; região reduz capacidade |
| Aplicação parcial no kernel | Relê estado, não confirma geração e reaplica snapshot |
| Gateway perde disco | Reprovisiona identidade e recebe snapshot completo |
| DB indisponível | API falha fechado; gateways mantêm peers já aplicados |
| Colisão de IP | Unique constraint aborta; allocator escolhe outro endereço |
| Reuse de refresh token | Revoga toda a família e exige novo login |

## Métricas operacionais

Permitidas: número agregado de peers por gateway, atraso de reconciliação, erros por código, disponibilidade, CPU/memória, bytes totais do gateway e saturação de pools. Proibidas: séries por destino, domínio, aplicação ou conteúdo. Identificadores por dispositivo só aparecem em auditoria operacional com acesso e retenção limitados.

Objetivos iniciais: p95 da API abaixo de 300 ms sem reconciliação; revogação confirmada abaixo de 30 s com gateways online; snapshot completo após recuperação abaixo de 60 s; nenhuma atribuição acima da capacidade declarada.
