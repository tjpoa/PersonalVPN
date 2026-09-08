BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE device_platform AS ENUM ('windows', 'android', 'ios', 'android_tv');
CREATE TYPE device_state AS ENUM ('active', 'revocation_pending', 'revoked');
CREATE TYPE gateway_state AS ENUM ('provisioning', 'ready', 'draining', 'offline');
CREATE TYPE assignment_state AS ENUM ('pending', 'active', 'removal_pending', 'removed');
CREATE TYPE snapshot_state AS ENUM ('queued', 'in_flight', 'acked', 'failed');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email_normalized text NOT NULL UNIQUE,
    password_hash text,
    disabled_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_email_normalized CHECK (
        email_normalized = lower(btrim(email_normalized))
        AND length(email_normalized) BETWEEN 3 AND 320
    ),
    CONSTRAINT users_auth_method CHECK (password_hash IS NOT NULL)
);

CREATE TABLE devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    platform device_platform NOT NULL,
    public_key text NOT NULL UNIQUE,
    state device_state NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    CONSTRAINT devices_name_length CHECK (length(btrim(name)) BETWEEN 1 AND 80),
    CONSTRAINT devices_public_key CHECK (public_key ~ '^[A-Za-z0-9+/]{43}=$'),
    CONSTRAINT devices_revocation_consistent CHECK (
        (state = 'revoked' AND revoked_at IS NOT NULL)
        OR (state <> 'revoked' AND revoked_at IS NULL)
    )
);

CREATE INDEX devices_user_id_idx ON devices(user_id);

CREATE TABLE regions (
    id text PRIMARY KEY,
    display_name text NOT NULL,
    ipv4_pool cidr NOT NULL,
    ipv6_pool cidr NOT NULL,
    dns_ipv4 inet NOT NULL,
    dns_ipv6 inet NOT NULL,
    available boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT regions_id CHECK (id ~ '^[a-z0-9-]{2,32}$'),
    CONSTRAINT regions_display_name CHECK (length(display_name) BETWEEN 1 AND 80),
    CONSTRAINT regions_ipv4_pool CHECK (family(ipv4_pool) = 4),
    CONSTRAINT regions_ipv6_pool CHECK (family(ipv6_pool) = 6),
    CONSTRAINT regions_dns_families CHECK (family(dns_ipv4) = 4 AND family(dns_ipv6) = 6)
);

CREATE TABLE gateways (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    region_id text NOT NULL REFERENCES regions(id) ON DELETE RESTRICT,
    name text NOT NULL UNIQUE,
    endpoint_host text NOT NULL,
    endpoint_port integer NOT NULL DEFAULT 51820,
    public_key text NOT NULL UNIQUE,
    state gateway_state NOT NULL DEFAULT 'provisioning',
    capacity_peers integer NOT NULL,
    generation bigint NOT NULL DEFAULT 0,
    last_seen_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT gateways_name CHECK (name ~ '^[a-z0-9-]{2,63}$'),
    CONSTRAINT gateways_endpoint_host CHECK (length(endpoint_host) BETWEEN 1 AND 253),
    CONSTRAINT gateways_endpoint_port CHECK (endpoint_port BETWEEN 1 AND 65535),
    CONSTRAINT gateways_public_key CHECK (public_key ~ '^[A-Za-z0-9+/]{43}=$'),
    CONSTRAINT gateways_capacity CHECK (capacity_peers > 0)
);

CREATE INDEX gateways_region_state_idx ON gateways(region_id, state);

CREATE TABLE ip_leases (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    region_id text NOT NULL REFERENCES regions(id) ON DELETE RESTRICT,
    device_id uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    ipv4 inet NOT NULL,
    ipv6 inet NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    released_at timestamptz,
    CONSTRAINT ip_leases_families CHECK (family(ipv4) = 4 AND family(ipv6) = 6),
    CONSTRAINT ip_leases_host_addresses CHECK (masklen(ipv4) = 32 AND masklen(ipv6) = 128)
);

CREATE UNIQUE INDEX ip_leases_active_device_idx
    ON ip_leases(device_id) WHERE released_at IS NULL;
CREATE UNIQUE INDEX ip_leases_active_ipv4_idx
    ON ip_leases(region_id, ipv4) WHERE released_at IS NULL;
CREATE UNIQUE INDEX ip_leases_active_ipv6_idx
    ON ip_leases(region_id, ipv6) WHERE released_at IS NULL;

CREATE TABLE peer_assignments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id uuid NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    gateway_id uuid NOT NULL REFERENCES gateways(id) ON DELETE RESTRICT,
    lease_id uuid NOT NULL REFERENCES ip_leases(id) ON DELETE RESTRICT,
    state assignment_state NOT NULL DEFAULT 'pending',
    desired_generation bigint NOT NULL DEFAULT 1,
    observed_generation bigint NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT peer_assignments_generation CHECK (
        desired_generation > 0
        AND observed_generation >= 0
        AND observed_generation <= desired_generation
    ),
    CONSTRAINT peer_assignments_expiry CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX peer_assignments_live_device_idx
    ON peer_assignments(device_id)
    WHERE state IN ('pending', 'active', 'removal_pending');
CREATE INDEX peer_assignments_gateway_reconcile_idx
    ON peer_assignments(gateway_id, state, desired_generation, observed_generation);

CREATE TABLE idempotency_keys (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    operation text NOT NULL,
    key_hash bytea NOT NULL,
    request_hash bytea NOT NULL,
    status_code integer,
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, operation, key_hash),
    CONSTRAINT idempotency_status CHECK (status_code IS NULL OR status_code BETWEEN 200 AND 599),
    CONSTRAINT idempotency_expiry CHECK (expires_at > created_at)
);

CREATE TABLE refresh_token_families (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    current_token_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    reuse_detected_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refresh_token_expiry CHECK (expires_at > created_at)
);

CREATE INDEX refresh_token_families_user_idx ON refresh_token_families(user_id);

CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    resource_type text NOT NULL,
    resource_id uuid,
    result text NOT NULL CHECK (result IN ('success', 'denied', 'failure')),
    trace_id uuid NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT audit_action_length CHECK (length(action) BETWEEN 1 AND 80),
    CONSTRAINT audit_resource_type_length CHECK (length(resource_type) BETWEEN 1 AND 40),
    CONSTRAINT audit_metadata_object CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX audit_events_actor_time_idx ON audit_events(actor_user_id, occurred_at DESC);
CREATE INDEX audit_events_trace_id_idx ON audit_events(trace_id);

CREATE TABLE gateway_snapshots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id uuid NOT NULL REFERENCES gateways(id) ON DELETE CASCADE,
    generation bigint NOT NULL,
    payload jsonb NOT NULL,
    signature bytea NOT NULL,
    state snapshot_state NOT NULL DEFAULT 'queued',
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    acked_at timestamptz,
    last_error text,
    CONSTRAINT gateway_snapshots_generation CHECK (generation > 0),
    CONSTRAINT gateway_snapshots_payload_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT gateway_snapshots_signature_length CHECK (octet_length(signature) = 64),
    CONSTRAINT gateway_snapshots_attempts CHECK (attempts >= 0 AND attempts <= 100),
    CONSTRAINT gateway_snapshots_state_times CHECK (
        (state = 'acked' AND acked_at IS NOT NULL)
        OR (state <> 'acked')
    )
);

CREATE UNIQUE INDEX gateway_snapshots_generation_idx
    ON gateway_snapshots(gateway_id, generation);
CREATE INDEX gateway_snapshots_delivery_idx
    ON gateway_snapshots(gateway_id, state, available_at, generation);

COMMENT ON TABLE audit_events IS
    'Allowlisted administrative metadata only; never store traffic, DNS, tokens, or IP destinations.';
COMMENT ON COLUMN devices.public_key IS
    'WireGuard public key only. Client private keys must never enter the control plane.';
COMMENT ON TABLE gateway_snapshots IS
    'Durable signed outbox. Payload contains typed peer snapshots, never private keys or traffic data.';

COMMIT;
