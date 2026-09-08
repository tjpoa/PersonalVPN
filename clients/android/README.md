# Módulo Android

O serviço usa `VpnService` apenas para o ciclo de vida e delega o transporte a `WireGuardEngine`. A dependência fixada é `com.wireguard.android:tunnel:1.0.20260102`, publicada pelo projeto [wireguard-android](https://git.zx2c4.com/wireguard-android/), cuja API inclui o `GoBackend` para modo userspace sem root. Fixe uma versão/revisão e valide licenças e artefactos antes de publicar.

O módulo ativa Java 17 e core-library desugaring, requisito do binding oficial para recursos Java 8+.

Os ficheiros `local.properties` e os certificados `.crt` são locais e ignorados pelo Git. Para usar a CA do proxy Caddy em debug, coloque `caddy_local_root.crt` e `caddy_local_intermediate.crt` em `app/src/debug/res/raw/`. Sem estes recursos, o cliente usa a configuração de confiança normal do Android; o build debug também permite CAs instaladas pelo utilizador. Builds release usam apenas a configuração de confiança de produção.

Use `RegisterDeviceRequest.forDevice(...)` ao registar o equipamento: ele envia `android_tv` quando `UiModeManager` identifica televisão e `android` nos restantes dispositivos.

Fluxo de integração:

1. Pedir `VpnService.prepare()` e obter consentimento do utilizador.
2. Converter `TunnelConfiguration` para o modelo de configuração do binding.
3. Renderizar a configuração com `WireGuardConfigRenderer`; a chave privada permanece apenas em memória.
4. Declarar `GoBackend.VpnService` (a classe que `GoBackend.setState()` inicia) e entregá-lo ao backend oficial; ele cria o TUN, configura rotas/DNS e protege os sockets de controlo.
5. Parar o backend em `onDestroy`, revogação, logout e erro fatal.

Os pedidos HTTPS ao plano de controlo usam a `Network` física escolhida antes de
ativar o túnel. Isto evita que uma configuração full-tunnel tente contactar o
próprio servidor local através do TUN (`10.0.2.2:8443`).

`WireGuardIdentityStore` gera o par com `KeyPair()` do binding oficial e guarda apenas o valor cifrado da chave privada em `SharedPreferences`; a chave de cifragem é não exportável no Android Keystore. A criação usa `commit()` síncrono para garantir persistência antes de enviar o public key, que é o único material enviado ao plano de controlo.
Ao terminar sessão ou confirmar revogação, chamar `delete()` e aguardar o `commit()` antes de libertar a UI; o método remove tanto o blob cifrado como a chave Keystore.

Em Android 14+ o serviço declara `specialUse`/`FOREGROUND_SERVICE_SPECIAL_USE` e chama `startForeground` com o tipo correspondente; o canal de baixa prioridade mantém o túnel visível e permite ao utilizador pará-lo.

Não copie código criptográfico do binding nem grave a configuração WireGuard completa em logs. O projeto já fixa o AAR oficial e `GoBackendWireGuardEngine` faz a ponte para `Config.parse`/`setState`; a ativação completa ainda depende de uma configuração emitida pela API e de validação em hardware. `VpnTunnelService` coordena intents da app; o túnel de dados é executado pelo `GoBackend.VpnService` do AAR.
