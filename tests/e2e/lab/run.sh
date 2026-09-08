#!/usr/bin/env bash
set -Eeuo pipefail

readonly RESULT_DIR="/results"
readonly RESULT_FILE="${RESULT_DIR}/wireguard-lab.txt"
readonly KEY_DIR="$(mktemp -d)"
readonly RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"

declare -a PASSED=()
declare -a FAILED=()

cleanup() {
  for pid_file in /tmp/dnsmasq-lab.pid; do
    if [[ -f "${pid_file}" ]]; then
      kill "$(cat "${pid_file}")" 2>/dev/null || true
    fi
  done
  for ns in client-a client-b gateway internet; do
    ip netns del "${ns}" 2>/dev/null || true
  done
  ip link del underlay 2>/dev/null || true
  rm -rf "${KEY_DIR}"
}
trap cleanup EXIT

record() {
  local id="$1"
  local description="$2"
  shift 2
  if "$@" >/tmp/test-output 2>&1; then
    PASSED+=("${id} ${description}")
    printf 'PASS %-4s %s\n' "${id}" "${description}"
  else
    FAILED+=("${id} ${description}: $(tr '\n' ' ' </tmp/test-output)")
    printf 'FAIL %-4s %s\n' "${id}" "${description}"
  fi
}

wait_for_handshake() {
  local attempt
  for attempt in $(seq 1 20); do
    if ip netns exec gateway wg show wg0 latest-handshakes | awk '$2 > 0 { found=1 } END { exit !found }'; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

has_two_handshakes() {
  [[ "$(ip netns exec gateway wg show wg0 latest-handshakes | awk '$2 > 0 { count++ } END { print count+0 }')" -eq 2 ]]
}

revocation_blocks_client_b() {
  ip netns exec gateway wg set wg0 peer "$(cat "${KEY_DIR}/b.pub")" remove
  ! ip netns exec client-b ping -c 1 -W 1 10.70.0.1
}

mkdir -p "${RESULT_DIR}"
umask 077

if ! ip link add wg-probe type wireguard 2>/tmp/wg-probe-error; then
  printf 'WireGuard kernel support unavailable: %s\n' "$(cat /tmp/wg-probe-error)" | tee "${RESULT_FILE}"
  exit 2
fi
ip link del wg-probe

for peer in gateway a b; do
  wg genkey >"${KEY_DIR}/${peer}.key"
  wg pubkey <"${KEY_DIR}/${peer}.key" >"${KEY_DIR}/${peer}.pub"
done

for ns in client-a client-b gateway internet; do
  ip netns add "${ns}"
  ip -n "${ns}" link set lo up
done

ip link add underlay type bridge
ip link set underlay up

attach_underlay() {
  local ns="$1"
  local suffix="$2"
  local address="$3"
  ip link add "v-${suffix}" type veth peer name eth0 netns "${ns}"
  ip link set "v-${suffix}" master underlay
  ip link set "v-${suffix}" up
  ip -n "${ns}" address add "${address}/24" dev eth0
  ip -n "${ns}" link set eth0 up
}

attach_underlay gateway gw 192.0.2.1
attach_underlay client-a ca 192.0.2.2
attach_underlay client-b cb 192.0.2.3

ip link add gw-inet type veth peer name eth0 netns internet
ip link set gw-inet netns gateway
ip -n gateway address add 198.51.100.1/24 dev gw-inet
ip -n gateway link set gw-inet up
ip -n internet address add 198.51.100.2/24 dev eth0
ip -n internet link set eth0 up

for ns in gateway client-a client-b; do
  ip -n "${ns}" link add wg0 type wireguard
done

ip -n gateway address add 10.70.0.1/24 dev wg0
ip netns exec gateway wg set wg0 \
  listen-port 51820 \
  private-key "${KEY_DIR}/gateway.key" \
  peer "$(cat "${KEY_DIR}/a.pub")" allowed-ips 10.70.0.2/32 \
  peer "$(cat "${KEY_DIR}/b.pub")" allowed-ips 10.70.0.3/32
ip -n gateway link set wg0 up

configure_client() {
  local ns="$1"
  local key="$2"
  local address="$3"
  ip -n "${ns}" address add "${address}/32" dev wg0
  ip netns exec "${ns}" wg set wg0 \
    private-key "${KEY_DIR}/${key}.key" \
    peer "$(cat "${KEY_DIR}/gateway.pub")" \
    endpoint 192.0.2.1:51820 \
    allowed-ips 10.70.0.0/24,198.51.100.0/24 \
    persistent-keepalive 5
  ip -n "${ns}" link set wg0 up
  ip -n "${ns}" route add 10.70.0.0/24 dev wg0
  ip -n "${ns}" route add 198.51.100.0/24 dev wg0
}

configure_client client-a a 10.70.0.2
configure_client client-b b 10.70.0.3

ip netns exec gateway sysctl -q -w net.ipv4.ip_forward=1
ip netns exec gateway nft -f - <<'NFT'
table inet lab_filter {
  chain forward {
    type filter hook forward priority 0; policy drop;
    ct state established,related accept
    iifname "wg0" oifname "gw-inet" accept
  }
}
table ip lab_nat {
  chain postrouting {
    type nat hook postrouting priority 100; policy accept;
    oifname "gw-inet" ip saddr 10.70.0.0/24 masquerade
  }
}
NFT

ip netns exec gateway dnsmasq \
  --no-daemon \
  --bind-interfaces \
  --interface=wg0 \
  --listen-address=10.70.0.1 \
  --no-resolv \
  --address=/vpn.test/198.51.100.2 \
  --pid-file=/tmp/dnsmasq-lab.pid \
  >/tmp/dnsmasq.log 2>&1 &
echo "$!" >/tmp/dnsmasq-lab.pid
sleep 0.5

ip netns exec client-a ping -c 1 -W 2 10.70.0.1 >/dev/null
ip netns exec client-b ping -c 1 -W 2 10.70.0.1 >/dev/null

record L01 "dois peers com handshake" wait_for_handshake
record L01B "handshake de ambos os peers" has_two_handshakes
record L02 "encaminhamento e NAT até Internet sintética" ip netns exec client-a ping -c 2 -W 2 198.51.100.2
record L04 "DNS servido exclusivamente no endereço do túnel" ip netns exec client-a dig +short +time=2 +tries=1 @10.70.0.1 vpn.test A
record L08 "revogação bloqueia client-b" revocation_blocks_client_b
record L02B "revogação não afeta client-a" ip netns exec client-a ping -c 1 -W 2 198.51.100.2

{
  printf 'PersonalVPN WireGuard isolated lab\n'
  printf 'run_id=%s\n' "${RUN_ID}"
  printf 'kernel=%s\n' "$(uname -r)"
  printf 'wireguard_tools=%s\n' "$(wg --version | head -n1)"
  printf 'passed=%d\n' "${#PASSED[@]}"
  printf 'failed=%d\n' "${#FAILED[@]}"
  printf '\nPassed:\n'
  printf '%s\n' "${PASSED[@]}"
  if ((${#FAILED[@]})); then
    printf '\nFailed:\n'
    printf '%s\n' "${FAILED[@]}"
  fi
} >"${RESULT_FILE}"

if ((${#FAILED[@]})); then
  exit 1
fi
