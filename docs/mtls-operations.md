# Operação mTLS entre API e gateway-agent

Este procedimento cobre certificados de serviço e de cliente usados pelo listener dedicado do agent. A CA nunca deve ser a mesma das chaves WireGuard nem ser guardada no repositório.

## Emissão

1. Criar uma CA privada num gestor de segredos/HSM e emitir certificados separados por ambiente e gateway.
2. Usar SAN DNS explícito para o serviço e EKU `serverAuth` no certificado da API; cada agent recebe EKU `clientAuth` e um identificador de gateway no `Subject CN`.
3. Distribuir ficheiros através do gestor de segredos com permissões de leitura apenas para a conta do processo (`0600` em Linux). Registar apenas serial, emissor, SAN e validade — nunca chaves privadas.
4. Executar `scripts/check-cert-expiry.ps1` no pipeline de staging e gerar alerta com 30 dias de antecedência.

## Rotação sem interrupção

1. Emitir o novo certificado mantendo a CA atual e validar cadeia, SAN, EKU e relógio numa máquina descartável.
2. Instalar o par novo em caminho versionado, testar `ServerTLSConfig`/`ClientTLSConfig` e confirmar que o agent rejeita certificados sem a CA correta.
3. Reiniciar primeiro uma instância de API; verificar `/ready`, entrega de snapshot, ack e retry. Só depois rodar as restantes instâncias e agents em lotes pequenos.
4. Manter o certificado anterior até todos os agents confirmarem a nova cadeia; remover o antigo após a janela de compatibilidade documentada.

## Revogação e incidente

Revogar imediatamente o certificado comprometido na CA, remover o gateway da lista de allow-list e pausar as entregas. Emitir novo certificado e identidade de gateway, restaurar o serviço e confirmar que o serial antigo recebe `401`. Preservar apenas metadados de auditoria necessários ao incidente.

## Critérios de aceitação

- listener público nunca aceita HTTP nem certificado de cliente ausente;
- rotação é concluída sem perder snapshots (outbox permite reclamar/repetir);
- expiração e falhas de cadeia produzem alerta antes de afetar clientes;
- rollback restaura a versão anterior sem alterar rotas ou chaves WireGuard.
