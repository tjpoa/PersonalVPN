# ADR 0001: WireGuard como protocolo do túnel

- Estado: aceite
- Data: 2026-09-02

## Contexto

O produto precisa de um túnel seguro e eficiente em Windows, Android, iOS e Android TV. Criar criptografia ou um protocolo próprio aumentaria drasticamente o risco, tempo de auditoria e manutenção. IKEv2 tem integração nativa ampla, mas oferece menos controlo uniforme para incorporar uma experiência de produto própria.

## Decisão

Usar WireGuard como protocolo de dados e os componentes oficiais de integração em cada plataforma. O plano de controlo gere identidades, chaves públicas, leases e peers, mas nunca transporta chaves privadas nem tráfego do utilizador.

## Consequências

- O gateway principal será Linux com WireGuard no kernel.
- O transporte normal é UDP; redes que bloqueiam UDP não terão fallback no MVP.
- Rotação/revogação de chaves, DNS, kill switch, obfuscação e alta disponibilidade continuam a ser responsabilidades do produto.
- Android/TV partilham a biblioteca de túnel, iOS usa WireGuardKit/NetworkExtension e Windows usa o embeddable service.
- Qualquer protocolo alternativo exige novo ADR, threat model e testes de interoperabilidade.
