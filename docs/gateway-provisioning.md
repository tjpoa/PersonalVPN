# Provisionamento de gateway Linux

Use uma VM ou servidor Linux LTS descartável por região. Não execute estes comandos no computador de desenvolvimento nem em gateways que contenham dados não relacionados.

## Base mínima

- Kernel LTS com WireGuard nativo, `wireguard-tools`, `nftables`, `systemd`, `ca-certificates` e `dnsmasq`/resolved privado.
- Uma interface pública UDP (por defeito `51820`) e uma interface de gestão separada; PostgreSQL nunca fica acessível pelo gateway.
- Relógio sincronizado por NTP, hostname estável, firewall do fornecedor em default-deny e acesso administrativo por bastion/SSH com chave curta.

## Rede e firewall

Defina sub-redes privadas não sobrepostas por região e endereços `/32` e `/128` por dispositivo. Ative forwarding com ficheiro versionado em `/etc/sysctl.d/` (`net.ipv4.ip_forward=1` e `net.ipv6.conf.all.forwarding=1`). A política `nftables` deve aceitar apenas WireGuard→uplink, respostas `established,related` e DNS para o resolvedor privado; bloqueie o restante e faça NAT IPv4 apenas quando necessário. IPv6 deve ser encaminhado sem NAT ou bloqueado de forma explícita — nunca deixar fallback público silencioso.

## Instalação do agent

1. Verificar assinatura/hash do binário e instalar `/opt/personalvpn/personalvpn-gateway-agent` como `root:root`, modo `0755`.
2. Instalar os certificados mTLS e a chave pública de snapshots fora do Git, com permissões mínimas.
3. Instalar a unidade [systemd](systemd/README.md), executar `systemd-analyze security` e iniciar apenas depois de testar `wg show`.
4. Confirmar que o agent consegue alcançar apenas o endpoint HTTPS da API e que a porta de gestão não está na interface pública.

## Observabilidade e recuperação

Recolha apenas idade do handshake, bytes agregados, CPU, memória, fila de snapshots e estado do serviço. Configure alertas para ausência de handshake, expiração de certificados, outbox crescente, pool de endereços e falhas de `nftables`. Mantenha a imagem/VM reproduzível: em incidente, substituir o gateway e reconciliar snapshots é preferível a reparar manualmente um host comprometido.
