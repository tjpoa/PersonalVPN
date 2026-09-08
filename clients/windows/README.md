# Cliente Windows

O projeto `PersonalVpn.Client.csproj` é a fundação da UI/cliente em .NET 8 para Windows 10/11 x64. A UI não deve alterar rotas, adaptadores ou firewall diretamente; essas operações pertencem a um serviço separado, instalado com privilégios mínimos.

## Dependência WireGuard

Instale a distribuição oficial assinada (requer UAC) antes dos testes de túnel:

```powershell
winget install --id WireGuard.WireGuard --exact --source winget
powershell -File scripts/check-wireguard-windows.ps1
```

O preflight é apenas de leitura; não instala serviços nem altera adaptadores.

## Boundaries de segurança

- `ApiClient` aceita apenas uma origem HTTPS sem credenciais embutidas, rejeita redirecionamentos e exige token/idempotência válidos.
- `DpapiKeyStore` protege a chave privada com DPAPI `CurrentUser`, validando 32 bytes ao proteger e recuperar.
- `AuthTokenStore` protege a sessão serializada com DPAPI `CurrentUser` e falha fechado quando o ficheiro está corrompido.
- `WireGuardConfigRenderer` valida chaves, endpoint UDP, endereços, rotas e keepalive antes de produzir o perfil.
- `WireGuardTool` chama o `wg.exe` oficial para gerar/derivar chaves; não contém uma implementação criptográfica própria.
- `WireGuardIdentityStore` guarda o par derivado cifrado com DPAPI e verifica que a pública corresponde à privada ao carregar.
- `VpnProvisioningCoordinator` reutiliza/regista essa identidade na API e lista regiões sem expor a chave privada.
- `WireGuardTunnelService` prepara perfis com ACL restritiva e chama apenas `wireguard.exe` para instalar/remover o serviço do túnel; requer elevação explícita.
- `WireGuardPipeProtocol` limita comandos IPC a `start`, `stop` e `status`, com versão explícita e rejeição de campos JSON desconhecidos; payloads nunca incluem chaves privadas.

## Integração pendente

`WireGuardNtBackend` permanece fail-closed até a integração com o serviço/manager oficial WireGuard. A implementação deve usar named pipe com ACL restritiva, autenticar o cliente, validar novamente o perfil no serviço e limpar adaptador, rotas e DNS em stop, revogação, crash e uninstall. Não implemente Curve25519 ou o protocolo WireGuard neste projeto.

Valide em VM limpa com UAC aceite/negado, sleep/resume, troca de rede, UDP bloqueado, revogação remota e remoção completa. O job `windows-client` do CI executa o build .NET; testes de túnel exigem Windows real e o componente oficial assinado.
