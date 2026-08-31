# ChainWatch

ChainWatch is a production-oriented Ethereum ERC-20 indexing service written in Go.

It continuously follows the Ethereum chain, decodes ERC-20 `Transfer` events, persists indexed data into PostgreSQL, handles chain reorganizations, enriches transfers with token metadata, and exposes a queryable HTTP API with cursor pagination and observability.

The project was built to explore practical Go backend and blockchain infrastructure patterns such as bounded concurrency, atomic persistence, reorg recovery, resilient RPC access, structured logging, metrics, profiling, graceful shutdown, and production deployment.

---

## Features

* Ethereum block indexing
* ERC-20 `Transfer` event decoding
* Concurrent block fetching with bounded worker pools
* Ordered block persistence
* PostgreSQL-backed checkpoints
* Atomic transfer + checkpoint persistence
* Crash-safe and idempotent indexing
* Ethereum reorg detection and rollback
* Bounded common-ancestor recovery
* ERC-20 token metadata resolution

  * `name`
  * `symbol`
  * `decimals`
* Multi-level token metadata caching

  * in-memory
  * PostgreSQL
  * Ethereum RPC fallback
* Exact token amount formatting with `big.Int`
* HTTP API
* Cursor/keyset pagination
* Token, wallet, and block filters
* Structured JSON logging with `log/slog`
* Prometheus-compatible metrics
* Graceful shutdown
* RPC retry, rate limiting, and backpressure
* Request IDs and panic recovery
* Docker support
* Benchmarks and optional profiling
* Configuration through validated environment variables
* Race-tested concurrent components

---

## Architecture

```text
                         Ethereum RPC
                              │
                              ▼
                    Resilient RPC Client
                              │
                              ▼
                    Concurrent Indexer
                    ┌─────────┴─────────┐
                    │                   │
              Block Fetching       ERC-20 Decoding
                    │                   │
                    └─────────┬─────────┘
                              ▼
                    Ordered Persistence
                              │
                              ▼
                         PostgreSQL
                 ┌────────────┼────────────┐
                 │            │            │
                 ▼            ▼            ▼
             Transfers    Checkpoints   Block History
                 │
                 ▼
             Query Layer
                 │
                 ▼
              HTTP API
         ┌───────┼────────┐
         │       │        │
         ▼       ▼        ▼
      /health /transfers /metrics

Token metadata:

Transfer
   │
   ▼
Memory Cache
   │ miss
   ▼
PostgreSQL Cache
   │ miss
   ▼
Ethereum eth_call
```

---

## Important Design Invariants

ChainWatch follows several consistency rules.

### Checkpoints only advance after successful persistence

```text
fetch block
   ↓
decode transfers
   ↓
persist block state atomically
   ↓
advance checkpoint
```

A checkpoint never represents data that was not successfully stored.

### Concurrent fetching does not mean concurrent checkpoint advancement

Workers may fetch and decode blocks concurrently, but completed results are persisted in canonical block order.

```text
workers:
102 ──────┐
100 ──┐   │
101 ────┐ │
        ▼ ▼

commit order:
100 → 101 → 102
```

### Blockchain values remain exact

Raw ERC-20 values are stored as integer values.

For example:

```text
121748369019
```

For a 6-decimal token:

```text
121748.369019
```

ChainWatch does not use floating-point arithmetic for authoritative blockchain values.

---

## Ethereum Reorg Handling

The indexer stores both the block number and block hash associated with its checkpoint.

Before continuing from persisted state, ChainWatch can compare its stored block history against the canonical Ethereum chain.

If a mismatch is detected:

```text
stored chain
    │
    ├── block N
    ├── block N+1
    └── block N+2  ← orphaned

canonical chain
    │
    ├── block N
    ├── block N+1'
    └── block N+2'
```

ChainWatch searches backward for a bounded common ancestor, removes orphaned indexed state, rewinds the checkpoint, and replays the canonical chain.

---

## API

Default HTTP address:

```text
:8080
```

### Health

```http
GET /health
```

Example response:

```json
{
  "status": "ok"
}
```

---

### Transfers

```http
GET /transfers
```

Supported query parameters:

```text
block
token
address
limit
cursor
```

Examples:

```bash
curl "http://localhost:8080/transfers?limit=20"
```

```bash
curl "http://localhost:8080/transfers?block=25871014"
```

```bash
curl "http://localhost:8080/transfers?token=0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48&limit=10"
```

```bash
curl "http://localhost:8080/transfers?address=0x937f958F6B5d12AC71f8A1Feee780b46dA9B1E81"
```

Example response:

```json
{
  "data": [
    {
      "blockNumber": 25871588,
      "blockHash": "0x...",
      "transactionHash": "0x...",
      "logIndex": 313,
      "token": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
      "from": "0x...",
      "to": "0x...",
      "value": "121748369019",
      "formattedValue": "121748.369019",
      "tokenMetadata": {
        "name": "USD Coin",
        "symbol": "USDC",
        "decimals": 6
      }
    }
  ],
  "pagination": {
    "limit": 20,
    "hasMore": true,
    "nextCursor": "eyJ..."
  }
}
```

---

## Cursor Pagination

ChainWatch uses keyset/cursor pagination rather than SQL `OFFSET`.

Results are ordered approximately by:

```sql
ORDER BY
    block_number DESC,
    log_index DESC
```

A cursor represents the last returned position.

Conceptually:

```text
25871082 / log 48
25871082 / log 47  ← cursor
25871082 / log 40
25871081 / log 900
```

The next page continues after the cursor.

This avoids large `OFFSET` scans and remains much more stable while new blocks are being indexed.

---

## Metrics

```http
GET /metrics
```

The endpoint exposes Prometheus-style metrics.

Examples:

```text
chainwatch_http_requests_total
chainwatch_http_errors_total
chainwatch_http_request_duration_seconds_sum
chainwatch_http_request_duration_seconds_count

chainwatch_indexed_blocks_total
chainwatch_indexed_transfers_total
chainwatch_indexer_errors_total

chainwatch_token_metadata_errors_total
```

Example:

```bash
curl http://localhost:8080/metrics
```

---

## Structured Logging

ChainWatch uses Go's `log/slog`.

Example:

```json
{
  "time": "2026-08-31T13:51:41Z",
  "level": "INFO",
  "msg": "indexed block",
  "blockNumber": 25871589,
  "blockHash": "0x...",
  "transfers": 354
}
```

HTTP requests are also logged with information such as:

```text
method
path
status
duration
request ID
```

---

## Requirements

* Go 1.27+
* PostgreSQL
* Docker / Docker Compose for the recommended local setup
* Ethereum RPC endpoint

---

## Quick Start

### 1. Clone the repository

```bash
git clone git@github.com:Soheilprs/ChainWatch.git
cd ChainWatch
```

---

### 2. Start PostgreSQL

```bash
docker compose up -d
```

Default local PostgreSQL configuration:

```text
database: chainwatch
user:     chainwatch
password: chainwatch
port:     5432
```

---

### 3. Configure the environment

At minimum:

```bash
export ETH_RPC_URL="https://your-ethereum-rpc"
export DATABASE_URL="postgres://chainwatch:chainwatch@localhost:5432/chainwatch?sslmode=disable"
```

ChainWatch also supports validated runtime configuration for settings such as:

```text
HTTP address
worker count
poll interval
shutdown timeout
HTTP timeouts
RPC rate limits
profiling/debug configuration
```

See the configuration implementation for the complete supported environment options.

Never commit RPC credentials or production secrets to the repository.

---

### 4. Apply migrations

The project includes SQL migrations for:

```text
checkpoints
ERC-20 transfers
token metadata
block history / reorg state
supporting indexes
```

Apply the migrations to your PostgreSQL database before running the application.

For example:

```bash
docker exec -i chainwatch-postgres \
  psql \
  -U chainwatch \
  -d chainwatch \
  < migrations/001_create_checkpoints.sql
```

Repeat for the remaining migrations in order.

---

### 5. Run ChainWatch

```bash
go run ./cmd/chainwatch
```

Depending on the final repository layout, the application entrypoint may also be runnable through the project's configured build command.

Expected startup logs include:

```text
connected to PostgreSQL
resuming from checkpoint
ChainWatch started
```

---

## Docker

Build:

```bash
docker build -t chainwatch .
```

Run with the required environment configuration:

```bash
docker run \
  --rm \
  -p 8080:8080 \
  -e ETH_RPC_URL="$ETH_RPC_URL" \
  -e DATABASE_URL="$DATABASE_URL" \
  chainwatch
```

The production image uses a multi-stage build and is designed to run with minimal runtime dependencies.

---

## Database Model

### ERC-20 Transfers

Transfer identity is based on:

```text
transaction_hash + log_index
```

This is important because one Ethereum transaction may emit multiple `Transfer` events.

Typical stored fields include:

```text
block_number
block_hash
transaction_hash
log_index
token_address
from_address
to_address
value
```

---

### Checkpoints

Checkpoints track the last successfully persisted canonical block.

Typical fields:

```text
name
block_number
block_hash
updated_at
```

---

### Token Metadata

Metadata is persisted so repeated API requests do not require repeated Ethereum RPC calls.

Stored fields:

```text
token_address
name
symbol
decimals
updated_at
```

---

## Testing

Run formatting:

```bash
go fmt ./...
```

Run vet:

```bash
go vet ./...
```

Run the full test suite:

```bash
go test -count=1 -v ./...
```

Run with the race detector:

```bash
go test -count=1 -race ./...
```

PostgreSQL integration tests require:

```bash
export DATABASE_URL="postgres://chainwatch:chainwatch@localhost:5432/chainwatch?sslmode=disable"
```

and a running PostgreSQL instance.

---

## Benchmarks

Run all benchmarks:

```bash
go test -bench=. -benchmem ./...
```

The project includes benchmarks for selected hot paths and supports profiling for performance analysis.

Useful Go tooling includes:

```bash
go test -bench=. -benchmem ./...
go tool pprof
```

Profiling support should remain disabled or restricted in production unless explicitly configured.

---

## Reliability

ChainWatch includes several safeguards for long-running operation:

* bounded worker pools
* context cancellation
* graceful SIGTERM/SIGINT shutdown
* PostgreSQL transactions
* idempotent persistence
* RPC retries
* rate limiting
* backpressure
* reorg detection
* bounded rollback
* panic recovery
* HTTP timeouts
* structured error handling
* race-tested shared state

---

## Project Structure

The final architecture separates major responsibilities into internal packages.

A simplified layout looks like:

```text
.
├── cmd/
│   └── chainwatch/
│
├── internal/
│   ├── api/
│   ├── config/
│   ├── ethereum/
│   ├── indexer/
│   ├── metadata/
│   ├── observability/
│   └── store/
│
├── migrations/
├── docs/
│   └── PRODUCTION_REVIEW.md
│
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

---

## Production Review

A more detailed review of the final system architecture, invariants, failure modes, and remaining production work is available in:

```text
docs/PRODUCTION_REVIEW.md
```

---

## Known Limitations

ChainWatch is intentionally designed as a strong production-oriented learning project rather than a fully managed indexing platform.

Current limitations include:

* single-writer checkpoint ownership
* process-local rate limiting
* process-local in-memory metadata cache
* liveness-oriented health checks rather than complete dependency readiness checks
* no API authentication
* no distributed worker coordination
* no durable range leasing
* no multi-provider distributed failover architecture
* no managed migration service
* no built-in CI/CD pipeline
* no integrated SBOM/signing workflow
* no integrated backup/restore automation
* no built-in alerting stack

These are natural areas for a distributed version of the system.

---

## What This Project Demonstrates

ChainWatch covers practical Go concepts including:

```text
context.Context
goroutines
channels
worker pools
sync.WaitGroup
sync.Mutex
sync.RWMutex
sync/atomic
interfaces
dependency injection
typed errors
errors.Is / errors.As
net/http
middleware
graceful shutdown
log/slog
PostgreSQL
pgx
transactions
cursor pagination
benchmarks
pprof
Docker
```

And blockchain infrastructure concepts including:

```text
Ethereum RPC
block indexing
transaction receipts
event logs
ERC-20 decoding
ABI calls
big.Int
token metadata
checkpointing
idempotency
chain reorganizations
canonical block hashes
common-ancestor recovery
ordered persistence
RPC resilience
```

---

## Why ChainWatch?

The main purpose of ChainWatch is not just to decode Ethereum events.

It demonstrates how a long-running blockchain backend can maintain correctness while dealing with:

```text
concurrency
process crashes
database failures
RPC instability
duplicate processing
chain reorganizations
high-volume queries
API traffic
shutdown signals
```

The project prioritizes recoverability and deterministic behavior over simply maximizing throughput.

---

## Future Work

Possible extensions include:

* multi-chain indexing
* distributed indexing workers
* durable block-range leasing
* leader election
* Kafka / NATS / Redis Streams
* OpenTelemetry tracing
* multiple RPC provider failover
* distributed rate limiting
* authenticated APIs
* partitioned PostgreSQL tables
* historical backfill jobs
* WebSocket subscriptions
* contract/event subscriptions beyond ERC-20
* Kubernetes deployment
* CI benchmark regression testing
* chaos/fault-injection testing

---

## License

Add the appropriate open-source license before distributing or accepting external contributions.

---

## Author

Built by [Soheil Parsaei](https://github.com/Soheilprs) as a practical Go and blockchain infrastructure project.
