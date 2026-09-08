# Unidade systemd do gateway-agent

Use [`../gateway/gateway-agent.env.example`](../gateway/gateway-agent.env.example) como referência para o ambiente do agent e [`../gateway/nftables.conf.example`](../gateway/nftables.conf.example) para uma política default-deny. São templates: substitua IDs, certificados e interfaces numa VM Linux descartável e faça revisão antes de aplicar.

O ficheiro `personalvpn-gateway-agent.service` é um template de staging/produção. Instale o binário em `/opt/personalvpn/` e crie o utilizador dedicado `personalvpn-agent` sem shell de login. A interface WireGuard deve existir previamente e o serviço recebe apenas `CAP_NET_ADMIN` para executar as operações `wg` necessárias.

Crie `/etc/personalvpn/gateway-agent.env` com permissões `0600` e propriedade `root:personalvpn-agent`:

```text
CONTROL_API_URL=https://api.internal.example
GATEWAY_ID=00000000-0000-0000-0000-000000000000
WG_INTERFACE=wg0
SNAPSHOT_VERIFY_KEY=base64-ed25519-public-key
AGENT_TLS_CERT_FILE=/etc/personalvpn/tls/agent.crt
AGENT_TLS_KEY_FILE=/etc/personalvpn/tls/agent.key
AGENT_TLS_CA_FILE=/etc/personalvpn/tls/ca.crt
AGENT_TLS_SERVER_NAME=api.internal.example
AGENT_POLL_INTERVAL=5s
```

Instalação típica, após validar hashes/assinaturas e certificados:

```bash
sudo install -o root -g root -m 0755 personalvpn-gateway-agent /opt/personalvpn/personalvpn-gateway-agent
sudo install -o root -g root -m 0644 personalvpn-gateway-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now personalvpn-gateway-agent.service
systemctl status personalvpn-gateway-agent.service
```

Não coloque chaves privadas, tokens ou o ficheiro `.env` no Git. Teste `systemd-analyze security personalvpn-gateway-agent.service` e confirme que a unidade não obtém privilégios além de `CAP_NET_ADMIN`.
