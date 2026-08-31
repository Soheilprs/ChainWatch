# ChainWatch

ChainWatch is an Ethereum ERC-20 transfer indexer and Go HTTP service backed by PostgreSQL.

The [production review](docs/PRODUCTION_REVIEW.md) describes the final architecture, safety invariants, failure handling, current limits, scaling roadmap, and project-based interview exercises.

## Run with Docker Compose

Copy the environment template and set `ETH_RPC_URL` to an Ethereum JSON-RPC endpoint:

```bash
cp .env.example .env
docker compose --profile app up --build
```

Compose initializes a new PostgreSQL volume from `migrations/`, waits for the database health check, builds the application image, and publishes ChainWatch on `http://localhost:8080`. Verify it with:

```bash
curl --fail http://localhost:8080/health
curl --fail http://localhost:8080/metrics
```

The `app` profile keeps the existing database-only workflow available:

```bash
docker compose up -d postgres
```

To apply migrations to an existing PostgreSQL volume, run each new migration before starting the application. PostgreSQL's initialization directory only runs scripts when it creates an empty data directory.

## Build the production image

```bash
docker build --tag chainwatch:local .
docker run --rm \
  --publish 8080:8080 \
  --env ETH_RPC_URL=https://ethereum.example/v1/replace-me \
  --env 'DATABASE_URL=postgres://chainwatch:chainwatch@host.docker.internal:5432/chainwatch?sslmode=disable' \
  chainwatch:local
```

The multi-stage build uses digest-pinned base images and produces a stripped, statically linked binary. The runtime image contains CA certificates, runs as the unprivileged `chainwatch` user, and checks `/health`. Runtime behavior is configured entirely with environment variables; see `config.go` for defaults and validation.

## Performance diagnostics

Run the representative microbenchmarks with allocation reporting:

```bash
go test -run '^$' -bench . -benchmem ./...
```

Runtime profiles are disabled by default. To enable them, set `PPROF_ADDRESS` to a loopback-only listener such as `127.0.0.1:6060`, then access `/debug/pprof/` on that separate address. ChainWatch rejects non-loopback profiling addresses, and the public HTTP listener never registers profiling routes.
