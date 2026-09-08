\set ON_ERROR_STOP on

DO $$
DECLARE
    expected_platforms text[] := ARRAY['android', 'android_tv', 'ios', 'windows'];
    actual_platforms text[];
    forbidden_columns integer;
    test_user_id uuid;
    first_device_id uuid;
    second_device_id uuid;
    test_region_id text := 'pt-test';
BEGIN
    SELECT array_agg(enumlabel ORDER BY enumlabel)
      INTO actual_platforms
      FROM pg_enum
      JOIN pg_type ON pg_type.oid = pg_enum.enumtypid
     WHERE pg_type.typname = 'device_platform';

    IF actual_platforms IS DISTINCT FROM expected_platforms THEN
        RAISE EXCEPTION 'device_platform mismatch: %', actual_platforms;
    END IF;

    SELECT count(*)
      INTO forbidden_columns
      FROM information_schema.columns
     WHERE table_schema = 'public'
       AND lower(column_name) IN ('private_key', 'privatekey', 'client_private_key');

    IF forbidden_columns <> 0 THEN
        RAISE EXCEPTION 'schema contains forbidden private-key columns';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_type WHERE typname = 'snapshot_state'
    ) OR NOT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'public' AND table_name = 'gateway_snapshots'
    ) THEN
        RAISE EXCEPTION 'durable gateway snapshot outbox is missing';
    END IF;

    INSERT INTO users (email_normalized, password_hash)
    VALUES ('integration@example.invalid', 'argon2id-placeholder-not-a-real-secret')
    RETURNING id INTO test_user_id;

    INSERT INTO regions (
        id, display_name, ipv4_pool, ipv6_pool, dns_ipv4, dns_ipv6, available
    ) VALUES (
        test_region_id, 'Integration Test', '10.70.0.0/24', 'fd70::/64',
        '10.70.0.1', 'fd70::1', true
    );

    INSERT INTO devices (user_id, name, platform, public_key)
    VALUES (
        test_user_id, 'Device A', 'android',
        'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA='
    ) RETURNING id INTO first_device_id;

    BEGIN
        INSERT INTO devices (user_id, name, platform, public_key)
        VALUES (
            test_user_id, 'Duplicate key', 'ios',
            'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA='
        );
        RAISE EXCEPTION 'duplicate public key was accepted';
    EXCEPTION
        WHEN unique_violation THEN NULL;
    END;

    INSERT INTO devices (user_id, name, platform, public_key)
    VALUES (
        test_user_id, 'Device B', 'windows',
        'AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE='
    ) RETURNING id INTO second_device_id;

    INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6)
    VALUES (test_region_id, first_device_id, '10.70.0.2/32', 'fd70::2/128');

    BEGIN
        INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6)
        VALUES (test_region_id, second_device_id, '10.70.0.2/32', 'fd70::3/128');
        RAISE EXCEPTION 'duplicate active IPv4 lease was accepted';
    EXCEPTION
        WHEN unique_violation THEN NULL;
    END;

    BEGIN
        INSERT INTO ip_leases (region_id, device_id, ipv4, ipv6)
        VALUES (test_region_id, second_device_id, 'fd70::4/128', '10.70.0.4/32');
        RAISE EXCEPTION 'swapped IP families were accepted';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;

    UPDATE devices
       SET state = 'revocation_pending'
     WHERE id = first_device_id;

    BEGIN
        UPDATE devices
           SET state = 'revoked'
         WHERE id = first_device_id;
        RAISE EXCEPTION 'revoked device without revoked_at was accepted';
    EXCEPTION
        WHEN check_violation THEN NULL;
    END;
END
$$;

SELECT 'database integration assertions passed' AS result;
