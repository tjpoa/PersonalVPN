# Modelo de aviso de privacidade

> Template técnico para revisão por responsável de proteção de dados/jurista. Substitua os campos entre `<...>` antes de publicar; não representa aconselhamento jurídico.

## O que a PersonalVPN trata

- **Conta:** `<email/passkey e finalidade>`, para autenticação e recuperação.
- **Dispositivo:** nome, plataforma, chave pública WireGuard, estado de revogação e endereços privados atribuídos, para provisionar o túnel.
- **Operação:** horários de sessão, idade do handshake, bytes agregados e erros técnicos, para disponibilidade e diagnóstico.
- **Pagamentos/suporte:** `<se aplicável, fornecedor e dados tratados>`.

A chave privada WireGuard é gerada e guardada no dispositivo (Android Keystore, iOS Keychain ou DPAPI/credential vault no Windows) e não é enviada ao plano de controlo. Não registamos destinos visitados, conteúdo, consultas DNS, payloads ou capturas de tráfego. Se essa política mudar, atualizar este aviso, a ficha da loja e o consentimento antes do release.

## Finalidade, base legal e retenção

Use os campos aprovados pelo responsável: `<finalidade>`, `<base legal>`, `<prazo de retenção>`, `<critério de eliminação>`. Logs técnicos devem ter retenção mínima e acesso restrito; eventos de segurança podem exigir retenção diferente, documentada e proporcional.

## Partilha e transferências

Liste cada subcontratante (cloud, email, pagamentos, suporte), localização, dados partilhados e salvaguardas: `<lista validada>`. Não vender nem usar dados de tráfego para publicidade. Gateways devem receber apenas configuração necessária ao encaminhamento, nunca credenciais da conta ou conteúdo.

## Direitos e contacto

Indique responsável, contacto, autoridade de controlo, processo para acesso/retificação/eliminação/portabilidade e prazo de resposta: `<dados do responsável>`. Inclua contacto de segurança separado conforme `SECURITY.md`.

## Consentimento e lojas

No Android, mostrar divulgação destacada antes de ativar `VpnService` e completar a declaração do Play Console. No iOS, declarar os dados antes de qualquer compra/uso e manter a política pública sincronizada com App Store Connect. Registar versão e data do texto aceite sem armazenar tráfego.
