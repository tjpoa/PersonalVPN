# Android TV

O Android TV reutiliza o módulo de autenticação, provisionamento e `VpnTunnelService` de `clients/android`; a UI deve ser tratada como um alvo de apresentação separado.

## Configuração

- Manter `android.software.leanback` e `android.hardware.touchscreen` como features não obrigatórias para permitir instalação em TV e telemóvel.
- Publicar banner TV, launcher `LEANBACK_LAUNCHER` e layout landscape com margens de overscan.
- Usar Compose for TV no ecrã final; todos os controlos devem ter foco visível, ordem D-pad previsível e áreas de toque grandes.
- Preferir login por QR code ou código curto associado à conta; não guardar palavra-passe na TV.

## Gate de validação

```bash
adb -s <tv> shell settings put system accelerometer_rotation 0
adb -s <tv> shell am instrument -w \
  com.personalvpn.client.test/androidx.test.runner.AndroidJUnitRunner
```

Validar em emulador TV API 35 e numa Google TV física: navegação apenas com D-pad, foco após rotação/suspensão, teclado remoto, troca Wi‑Fi, consentimento `VpnService.prepare`, Always-on/lockdown e revogação. O estado “VPN ativa” só pode ser mostrado depois de o backend WireGuard aceitar a configuração; handshake e DNS/IPv6 devem ser confirmados no gateway, não pela UI.
