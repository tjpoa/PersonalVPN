# Clientes por plataforma

Os clientes devem usar o protocolo WireGuard oficial da plataforma; não devem implementar criptografia ou o túnel em código próprio. Cada cliente gera a chave privada localmente, envia apenas a chave pública para a API e guarda o perfil no armazenamento seguro do sistema.

| Plataforma | Integração nativa | UI/runtime | Armazenamento obrigatório |
|---|---|---|---|
| Windows 10/11 | WireGuardNT ou serviço oficial WireGuard | WinUI 3 + serviço Windows | DPAPI/Windows Credential Manager |
| Android | `VpnService` + `com.wireguard.android:tunnel` (`GoBackend`) | Kotlin/Jetpack Compose | Android Keystore |
| Android TV | `VpnService` compatível com TV | Compose for TV, foco remoto | Android Keystore |
| iOS | Network Extension (`NEPacketTunnelProvider`) | SwiftUI + extensão separada | Keychain, App Groups |

## Contrato comum

1. Autenticar utilizador e obter sessão curta; guardar refresh token no cofre do sistema.
2. Registar dispositivo com `platform` e chave pública WireGuard.
3. Solicitar configuração com uma chave de idempotência aleatória (16–128 caracteres).
4. Validar endpoint, chave pública, endereços host e DNS antes de instalar o túnel.
5. Mostrar estado `connecting`, `connected`, `reconnecting` ou `disconnected`; nunca expor chaves privadas em logs.

O módulo inicial Android está em `clients/android`: inclui a base Gradle, ciclo de vida `VpnService`, cliente HTTPS e cifragem de material privado com Android Keystore. A geração Curve25519 e o transporte do túnel devem ser delegados ao binding WireGuard oficial; sem esse binding o serviço falha fechado. A compilação ainda requer Android Studio/SDK local. O esqueleto Windows está em `clients/windows` (manager IPC do WireGuardNT ainda por ligar) e a preparação iOS em `clients/ios`. Builds Windows exigem .NET SDK/Windows; builds iOS exigem Xcode num runner macOS.
