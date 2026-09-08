# Checklist de beta e publicação

## Pré-beta

- [ ] Todos os jobs CI (`api`, `repository`, `deployment`, `android`, `windows-client` e `ios-client`) concluíram com sucesso no commit de release.
- [ ] Conta Google Play do tipo organização verificada; declaração de `VpnService` submetida e aprovada.
- [ ] WireGuard oficial fixado por versão e SBOM gerado.
- [ ] Migrations aplicadas em PostgreSQL de staging; backups e restauração testados.
- [ ] API e listener gateway com TLS 1.3, mTLS, rotação e allow-list verificados.
- [ ] Aviso de privacidade (`docs/privacy-notice-template.md`) revisto juridicamente, publicado e sincronizado com Play Console/App Store Connect.
- [ ] Chaves privadas geradas/localmente guardadas em Android Keystore, iOS Keychain, DPAPI/credential vault no Windows.
- [ ] Logs redigidos; sem tokens, chaves, destinos, DNS ou conteúdo de tráfego.
- [ ] Dashboards, alertas e retenção seguem o contrato de observabilidade sem labels identificáveis.
- [ ] Revogação, expiração, reconnect, clock skew e falhas de gateway testados.

## Testes por plataforma

- [ ] Android telefone: Wi-Fi, LTE/5G, Doze, Always-on e lockdown.
- [ ] Android TV: navegação por comando remoto, suspensão/retoma e troca de rede.
- [ ] Windows: UAC, serviço sem privilégios excessivos, sleep/resume e uninstall limpo.
- [ ] Windows: validar o [runbook de integração embeddable](windows-wireguard-integration.md) em VM limpa e ARM64.
- [ ] iOS: consentimento VPN, background/kill, Network Extension e App Store review.
- [ ] Verificar IPv4/IPv6, DNS leak, kill-switch, MTU e captive portals.
- [ ] Executar a [matriz de validação por plataforma](platform-validation-matrix.md) duas vezes, incluindo hardware físico.

## Operação

- [ ] Monitorizar liveness, readiness, handshake age, filas e capacidade sem recolher tráfego.
- [ ] Alertar falhas de claim/ack/retry e esgotamento de endereços.
- [ ] Runbooks de incidente, rotação de certificados, rollback e revogação de dispositivo.
- [ ] Executar o [runbook de operação mTLS](mtls-operations.md), incluindo rollback e certificado expirado.
- [ ] Política de privacidade, termos, suporte e processo de resposta a vulnerabilidades publicados.
