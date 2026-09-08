-- Disposable local data only. Never use these addresses or gateway values in production.
INSERT INTO regions (id, display_name, ipv4_pool, ipv6_pool, dns_ipv4, dns_ipv6, available)
VALUES ('pt-lis', 'Portugal (local)', '10.77.0.0/16', 'fd77:5056:2::/48', '1.1.1.1', '2606:4700:4700::1111', true)
ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, available = EXCLUDED.available;

-- Placeholder gateway only so the Android flow can exercise allocation locally.
-- It is not a reachable WireGuard server and must never be used in production.
INSERT INTO gateways (region_id, name, endpoint_host, endpoint_port, public_key, state, capacity_peers)
VALUES ('pt-lis', 'local-gateway', '10.0.2.2', 51820,
        'B6N8vBQgk8i3VdwbEOhstCY3StFqqFPtC9/AsrhtHHw=', 'ready', 100)
ON CONFLICT (name) DO UPDATE SET region_id = EXCLUDED.region_id,
    endpoint_host = EXCLUDED.endpoint_host, endpoint_port = EXCLUDED.endpoint_port,
    public_key = EXCLUDED.public_key, state = EXCLUDED.state,
    capacity_peers = EXCLUDED.capacity_peers;
