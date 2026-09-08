# Plano de teste em hardware

Os simuladores validam UI e serialização, mas não provam encaminhamento, suspensão ou fugas de rede. Antes de beta, reserve uma matriz física e repita cada cenário duas vezes com logs redigidos.

## Laboratório mínimo

| Alvo | Equipamento | Condições obrigatórias |
|---|---|---|
| Gateway | 1–2 VMs ou servidores Linux LTS | WireGuard kernel, `nftables`, IPv4/IPv6, DNS privado, relógio sincronizado |
| Windows | Windows 10/11 x64 VM limpa | Conta padrão + administrador, UAC, sleep/resume, Wi‑Fi e Ethernet |
| Android | Telefone Android API 35 e um aparelho Android 12–14 | Wi‑Fi, LTE/5G, Doze, always-on e lockdown |
| Android TV | Dispositivo/box Android TV real | D-pad, landscape, sem touchscreen, suspensão/retoma e troca de rede |
| iOS | iPhone/iPad físico suportado | Wi‑Fi/móvel, consentimento VPN, background/kill e Network Extension |

Use uma rede de gestão separada da rede de tráfego. O gateway deve ser descartável; não reutilize credenciais de produção. Registe versão do SO, versão do cliente, commit, região, MTU e horário UTC em cada execução.

## Casos e evidências

1. Ligar/desligar: handshake recente, bytes nos dois sentidos e estado correto na UI.
2. DNS/IPv4/IPv6: resolver dentro do túnel; consultas externas e IPv6 sem túnel devem falhar quando o kill switch estiver ativo.
3. Rede instável: trocar Wi‑Fi↔móvel, bloquear UDP e reiniciar gateway; cliente deve reconectar ou declarar falha, nunca mostrar ligação falsa.
4. Revogação: remover peer e confirmar que o tráfego cessa, a configuração é apagada e o cliente não reconecta.
5. Limpeza: desinstalar/parar e verificar ausência de adaptadores, rotas, DNS, serviços e perfis residuais.

Guarde apenas métricas agregadas (latência, throughput, CPU/bateria, idade do handshake e perda de pacotes). Não recolha destinos, conteúdo, chaves ou capturas de tráfego. Marque cada caso `PASS`/`FAIL` e anexe comandos, timestamps e versões ao relatório de release.
