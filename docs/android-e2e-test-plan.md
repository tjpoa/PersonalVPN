# Plano Android end-to-end

Este plano cobre o primeiro fluxo completo no emulador Android 15/API 35. Não substitui a validação posterior em telefone e Android TV físicos.

## Pré-requisitos

- API de controlo acessível por uma origem HTTPS válida pelo emulador (`10.0.2.2` só identifica o host; não dispensa TLS).
- Conta de teste criada através de `POST /v1/auth/register` e credenciais mantidas fora do repositório.
- Para testar apenas API/provisionamento: executar `powershell -File scripts/seed-local.ps1`, que cria `pt-lis` e `local-gateway` placeholder.
- Para testar tráfego: substituir o placeholder por um gateway WireGuard Linux operacional e isolado; o seed local não fornece UDP na porta `51820`.
- CA/certificado confiável no cliente de teste; nunca desativar validação TLS.

## Sequência

### Staging local no emulador

1. Iniciar a stack: `docker compose -f deploy/docker-compose.staging.yml up --build -d`.
2. Aplicar o seed descartável: `powershell -File scripts/seed-local.ps1`.
3. Iniciar o proxy: `powershell -File scripts/start-local-proxy.ps1`; confiar a CA apenas no AVD de teste.
4. Instalar/abrir a variante debug Android e confirmar que a origem é `https://10.0.2.2:8443`.

5. Fazer login e guardar o par access/refresh cifrado no Android Keystore.
6. Gerar a identidade WireGuard localmente; enviar apenas a chave pública.
7. Listar dispositivos e reutilizar o registo cuja chave pública coincida.
8. Pedir `VpnService.prepare()` e obter consentimento do utilizador antes de ligar.
9. Listar regiões e pedir configuração com `Idempotency-Key` único.
10. Renderizar/validar a configuração, chamar `GoBackend.setState(UP)` e confirmar estado.
11. Com gateway real, testar tráfego; com placeholder, esperar falha de handshake e não declarar ligação ativa.
12. Testar desligar, revogar sessão, reiniciar a app e repetir sem duplicar dispositivo.

## Evidência obrigatória

Guardar apenas estados, códigos HTTP e métricas sem tokens, chaves privadas, configurações completas, IPs de utilizadores ou conteúdo de tráfego. Repetir os cenários de suspensão/retoma, troca de rede, DNS e Always-on num dispositivo físico antes de qualquer publicação.
