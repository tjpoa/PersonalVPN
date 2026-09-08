# PersonalVPN

Projeto de uma VPN própria para Windows, Android, iOS e Android TV, baseada em WireGuard.

O repositório combina pesquisa, um protótipo funcional do plano de controlo Go e fundações dos clientes nativos. A proposta de arquitetura, stack, requisitos por plataforma, segurança, testes, publicação e roadmap está em [docs/research-and-architecture.md](docs/research-and-architecture.md).

Documentos implementáveis:

- [Modelo de ameaças](docs/threat-model.md)
- [Laboratório WireGuard](docs/lab.md)
- [ADR: escolha de WireGuard](docs/adr/0001-wireguard.md)
- [ADR: contrato de tokens de sessão](docs/adr/0002-session-token-contract.md)
- [Contrato inicial da API](api/openapi.yaml)
- [Plano de controlo e reconciliação](docs/control-plane.md)
- [Migração PostgreSQL inicial](services/api/migrations/0001_initial.sql)
- [Builds nativos e distribuição](docs/native-builds.md)
- [Matriz de validação por plataforma](docs/platform-validation-matrix.md)
- [Plano de teste em hardware](docs/hardware-test-plan.md)
- [Plano Android end-to-end](docs/android-e2e-test-plan.md)
- [Integração WireGuard no Windows](docs/windows-wireguard-integration.md)
- [Unidade systemd do gateway-agent](deploy/systemd/README.md)
- [Provisionamento de gateway Linux](docs/gateway-provisioning.md)
  - [Checklist de beta e publicação](docs/release-checklist.md)
  - [Build iOS em macOS](docs/ios-macos-build.md)
  - [Validação Android TV](docs/android-tv.md)
- [Runbook de operação mTLS](docs/mtls-operations.md)
- [Staging deployment](deploy/README.md)
- [Política de segurança e divulgação](SECURITY.md)
- [Modelo de aviso de privacidade](docs/privacy-notice-template.md)
- [Observabilidade sem dados de tráfego](docs/observability.md)

## Estado

- [x] Pesquisa técnica e restrições das plataformas
- [x] Arquitetura de referência e roadmap
- [x] Especificação do laboratório WireGuard reproduzível
- [x] Harness Docker isolado para handshake, encaminhamento, DNS e revogação
  - [x] Execução isolada inicial: handshake, NAT, DNS e revogação (L01/L01B/L02/L02B/L04/L08)
  - [ ] Execução e evidência dos testes restantes L03/L05–L12
- [x] Contrato e modelo persistente do plano de controlo
- [x] Núcleo de domínio da API com testes unitários
- [x] Migração e persistência PostgreSQL de utilizadores/dispositivos
- [x] Alocação transacional dual-stack, idempotência e capacidade de gateways
- [x] Argon2id, sessões opacas com rotação e handlers HTTP testados
- [x] Protocolo tipado e snapshots assinados do gateway-agent
- [x] Executor WireGuard com argumentos fixos e testes de rejeição
- [x] Wiring de runtime, health/readiness e shutdown gracioso
- [x] Endpoints HTTP autenticados de dispositivos/configuração
- [x] Outbox PostgreSQL durável e configuração mTLS TLS 1.3 do agent
- [x] Worker do agent, transporte HTTPS tipado e fluxo ack/retry testado
- [x] Reconciliador automático de snapshots com shutdown e intervalo configurável
- [x] Endpoints de entrega, listener mTLS dedicado e autorização por certificado
- [x] Fundação Android/Android TV compilável, com identidade Keystore, binding GoBackend e testes de validação
- [x] Fluxo Android debug HTTPS testado no AVD API 35: registo, sessão, dispositivo, provisionamento, handshake WireGuard e tráfego IPv4/NAT pela Internet via gateway WSL2
- [x] Camada de autenticação/provisionamento HTTPS para Windows e armazenamento DPAPI
- [x] Identidade Windows via `wg.exe` oficial, DPAPI e coordenador de provisionamento
- [x] Camada de autenticação/provisionamento HTTPS para iOS e armazenamento Keychain
- [x] Instalação validada do WireGuard oficial no Windows
- [ ] Serviço embeddable/named pipe Windows ligado ao backend
- [ ] Rotação operacional de certificados e operação mTLS end-to-end em ambiente real
- [ ] UI completa e integração de túnel nos clientes Android/TV, Windows e iOS
- [ ] Auditoria, beta e publicação

Não adicione chaves privadas, perfis VPN reais, tokens, certificados ou estados de infraestrutura ao repositório.

Execute `powershell -File scripts/verify.ps1` para verificar extensões proibidas, ausência de campos de chave privada e cobertura mínima das quatro plataformas e entidades persistentes.

O serviço Go fica em `services/api/`. `powershell -File scripts/test-api.ps1` executa os testes e cria uma imagem mínima, sem iniciar contentores nem expor portas.

A matriz de integração e os limites de cada cliente estão em [clients/README.md](clients/README.md); os clientes devem usar as APIs WireGuard oficiais de cada sistema.

`powershell -File scripts/test-db.ps1` inicia um PostgreSQL 17 temporário sem publicar portas, aplica a migração e testa plataformas, ausência de chaves privadas, unicidade de chaves/leases e transições de revogação. O contentor é removido no fim.

`powershell -File scripts/test-api.ps1` aplica `gofmt`, `go vet`, `go test -race` e compila a imagem mínima da API. O pacote de autenticação usa Argon2id via `golang.org/x/crypto/argon2`; a documentação oficial recomenda `IDKey`/Argon2id quando a função é password hashing ([referência](https://pkg.go.dev/golang.org/x/crypto/argon2)).
