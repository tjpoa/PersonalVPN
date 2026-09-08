# Staging deployment

`docker-compose.staging.yml` é apenas uma base para staging isolado. Antes de usar:

1. Criar `deploy/secrets/postgres_password` fora do Git com permissões restritas.
2. Criar `deploy/secrets/snapshot_signing_key` com uma chave Ed25519 privada em Base64, fora do Git; a chave pública correspondente deve estar configurada em cada agent.
3. Criar `postgres_ca.pem`, `postgres_server_cert.pem` e `postgres_server_key.pem` em `deploy/secrets/`, emitidos por uma CA privada; o certificado deve conter SAN `db` e a chave deve ser legível pelo UID 999 do PostgreSQL.
4. O serviço one-shot `migrate` mantém `schema_migrations` e aplica a migration versionada de forma idempotente antes de API/reconciliador; em produção, substitua-o por um runner de migrations com controlo de versão e aprovação.
5. Configurar CA/certificados e um reverse proxy HTTPS fora deste compose.
6. Fazer backup e testar restauração do volume antes de dados reais.

O compose usa uma rede Docker interna e não publica PostgreSQL. Não é uma configuração de produção: faltam proxy, observabilidade, rotação de secrets e política de backup gerida.

Para dados descartáveis de desenvolvimento, depois de iniciar o compose execute `powershell -File scripts/seed-local.ps1`. Isto cria a região `pt-lis` e um gateway WireGuard placeholder (`local-gateway`) apenas para testar alocação/provisionamento; não fornece tráfego VPN e nunca deve ser usado em produção.

Para expor a API ao emulador Android, execute `powershell -File scripts/start-local-proxy.ps1`. O proxy publica a porta de desenvolvimento `8443`, usa TLS interno Caddy e encaminha para a API; instale a CA de desenvolvimento Caddy no emulador antes de configurar o cliente. Restrinja esta porta na firewall do Windows quando estiver numa rede não confiável.
Extraia e abra o instalador da CA com `powershell -File scripts/trust-local-ca.ps1`; confirme a instalação no emulador. Este certificado é exclusivamente local e está em `deploy/secrets/` (ignorado pelo Git).

As imagens PostgreSQL, Go e Caddy estão fixadas por digest no Compose/Dockerfile; atualizações devem alterar explicitamente o tag e o digest após revisão.

## Execução controlada

Depois de criar todos os ficheiros em `deploy/secrets/`, valide e inicie o staging:

```powershell
docker compose -f deploy/docker-compose.staging.yml config --no-interpolate
docker compose -f deploy/docker-compose.staging.yml up --build -d
docker compose -f deploy/docker-compose.staging.yml ps
```

Para recolher logs e encerrar sem apagar o volume da base de dados:

```powershell
docker compose -f deploy/docker-compose.staging.yml logs --since 10m api reconciler migrate
docker compose -f deploy/docker-compose.staging.yml down
```

Não execute `down --volumes` sem confirmar um backup; essa opção elimina o volume local de PostgreSQL.

O healthcheck `pg_isready` só confirma que o processo PostgreSQL aceita ligações. API e reconciliador aguardam o término bem-sucedido de `migrate`; falhas de migration impedem o arranque e exigem intervenção.
