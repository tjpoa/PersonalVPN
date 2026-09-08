# Build iOS em macOS

Este procedimento é executado apenas num Mac controlado; o Windows não consegue compilar nem assinar `NetworkExtension`.

## Pré-requisitos

- macOS suportado pela versão fixada de Xcode e um Apple Developer Team ativo.
- Xcode Command Line Tools, Swift Package Manager e certificados de distribuição no Keychain.
- App ID com a capacidade `Network Extension` e App Group; perfis separados para a app e a extensão.
- SDK/binding WireGuardKit aprovado para a versão alvo. Fixar a revisão no projeto e rever licenciamento antes de distribuir.

## Targets e signing

Criar os targets `PersonalVPN` (app SwiftUI) e `PersonalVPNPacketTunnel` (App Extension). A extensão deve usar `NEPacketTunnelProvider`, a extensão point `com.apple.networkextension.packet-tunnel` e o entitlement `packet-tunnel-provider`. Os dois targets partilham apenas o estado mínimo necessário pelo App Group; chaves privadas ficam no Keychain `ThisDeviceOnly`.

## Validação local

```bash
xcodebuild -scheme PersonalVPN -configuration Debug \
  -destination 'platform=iOS Simulator,name=iPhone 15' test
xcodebuild -scheme PersonalVPN -configuration Release \
  -archivePath build/PersonalVPN.xcarchive archive
xcodebuild -exportArchive -archivePath build/PersonalVPN.xcarchive \
  -exportOptionsPlist ExportOptions.plist -exportPath build/export
```

O simulador valida apenas modelos, renderer e UI. Testar `startTunnel`, consentimento, mudança Wi‑Fi↔móvel, suspensão, revogação, DNS/IPv6 e remoção num iPhone físico. Guardar no artefacto de CI apenas logs redigidos, IPA assinado e checksum; nunca perfis, certificados ou chaves.
