# Builds nativos e distribuição

## Android e Android TV

Use Android Studio estável, JDK 17, Android SDK 35 e o Gradle Wrapper em `clients/android/gradlew.bat` (CI fixa Gradle 8.9). Configure `ANDROID_HOME` apenas no runner; não coloque keystores, `google-services.json` ou tokens no repositório. Validações mínimas:

```powershell
cd clients/android
gradle lint test assembleDebug --no-daemon
```

O mesmo APK pode declarar suporte TV com uma atividade Leanback, mas a UX deve ser testada com comando remoto e sem touchscreen. O binding WireGuard deve ser fixado por versão e origem verificável.

## Windows

Use Windows 10/11 x64, .NET 8 SDK (fixado em `global.json`), Windows SDK e WireGuardNT/manager oficial assinado. Execute:

```powershell
$dotnet = "$env:LOCALAPPDATA\PersonalVPN\dotnet8\dotnet.exe" # ou `dotnet` se estiver no PATH
& $dotnet restore clients/windows/PersonalVpn.Client.csproj
& $dotnet build clients/windows/PersonalVpn.Client.csproj --configuration Release --no-restore
```

O serviço de túnel deve ser instalado com privilégios explícitos e possuir uma conta de serviço sem acesso às chaves de outros utilizadores.

## iOS

Use um runner macOS com Xcode compatível, Apple Developer Team, entitlement Network Extension e perfis de assinatura separados para a app e `NEPacketTunnelProvider`. Execute `swift test` no pacote comum e valide a extensão em dispositivo físico; o simulador não prova encaminhamento WireGuard.

Todos os pipelines devem publicar apenas artefactos assinados, gerar SBOM e manter certificados/segredos num gestor externo. Nenhum pipeline deve alterar rotas ou firewall do host sem um ambiente descartável explicitamente aprovado.

No Windows, `powershell -ExecutionPolicy Bypass -File scripts/check-native-tools.ps1` faz um diagnóstico somente leitura dos comandos disponíveis e indica o runner necessário para cada plataforma.

O workflow `.github/workflows/ci.yml` já cobre a API Go, deployment, Android, Windows e o package Swift. A assinatura de releases e os testes em hardware permanecem separados dos builds de CI.
