#!/bin/sh
set -eu

# Docker Desktop for Windows ignores secret uid/gid/mode metadata and mounts
# secrets read-only. Copy only the private key to writable storage first.
cp /run/secrets/postgres_server_key.pem /tmp/postgres_server_key.pem
chown postgres:postgres /tmp/postgres_server_key.pem
chmod 600 /tmp/postgres_server_key.pem
if [ "${1:-}" = "postgres" ]; then
    shift
    exec /usr/local/bin/docker-entrypoint.sh postgres "$@" -c ssl_key_file=/tmp/postgres_server_key.pem
fi
exec /usr/local/bin/docker-entrypoint.sh "$@"
