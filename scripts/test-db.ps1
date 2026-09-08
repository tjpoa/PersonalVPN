[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$containerName = 'personalvpn-postgres-test'
$postgresImage = 'postgres:17-alpine'
$testPassword = 'local-integration-test-only'

try {
    docker run --rm --detach `
        --name $containerName `
        --mount "type=bind,source=${repoRoot},target=/workspace,readonly" `
        --env "POSTGRES_PASSWORD=${testPassword}" `
        --env 'POSTGRES_DB=personalvpn_test' `
        $postgresImage | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Could not start PostgreSQL test container: $LASTEXITCODE"
    }

    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        docker exec $containerName pg_isready --host 127.0.0.1 --username postgres --dbname personalvpn_test | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw 'PostgreSQL did not become ready within 30 seconds.'
    }

    docker exec $containerName psql `
        --host 127.0.0.1 `
        --username postgres `
        --dbname personalvpn_test `
        --file /workspace/services/api/migrations/0001_initial.sql
    if ($LASTEXITCODE -ne 0) {
        throw "Migration failed: $LASTEXITCODE"
    }

    docker exec $containerName psql `
        --host 127.0.0.1 `
        --username postgres `
        --dbname personalvpn_test `
        --file /workspace/tests/integration/sql/assertions.sql
    if ($LASTEXITCODE -ne 0) {
        throw "Database assertions failed: $LASTEXITCODE"
    }

    $apiRoot = Join-Path $repoRoot 'services/api'
    $buildImage = 'personalvpn-api-build:test'
    docker build --target build --tag $buildImage $apiRoot
    if ($LASTEXITCODE -ne 0) {
        throw "Could not build API integration-test image: $LASTEXITCODE"
    }

    docker run --rm `
        --network "container:${containerName}" `
        --env "TEST_DATABASE_URL=postgres://postgres:${testPassword}@127.0.0.1:5432/personalvpn_test?sslmode=disable" `
        $buildImage `
        go test -race -tags=integration ./internal/postgres
    if ($LASTEXITCODE -ne 0) {
        throw "Go/PostgreSQL integration tests failed: $LASTEXITCODE"
    }
}
finally {
    docker rm --force $containerName 2>$null | Out-Null
}
