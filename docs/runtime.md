# Runtime e operação local

## Configuração

O binário `services/api` exige `DATABASE_URL`; `API_LISTEN_ADDRESS` (por defeito `:8080`) e `APP_ENV` (por defeito `development`) são opcionais. Use um gestor de segredos em ambientes reais. [`.env.example`](../.env.example) contém apenas um formato ilustrativo.

Em produção, a URL PostgreSQL deve usar TLS verificado (`sslmode=verify-full` e CA adequada), uma identidade de base dedicada e permissões mínimas. O processo falha fechado se a base remota tiver TLS desativado. Não passe palavras-passe em argumentos de processo ou logs.

## Arranque seguro

1. Aplicar migrações a partir de uma pipeline de deploy aprovada; a API não executa DDL no arranque.
2. Iniciar a API com um utilizador sem privilégios de sistema e sem acesso a `/etc/wireguard`.
3. Publicar apenas HTTPS através de um reverse proxy que valide certificados e limite tamanho/rate.
4. Usar `/v1/health` para liveness e `/v1/ready` para readiness; não usar liveness para retirar um gateway por falha transitória de DB.
5. Enviar SIGTERM e aguardar até 10 segundos para shutdown gracioso.
6. Confirmar que logs não incluem `DATABASE_URL`, tokens, chaves, destinos ou corpos de pedidos.

## Listener de gateways

O `GATEWAY_LISTEN_ADDRESS` é um listener separado para `/v1/agent/...`. Quando definido, requer simultaneamente `GATEWAY_TLS_CERT_FILE`, `GATEWAY_TLS_KEY_FILE` e `GATEWAY_TLS_CA_FILE`; o servidor exige certificado de cliente verificado com TLS 1.3. Exponha esta porta apenas na rede privada dos gateways, com firewall allow-list por origem. A API de utilizadores deve continuar atrás de HTTPS/reverse proxy sem exigir o certificado de gateway.

Rode a rotação de certificados fora do processo, instale o novo par no gestor de segredos e reinicie de forma coordenada. Nunca coloque PEMs no repositório ou em argumentos de processo.
Antes da rotação, o check somente leitura `powershell -File scripts/check-cert-expiry.ps1 -Path cert.pem,ca.pem` sinaliza certificados expirados ou com menos de 30 dias (configurável).

## Limites atuais

O executável monta autenticação, endpoints de dispositivos/configuração e entrega mTLS de snapshots sobre PostgreSQL. O processo `cmd/reconciler` supervisionado publica snapshots assinados para alterações pendentes; a execução deve ter uma única réplica por base de dados. Ainda faltam rotação automatizada de certificados, binding nativo WireGuard em cada cliente e evidência do laboratório L01–L12 antes de produção.
