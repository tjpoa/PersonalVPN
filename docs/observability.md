# Observabilidade sem dados de tráfego

## Métricas permitidas

Exponha métricas agregadas, com retenção curta e acesso restrito:

- `personalvpn_api_requests_total{route,method,status}` e latência por rota;
- `personalvpn_gateway_handshake_age_seconds{gateway_id}`;
- `personalvpn_gateway_peers{gateway_id,state}` e capacidade disponível;
- `personalvpn_snapshot_deliveries_total{gateway_id,result}` (claim/ack/retry/failure);
- `personalvpn_ip_leases{region,state}`;
- `personalvpn_tunnel_bytes_total{platform,direction}` e `personalvpn_tunnel_packet_loss_ratio` quando medidos no cliente/gateway.

Não use como labels: utilizador, email, device name, chave pública, endereço IP, endpoint, domínio, destino, consulta DNS ou conteúdo. IDs de gateway/região devem ser internos e não conter dados pessoais.

## Logs e alertas

Registe eventos estruturados de autenticação falhada, revogação, reconciliação, rotação de certificados e erro de túnel, sempre com request ID e timestamp UTC. Redija tokens, chaves, URLs completas e payloads. Alertar para readiness indisponível, idade de handshake acima do SLO, filas de snapshots acumuladas, esgotamento de leases, falhas repetidas de ack e divergência de capacidade.

## Operação

Mantenha dashboards separados para API, gateways e clientes; não permita que uma consulta de métricas revele atividade individual. Defina `<retenção aprovada>`, `<SLO de disponibilidade>`, `<limiar de alerta>` e responsável de plantão antes do beta. Teste que logs e métricas continuam redigidos durante falhas, revogação e recuperação de backup.
