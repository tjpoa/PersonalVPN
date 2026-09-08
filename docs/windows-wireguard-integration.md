# Integração WireGuard no Windows

O cliente Windows deve separar a UI (sem privilégios) do serviço de túnel (conta de serviço dedicada). A implementação deve usar o serviço embeddable oficial do WireGuard, não invocar `wireguard.exe` nem manipular diretamente o driver WireGuardNT a partir da UI.

## Componentes

- **UI .NET 8:** autenticação, seleção de região, estado e pedidos ao control-plane.
- **Renderer de configuração:** `clients/windows/WireGuardConfigRenderer.cs` valida chaves Base64 de 32 bytes, endpoint UDP, rotas e keepalive antes de entregar o perfil ao serviço.
- **Windows Service:** valida uma configuração já estruturada, inicia/paralisa o serviço WireGuard e remove adaptador, rotas e DNS no encerramento.
- **Named pipe ACL:** pipe com nome fixo por instalação, permitido apenas ao utilizador interativo e à conta do serviço; mensagens limitadas a comandos versionados (`start`, `stop`, `status`). Nunca transportar a chave privada em logs ou argumentos.
- **Contrato IPC:** `clients/windows/WireGuardPipeProtocol.cs` valida versão/operação e exige configuração completa apenas em `start`; o payload não contém chave privada.
- **Armazenamento:** proteger a chave privada com DPAPI `CurrentUser` na UI; o serviço recebe apenas o material necessário através do canal autenticado e limpa buffers após uso.

## Instalação e atualização

1. Publicar MSI/MSIX assinado com o executável do serviço, DLLs e driver oficial verificados por hash/SBOM.
2. Criar o serviço com arranque manual inicialmente; pedir elevação UAC apenas para instalar, atualizar ou remover.
3. Aplicar ACL mínima no serviço, diretório, pipe e adaptador. Não conceder direitos administrativos à UI.
4. Atualizar de forma transacional: parar túnel, instalar binários assinados, validar versão, iniciar e confirmar handshake; reverter para o pacote anterior em qualquer falha.
5. Na remoção, parar o túnel e confirmar que não restam serviço, adaptador, rotas, DNS, tarefas agendadas ou segredos.

## Testes de aceitação

Validar Windows 10/11 x64 em VM limpa e, antes do release, ARM64. Cobrir UAC negado, crash/restart do serviço, sleep/resume, troca de rede, UDP bloqueado, revogação remota e desinstalação. O serviço deve falhar fechado: configuração inválida ou pipe não autenticado não altera a rede.

Consulte o [guia oficial de embedding do WireGuard](https://www.wireguard.com/embedding/) e mantenha a revisão do componente fixada no SBOM.
