# Laboratório WireGuard reproduzível

## Objetivo

Provar o plano de dados antes de criar aplicações: um gateway Linux e dois clientes devem estabelecer túneis, navegar por IPv4/IPv6, resolver DNS dentro do túnel, sobreviver à troca de endpoint e perder acesso após revogação.

Este documento é uma especificação, não um script para executar cegamente. Os passos de forwarding, NAT, rotas e firewall **alteram a rede** e devem correr apenas em VMs Linux descartáveis, nunca diretamente no computador de trabalho.

O harness inicial em `tests/e2e/lab/` cria quatro namespaces dentro de um contentor Docker privilegiado e uma Internet sintética. Execute a partir da raiz com `powershell -File scripts/run-lab.ps1`. O contentor é removido no fim e escreve apenas um resumo redigido em `tests/e2e/results/wireguard-lab.txt`. Reveja o script antes de o executar: `--privileged` é necessário para criar interfaces e namespaces.

Estado atual: a execução isolada de 2026-09-04 passou L01, L01B, L02, L04, L08 e L02B no kernel WSL2 6.18.33.2. O primeiro harness automatiza estes seis casos; L03 e L05–L12 continuam pendentes e exigem execução/revisão adicional.

## Topologia

```text
client-a 10.70.0.2/32, fd70::2/128 ─┐
                                     ├─ UDP 51820 ─ gateway ─ Internet/DNS
client-b 10.70.0.3/32, fd70::3/128 ─┘

management: rede separada, sem encaminhamento para o túnel
```

Usar três VMs Ubuntu/Debian LTS ou namespaces Linux dentro de uma VM. O gateway precisa de duas NICs: Internet/laboratório e gestão. Guardar chaves apenas em `/etc/wireguard` com permissões `0600` e gerar novas a cada reconstrução.

## Pré-requisitos

- Hipervisor com snapshots e uma VM Linux de controlo.
- No Windows, `docker-desktop` sozinho não é uma distro de trabalho: usar uma VM Ubuntu/Debian ou instalar uma distro WSL2 dedicada antes de executar ferramentas Linux interativas.
- `wireguard-tools`, `iproute2`, `nftables`, `curl`, `dig`, `tcpdump` e `iperf3`.
- IPv6 real ou uma segunda rede do laboratório para validar encaminhamento sem fingir cobertura.
- Captura fora do túnel para provar ausência de DNS e tráfego em claro.

## Sequência de implementação

1. Criar as redes e confirmar que clientes alcançam o IP público do gateway por UDP.
2. Gerar chaves com `umask 077` e `wg genkey`; nunca copiar chaves privadas para notas ou Git.
3. Criar `wg0`, atribuir `10.70.0.1/24` e `fd70::1/64`, adicionar peers com /32 e /128 exclusivos.
4. Ativar forwarding temporariamente na VM e definir `nftables` com política default-deny.
5. Permitir UDP 51820, tráfego established/related, encaminhamento `wg0`↔uplink e masquerade IPv4 apenas quando necessário.
6. Configurar clientes com `AllowedIPs = 0.0.0.0/0, ::/0`, DNS do túnel e `PersistentKeepalive = 25` apenas quando o NAT o exigir.
7. Capturar nas NICs física e virtual durante ligação, DNS, navegação, reconexão e revogação.
8. Destruir as VMs e confirmar que a reconstrução produz o mesmo resultado sem reutilizar segredos.

## Matriz de testes

| Caso | Ação | Resultado obrigatório |
|---|---|---|
| L01 | `wg show` após handshake | Handshake recente e contadores nos dois sentidos |
| L02 | Consultar endereço IPv4 externo | Saída corresponde ao gateway |
| L03 | Consultar endereço IPv6 externo | Saída corresponde ao gateway; caso contrário, IPv6 bloqueado sem fuga |
| L04 | Resolver nome e capturar uplink do cliente | Nenhum pacote DNS fora do túnel |
| L05 | Reduzir MTU/testar payloads | Sem black hole; valor funcional documentado |
| L06 | Reiniciar gateway | Clientes recuperam sem regenerar chave |
| L07 | Trocar cliente de rede/NAT | Reconecta dentro do objetivo de tempo definido |
| L08 | Remover peer no gateway | Tráfego do dispositivo cessa e não recupera |
| L09 | Duplicar `AllowedIPs` | Configuração é rejeitada pela reconciliação |
| L10 | Desligar DNS interno | Cliente falha fechado ou apresenta erro explícito, sem fallback público |
| L11 | Bloquear UDP | Cliente reporta diagnóstico correto, sem declarar ligação falsa |
| L12 | `iperf3` durante 10 minutos | Throughput/CPU/packet loss registados como baseline |

## Evidência a guardar

Guardar somente resultados sem segredos em `tests/e2e/results/`: versão do kernel/WireGuard, topologia, hashes dos scripts, latência, throughput, handshake/revogação e resumo redigido das capturas. Não guardar `.conf`, chaves, IP público pessoal ou PCAP de tráfego real.

O laboratório fica concluído quando L01–L12 passam numa execução limpa e repetida. Só então se automatizam os mesmos invariantes no gateway-agent.
