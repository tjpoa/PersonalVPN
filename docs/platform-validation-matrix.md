# Matriz de validação por plataforma

Use a mesma versão da API, gateway e perfil WireGuard em todos os testes. Cada execução deve guardar versão do commit, sistema operativo, modelo, rede usada, timestamp e resultado; nunca guardar chaves privadas, destinos visitados ou conteúdo de tráfego.

| Plataforma | Runner e ferramentas | Dispositivo mínimo | Gate obrigatório |
|---|---|---|---|
| Windows 10/11 | Windows x64, .NET 8 SDK, Windows SDK, WireGuard embeddable service, PowerShell | VM limpa x64; testar também ARM64 antes do lançamento | Instalação/remoção limpa, UAC, serviço ACL, sleep/resume, IPv4+IPv6 e kill switch |
| Android | Android Studio estável, JDK 17, SDK 35, Gradle Wrapper, `adb` | Pixel/emulador API 26 e aparelho físico API 35 | `VpnService.prepare`, Doze, Always-on/lockdown, Wi-Fi↔LTE, DNS/IPv6 leak e rotação de chave |
| Android TV | Mesmo SDK Android + Compose for TV, `adb` e comando remoto | Emulador TV API 35 e TV física/Google TV | Navegação D-pad sem touchscreen, foco/overscan, suspensão/retoma, troca de rede e login por código |
| iOS/iPadOS | macOS, Xcode fixado, Swift Package Manager, Apple Developer Team | iPhone/iPad físico em Wi-Fi e rede móvel | Consentimento, `NEPacketTunnelProvider`, background/kill, `includeAllNetworks`, DNS/IPv6 leak e App Store archive |

## Estado atual

O AVD Pixel 8 API 35 passou autenticação HTTPS, registo de dispositivo, provisionamento e arranque do serviço Android. Em `emulator-5554`, `scripts/test-android.ps1` passou o build unitário e a instrumentação (`AuthTokenStoreTest`). Um gateway WireGuard temporário em WSL2 passou handshake real e ping pelo `tun0` (`10.77.0.4` → `10.77.0.1`, 0% de perda). Com forwarding/NAT no WSL, `1.1.1.1` e `example.com` responderam pelo túnel e os contadores de encaminhamento confirmaram o tráfego. Isto valida o caminho Android↔gateway↔Internet em laboratório; kill switch, hardware físico e repetição após reinício continuam pendentes. Windows, iOS e Android TV permanecem gates de integração pendentes.

## Fluxo comum

1. Registar um dispositivo novo; confirmar que somente a chave pública chega à API.
2. Obter configuração, ligar o túnel e verificar handshake recente no gateway.
3. Testar resolução DNS, IPv4 e IPv6 externos; com kill switch, bloquear todo tráfego quando o túnel cai.
4. Mudar de rede, suspender/reiniciar e revogar o dispositivo; confirmar que a revogação converge e não há reconexão.
5. Desinstalar e reinstalar; confirmar ausência de rotas, adaptadores, perfis e segredos órfãos.

## Evidência e bloqueios

Um cenário só passa com logs locais redigidos, captura de métricas (latência, throughput, CPU/bateria) e resultado reproduzível duas vezes. Falha de DNS, IPv6, kill switch, revogação ou limpeza de instalação bloqueia o release. Simuladores/emuladores não substituem testes de encaminhamento em hardware real; o simulador iOS não prova o comportamento da Network Extension.
