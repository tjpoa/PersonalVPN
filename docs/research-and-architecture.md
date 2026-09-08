# Pesquisa e arquitetura da PersonalVPN

> Estado da pesquisa: 2 de setembro de 2026. Este documento propõe uma VPN de acesso à Internet para uso pessoal, com clientes Windows, Android, iOS e Android TV. Requisitos de lojas, APIs e legislação devem ser reconfirmados antes de cada lançamento.

## 1. Decisão principal

Construir a aplicação e o serviço em torno de **WireGuard**, sem criar um protocolo criptográfico próprio. WireGuard é pequeno, auditável, rápido, usa UDP e já oferece componentes oficiais para incorporar em aplicações: serviço/DLL no Windows, WireGuardKit em iOS e a biblioteca `com.wireguard.android:tunnel` em Android ([integração oficial](https://www.wireguard.com/embedding/), [artigo técnico](https://www.wireguard.com/papers/wireguard.pdf)).

O produto terá duas partes independentes:

- **Plano de dados:** clientes WireGuard ligam diretamente a gateways Linux. O gateway encaminha IPv4/IPv6, aplica NAT quando necessário e resolve DNS sem enviar o tráfego pela API.
- **Plano de controlo:** API HTTPS autentica utilizadores/dispositivos, regista apenas chaves públicas, escolhe a região, atribui endereços e cria/remove peers nos gateways.

Para um primeiro MVP pessoal, uma API central e um gateway numa única região são suficientes. A separação lógica deve ser mantida para permitir múltiplas regiões e alta disponibilidade depois.

## 2. Escopo recomendado

### MVP

- Criar conta/iniciar sessão, registar até N dispositivos e revogar um dispositivo.
- Lista pequena de regiões, ligação/desligamento e estado do túnel.
- Túnel integral IPv4 e IPv6, DNS dentro do túnel e bloqueio de fugas.
- Reconexão automática após mudança Wi-Fi/móvel, suspensão e reinício.
- Kill switch onde a plataforma o permite; exclusão de redes locais como opção explícita.
- Diagnóstico local redigido, sem histórico de navegação.

### Depois do MVP

Split tunneling por aplicação/domínio, seleção automática por latência, rotação de gateways, subscrições, suporte empresarial/MDM e protocolo de transporte alternativo para redes que bloqueiem UDP. Evitar prometer “zero logs” até a arquitetura, fornecedores e procedimentos o comprovarem.

## 3. Stack e ambiente de desenvolvimento

| Área | Escolha recomendada | Motivo |
|---|---|---|
| Backend | Go, REST/JSON, OpenAPI | Binário simples, boa concorrência e bibliotecas WireGuard maduras |
| Base de dados | PostgreSQL | Transações para utilizadores, dispositivos, regiões e auditoria |
| Gateways | Linux LTS, WireGuard no kernel, `nftables`, systemd | Caminho de dados mínimo e operacionalmente previsível |
| Android/TV | Kotlin, Gradle, Jetpack Compose; Compose for TV | Partilha domínio/túnel e mantém uma UI própria para comando remoto |
| iOS | Swift, SwiftUI, NetworkExtension, WireGuardKit | Integração nativa obrigatória com `NEPacketTunnelProvider` |
| Windows | C#/.NET para UI + serviço privilegiado Go/Windows ou embeddable WireGuard service | Separa UI sem privilégios das alterações de rede |
| Infraestrutura | OpenTofu/Terraform + Ansible; containers apenas para API/DB | Infra reproduzível; WireGuard do host permanece fora do container |
| CI/CD | GitHub Actions ou equivalente, builds assinados por plataforma | Testes, SBOM, análise de dependências e artefactos reproduzíveis |

Ferramentas locais: Git, Docker Desktop/Podman, Go, .NET SDK, Android Studio com emuladores phone/TV e Xcode. **Um Mac com Xcode e uma conta Apple Developer de organização são indispensáveis** para compilar, assinar e testar iOS. Manter versões fixadas em ficheiros de toolchain, não apenas neste documento.

## 4. Implementação por plataforma

### Android e Android TV

Usar um módulo Kotlin partilhado para autenticação, configuração e túnel, com dois frontends. O serviço declara `android.permission.BIND_VPN_SERVICE`, chama `VpnService.prepare()`, protege o socket do túnel para evitar recursão e cria a interface TUN. Android suporta always-on e lockdown; falhas em lockdown cortam toda a rede, logo esse fluxo exige testes de recuperação ([referência `VpnService`](https://developer.android.com/reference/android/net/VpnService)).

Android TV usa a mesma tecnologia, mas precisa de ecrãs landscape, foco visível, navegação D-pad, texto legível à distância e margens para overscan. Compose for TV é a abordagem recomendada e permite reutilizar a arquitetura Android, não o layout de telemóvel ([guia oficial](https://developer.android.com/training/tv/get-started/create), [layouts TV](https://developer.android.com/training/tv/playback/compose/layouts)). Prever login por QR code/código curto, pois escrever palavras-passe no comando é incómodo.

### iOS

Criar uma app SwiftUI e uma extensão Packet Tunnel que herda de `NEPacketTunnelProvider`. A app guarda a configuração com `NETunnelProviderManager`; a extensão configura IPv4/IPv6, DNS e rotas antes de concluir `startTunnel`. A capacidade `com.apple.developer.networking.networkextension` com `packet-tunnel-provider` é necessária ([Network Extension](https://developer.apple.com/documentation/BundleResources/Entitlements/com.apple.developer.networking.networkextension), [Packet Tunnel Provider](https://developer.apple.com/documentation/networkextension/packet-tunnel-provider)). Segredos ficam no Keychain/App Group; a chave privada nunca vai para a API.

Antes de prometer uma VPN de saída pública no iOS, rever a [TN3120 da Apple](https://developer.apple.com/documentation/technotes/tn3120-expected-use-cases-for-network-extension-packet-tunnel-providers): a Apple distingue acesso a recursos privados de um serviço VPN para encaminhar toda a Internet, e a revisão da App Store pode exigir um caso de uso compatível. Guardar a decisão e a evidência da revisão por versão.

No modo integral, rever cuidadosamente `includeAllNetworks`, rotas excluídas e comportamento durante transições para impedir fugas ([routing da Apple](https://developer.apple.com/documentation/networkextension/routing-your-vpn-network-traffic)). Testar em iPhone/iPad físicos: o simulador não prova o comportamento real da extensão e das mudanças de rede.

### Windows

Preferir o **embeddable WireGuard service**, recomendado pelo próprio projeto em vez de integrar diretamente WireGuardNT ([guia de embedding](https://www.wireguard.com/embedding/)). Executar alterações de adaptador/rotas num Windows Service assinado e privilegiado; a UI comunica por named pipe com ACL restritiva. Implementar instalação MSI/MSIX, atualizações assinadas, arranque automático opcional e remoção completa de serviço, adaptador, rotas e DNS. Validar Windows 10 e 11 em x64 e, se necessário, ARM64.

## 5. Backend e modelo de dados

Serviços iniciais:

1. `api`: autenticação, dispositivos, regiões, sessões e configuração.
2. `gateway-agent`: recebe apenas operações idempotentes assinadas para adicionar/remover peers.
3. `scheduler`: reconcilia estado desejado da base de dados com gateways e elimina peers expirados.
4. `postgres`: metadados; nunca chaves privadas de clientes nem conteúdo/DNS de tráfego.

Entidades mínimas: `users`, `devices`, `device_public_keys`, `gateways`, `ip_leases`, `peer_assignments`, `refresh_tokens` e `audit_events`. A API devolve endpoint, chave pública do servidor, IP atribuído, DNS e rotas. O cliente gera a sua chave privada usando o gerador seguro do sistema e envia somente a chave pública.

Autenticação sugerida: passkeys ou email + palavra-passe com Argon2id, access tokens curtos e refresh tokens rotativos guardados em hash. Administração exige MFA. mTLS ou identidades de workload protegem API–agent; os gateways não expõem PostgreSQL nem portas administrativas à Internet.

## 6. Gateway e rede

Cada região começa com dois gateways para manutenção sem indisponibilidade. Expor apenas UDP WireGuard e acesso administrativo por rede de gestão. Ativar forwarding IPv4/IPv6, regras `nftables` default-deny, limites de taxa e atualizações automáticas de segurança. O procedimento oficial cobre interfaces, peers, chaves e `PersistentKeepalive`, útil para clientes atrás de NAT ([WireGuard Quick Start](https://www.wireguard.com/quickstart/)).

Planeamento mínimo:

- Uma sub-rede privada distinta por região e endereços /32 e /128 por dispositivo.
- DNS recursivo no gateway ou serviço privado alcançável apenas pelo túnel.
- MTU validada em Wi-Fi, 4G/5G, CGNAT e IPv6-only; começar conservador e medir.
- Métricas de CPU, memória, bytes agregados, handshakes e disponibilidade, sem destinos consultados.
- Backups cifrados da base de dados e recuperação ensaiada; gateways devem ser descartáveis.

## 7. Segurança e privacidade

- Modelo de ameaças antes do código: atacante na rede local, conta roubada, gateway comprometido, API maliciosa, fornecedor cloud e fuga de DNS/IPv6.
- Chaves privadas geradas e guardadas no dispositivo (Keychain/Keystore/DPAPI); rotação e revogação simples.
- TLS 1.3 para controlo; WireGuard para dados; certificate pinning apenas com estratégia segura de rotação.
- Logs estruturados com allowlist de campos, IPs truncados quando indispensáveis e retenção curta documentada.
- Dependências fixadas, Renovate/Dependabot, SAST, secret scanning, SBOM e assinatura de releases.
- Separação de funções, credenciais administrativas temporárias e auditoria de ações de suporte.
- Teste independente antes de disponibilização pública e canal responsável para vulnerabilidades.

Na UE, definir finalidade, base legal, retenção, subcontratantes, pedidos dos titulares e resposta a incidentes. O RGPD exige minimização e medidas técnicas/organizativas adequadas; recolher “só porque pode ser útil” não é aceitável ([texto oficial do RGPD](https://eur-lex.europa.eu/eli/reg/2016/679/2016-05-04/eng/pdf)). Obter aconselhamento jurídico para países de operação e localização de gateways.

## 8. Testes e critérios de aceitação

- **Unitários:** parser/configuração, alocação IP, rotação de tokens, reconciliação e redaction.
- **Integração:** namespaces/VMs Linux comprovam handshake, DNS, NAT, IPv4/IPv6 e revogação.
- **Clientes:** ligar/desligar, reboot, sleep, mudança Wi-Fi–móvel, portal cativo, UDP bloqueado, relógio incorreto e servidor indisponível.
- **Fugas:** DNS, IPv6, WebRTC e tráfego durante reconexão/kill switch, medidos externamente.
- **Desempenho:** throughput, latência, consumo de bateria, memória e milhares de peers por gateway.
- **Segurança:** abuso da API, IDOR, token replay, permissões IPC, downgrade/update e análise mobile.
- **Recuperação:** perder um gateway, restaurar DB, revogar dispositivo comprometido e rodar chaves do servidor.

Aceitação do MVP: nenhum tráfego fora do túnel com kill switch ativo; chave privada nunca aparece no servidor/logs; revogação converge em segundos; cliente recupera de troca de rede; instalação/desinstalação não deixa rotas ou adaptadores; todas as plataformas passam a mesma matriz de conectividade.

## 9. Publicação e conformidade das lojas

Google Play permite `VpnService` quando VPN é a função principal, mas exige conta de programador do tipo **organização**, declaração no Play Console, encriptação até ao endpoint, descrição na ficha e consentimento destacado para dados sensíveis; a revisão pode pedir demonstração em vídeo ([política VpnService](https://support.google.com/googleplay/android-developer/answer/12564964?hl=pt-BR), [requisitos de conta](https://support.google.com/googleplay/android-developer/answer/10788890?hl=en)). Reconfirmar estes requisitos antes de cada submissão.

A Apple exige `NEVPNManager`, conta de programador de **organização**, declaração prévia dos dados no ecrã da app, política de privacidade que proíba venda/uso/divulgação a terceiros e licenças em territórios que as exijam ([App Review Guidelines, secção 5.4](https://developer.apple.com/app-store/review/guidelines/)). Preparar também Data Safety/Privacy Nutrition Labels, termos, contacto de suporte e processo de eliminação de conta.

## 10. Estrutura do repositório

```text
clients/
  android/          # módulos mobile, TV e core partilhado
  ios/              # app SwiftUI + PacketTunnel extension
  windows/          # UI, serviço e instalador
services/
  api/              # plano de controlo Go e cmd/gateway-agent
deploy/             # Compose, systemd e templates do gateway
api/openapi.yaml
docs/               # decisões, threat model e runbooks
tests/e2e/           # testes de conectividade entre plataformas
```

Não colocar perfis `.conf`, chaves, tokens, estados Terraform, ficheiros `.env` reais ou certificados no Git. Manter exemplos como `.env.example` apenas com valores fictícios.

## 11. Roadmap executável

### Fase 0 — decisões e laboratório (1–2 semanas)

Fixar países/uso pessoal versus comercial, política de logs e plataformas/versões mínimas. Criar threat model, ADR de WireGuard e laboratório Linux com um gateway e dois peers. Provar DNS, IPv4/IPv6, MTU, NAT e revogação.

### Fase 1 — plano de controlo (2–4 semanas)

Implementar OpenAPI, PostgreSQL, autenticação, dispositivos, leases, gateway-agent e reconciliação. Automatizar uma região com infraestrutura como código, métricas, alertas, backup e rotação.

### Fase 2 — clientes (6–12 semanas, em paralelo)

Construir Android primeiro, adaptar UI para TV, depois Windows e iOS. Partilhar especificação e casos de teste, não uma camada de UI multiplataforma que dificulte APIs nativas. Entregar builds internos assinados.

### Fase 3 — endurecimento e beta (3–6 semanas)

Executar matriz de fugas/reconexão, carga, pentest, recuperação de desastre e revisão de privacidade. Adicionar segundo gateway/região, suporte e telemetria mínima com opt-in quando aplicável.

### Fase 4 — lojas e operação contínua

Preparar textos, vídeos e declarações, beta TestFlight/Play/Windows, resposta a incidentes, SLOs e manutenção mensal. Só então abrir ao público ou cobrar pelo serviço.

## 12. Próximas decisões do proprietário

Antes de gerar o esqueleto de código, decidir: (a) uso apenas próprio/família ou serviço comercial; (b) países/regiões iniciais; (c) orçamento mensal e fornecedor cloud; (d) versões mínimas de cada SO; (e) login e modelo de subscrição; (f) política verificável de logs; e (g) se o primeiro protótipo deve priorizar Android ou Windows. Estas escolhas alteram custos, obrigações, API e calendário, mas não a arquitetura base acima.
