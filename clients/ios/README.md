# Cliente iOS

O cliente iOS deve ser uma aplicação SwiftUI com uma extensão separada `NEPacketTunnelProvider`. A aplicação coordena autenticação, pedido de configuração e consentimento; a extensão mantém o túnel enquanto o processo da UI está suspenso.

Requisitos antes do primeiro build:

- Apple Developer Team e entitlement Network Extension (`packet-tunnel-provider`).
- App Group partilhado para configuração cifrada e estado mínimo.
- Chave privada WireGuard criada no dispositivo e guardada no Keychain; nunca enviada à API.
- Integração com o SDK/binding WireGuard aprovado para iOS, sujeito às regras de distribuição da App Store.

O núcleo Swift Package está em `Package.swift`/`Sources/PersonalVPNClient`; ele cobre autenticação HTTPS, lista de dispositivos/regiões, modelos Codable, renderer WireGuard e o boundary do tunnel. `PacketTunnelProvider.swift` fornece uma extensão fail-closed até o target assinado injetar WireGuardKit. O projeto Xcode e os perfis de assinatura não são gerados neste ambiente Windows; os artefactos de distribuição devem ser criados e validados num runner macOS controlado.
`AuthTokenKeychain` guarda a sessão em Keychain `ThisDeviceOnly`, separada da identidade WireGuard.

`WireGuardKeychain` gera 32 bytes com `SecRandomCopyBytes` e guarda-os como Generic Password com `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`; a chave privada não entra na API. A derivação da chave pública e o uso no `NEPacketTunnelProvider` devem ser ligados ao WireGuardKit no target iOS.
Ao terminar sessão ou confirmar revogação, chamar `delete()` para remover o item do Keychain; a ausência do item já é tratada como sucesso.

## Entitlements e signing

Crie dois targets assinados separadamente: a app e uma extensão `Network Extension` com `NEPacketTunnelProvider`. A extensão precisa de `com.apple.developer.networking.networkextension` contendo `packet-tunnel-provider`; ambos os targets devem partilhar o mesmo App Group (`group.<identificador-da-app>`) para estado mínimo e Keychain access group. Gere perfis de provisioning distintos para Debug e Release e confirme Team ID, bundle IDs e App Group antes de chamar `NETunnelProviderManager`.

Os ficheiros `Resources/*.example` são templates para o target da extensão. Substitua apenas os identificadores no projeto Xcode/secret manager; não transforme estes exemplos em perfis assinados nem os preencha com Team IDs no Git.

Não inclua certificados, provisioning profiles ou identificadores reais no repositório. O pipeline macOS deve injetá-los por secret manager e executar `xcodebuild -scheme <app> -configuration Release archive`, exportando apenas IPA e extensão assinados.
