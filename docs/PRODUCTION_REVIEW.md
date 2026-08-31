# ChainWatch production review

This document records the production-readiness review at the end of the first ChainWatch course. It describes what the service guarantees today and distinguishes those guarantees from future scaling work.

## 1. Final architecture

```text
validated environment configuration
                 |
                 v
        application composition
                 |
       +---------+----------+
       |                    |
       v                    v
resilient Ethereum      HTTP API
RPC adapter             /health
       |                 /transfers
       |                 /metrics
       v
bounded concurrent          optional loopback-only
range indexer               pprof server
       |
       v
ordered block commit
       |
       v
atomic PostgreSQL transaction
  transfers + block history + checkpoint
```

`main.go` owns process concerns: structured logging, signal handling, non-zero failure exits, and calling the application bootstrap. `application.go` validates configuration and composes the Ethereum, PostgreSQL, indexing, metadata, API, metrics, and profiling components. `internal/lifecycle` supervises long-running services, cancels siblings after a failure, invokes shutdown hooks concurrently, joins errors, and bounds shutdown time.

The domain remains framework-free. Small interfaces are declared near their consumers, while concrete adapters handle Ethereum RPC and PostgreSQL. This keeps indexers and services deterministic under mocks without hiding the transaction and ordering rules behind a web framework or ORM.

## 2. Major Go concepts demonstrated

- Structs, methods, custom types, constructors, pointer receivers, defensive copying, and validation.
- Small consumer-owned interfaces and compile-time implementation assertions.
- Sentinel, typed, wrapped, classified, public, and joined errors using `errors.Is` and `errors.As`.
- `context.Context` propagation through RPC, database, worker, service, and shutdown boundaries.
- Goroutines, channels, worker pools, cancellation, backpressure, `sync.WaitGroup`, mutexes, and atomics.
- Ordered aggregation of concurrently produced results.
- Transactional PostgreSQL access with `pgx`, parameterized SQL, rollback cleanup, and integration tests.
- `net/http` middleware, panic recovery, structured JSON errors, request IDs, timeouts, and graceful shutdown.
- Structured `slog` logging and bounded-cardinality Prometheus-style metrics.
- Table tests, deterministic mocks, database integration tests, benchmarks, CPU/allocation profiles, and the race detector.
- Multi-stage static container builds, non-root runtime execution, health checks, and environment-based deployment.

## 3. Blockchain and backend concepts demonstrated

- Ethereum block, transaction, receipt, and log retrieval through go-ethereum.
- Sender recovery, contract-creation transactions, gas-limit versus actual-gas-used semantics, and exact `big.Int` values.
- Correct ERC-20 `Transfer` decoding while rejecting the ERC-721 topic/data layout.
- Transaction hash plus block-global log index as transfer identity.
- Confirmation depth, canonical block hashes, parent-hash continuity, reorg detection, bounded ancestor search, atomic rollback, and replay.
- Exact raw token values plus non-floating-point human formatting.
- Memory, PostgreSQL, and RPC layers for ERC-20 metadata enrichment; enrichment failure does not suppress raw transfers.
- Cursor/keyset pagination with stable descending block/log ordering instead of growing `OFFSET` scans.
- Bounded RPC retry, exponential backoff, jitter, retryability classification, rate limiting, and cancellation.

## 4. Key invariants

1. A checkpoint is the last successfully committed block, never the last requested block.
2. Transfers, indexed-block history, and checkpoint advancement commit in one PostgreSQL transaction.
3. Concurrent workers may fetch and decode out of order; persistence advances only in contiguous block order.
4. A failed block prevents all later blocks in that range from being committed.
5. Transfer writes are idempotent on `(transaction_hash, log_index)`.
6. Startup verifies the checkpoint hash against the canonical chain before resuming at `checkpoint + 1`.
7. A detected reorg rolls transfers, block history, and checkpoint back atomically to a recorded common ancestor.
8. Reorgs deeper than the configured bound fail closed instead of deleting unbounded history.
9. Raw blockchain integers remain exact strings or `big.Int`; derived formatting never becomes authoritative.
10. Internal causes are logged but HTTP responses expose only safe messages and correlation IDs.
11. Request IDs may appear in logs and responses, never as metric labels.
12. Profiling is disabled by default, uses a separate listener, and accepts loopback binds only.
13. A fatal startup or runtime error exits the process non-zero so an orchestrator can detect failure.

## 5. Failure scenarios

| Scenario | Current behavior |
|---|---|
| Required configuration is missing or malformed | Startup fails before dependencies are created; the process exits with status 1. |
| PostgreSQL is unavailable at startup | Pool creation/ping fails and the process exits non-zero. |
| A temporary RPC error occurs | The bounded retry policy applies backoff/jitter while honoring cancellation and rate limits. |
| A permanent RPC error occurs | It is not retried indefinitely; indexing stops and the lifecycle supervisor shuts down sibling servers. |
| Context or SIGTERM is received | Indexing is canceled, HTTP/pprof shutdown hooks run, and all service goroutines are awaited within the configured timeout. |
| Transfer persistence fails before checkpoint update | The entire block transaction rolls back; the checkpoint does not advance. |
| The process dies during a block commit | PostgreSQL commits all block state or none of it; restart reconciles and resumes from durable state. |
| Workers finish blocks out of order | Results wait in a finite per-range pending map until the next contiguous block is available. |
| Parent hash changes while indexing | The range stops with a chain-reorg error before the conflicting block is committed. |
| Stored checkpoint is no longer canonical | Startup searches recorded history, atomically rolls back to the common ancestor, then replays. |
| Reorg history is missing or too deep | ChainWatch fails closed for operator intervention. |
| Metadata lookup fails | The raw transfer remains in the response; metadata is omitted and an error metric/log is emitted. |
| An HTTP handler panics | Recovery logs the panic and stack with the request ID and returns a safe structured 500 when possible. |
| A supervised service exits unexpectedly | Siblings are canceled and the unexpected exit is returned as a process failure. |
| Shutdown does not finish in time | The lifecycle package returns a deadline error identifying remaining run/shutdown work. |

## 6. Remaining limitations

- ChainWatch is a single-chain, single-process indexer with one logical checkpoint. Horizontal indexer replicas would contend rather than coordinate ownership.
- PostgreSQL migrations are initialized automatically only for a new Compose volume; an existing production database needs a dedicated migration job/tool.
- The in-memory metadata cache has no size bound, expiry, negative caching, or request coalescing, so a large token universe can grow memory and simultaneous misses can duplicate RPC calls.
- `/health` is a liveness response, not a dependency-aware readiness check or an indexing-lag signal.
- The HTTP API has no authentication, authorization, tenant isolation, quotas, or configurable CORS policy.
- TLS is expected to terminate at a trusted reverse proxy or load balancer.
- Metrics are intentionally minimal counters/sums rather than native Prometheus histograms with route/status dimensions.
- Reorg reconciliation is bounded by locally retained `indexed_blocks` history and the configured maximum depth.
- RPC rate limiting is process-local; multiple replicas need a provider-aware shared budget or independently allocated quotas.
- There is no dead-letter workflow for permanently undecodable provider data and no automatic alternate-provider failover.
- Most feature code remains in the root package. Further package extraction should follow real ownership/deployment needs, not move types solely for visual symmetry.
- Automated CI, vulnerability scanning, signed images/SBOMs, alert rules, dashboards, backups, and restore drills remain deployment-platform work.

## 7. Additions for large-scale production

1. Introduce leader election or durable range leasing so multiple indexer replicas can divide work without violating ordered checkpoint ownership.
2. Separate ingestion from query serving, allowing independently scaled API replicas and a controlled indexing writer.
3. Add a managed migration workflow with advisory locking, schema-version reporting, rollback policy, and deployment gates.
4. Add dependency-aware readiness, finalized-head/indexed-head lag metrics, provider latency/error histograms, and alerting objectives.
5. Bound and instrument metadata caches; coalesce concurrent misses and define expiry/refresh behavior.
6. Add authenticated APIs, request budgets, trusted proxy handling, response compression limits, and explicit CORS at the edge.
7. Support multiple Ethereum providers with health scoring, circuit breaking, failover, and per-provider quotas.
8. Retain enough canonical history for the operational reorg policy and define a manual deep-reorg recovery runbook.
9. Add continuous integration for formatting, vet, unit/integration/race tests, migration checks, container builds, vulnerability scanning, and benchmark regression tracking.
10. Produce signed multi-architecture images, an SBOM/provenance attestation, pinned deployment manifests, and least-privilege secrets management.
11. Establish PostgreSQL capacity planning, partitioning/retention policy, backups, point-in-time recovery, replicas, and restore exercises.
12. Run load, soak, fault-injection, provider-degradation, database-failover, and shutdown-deadline tests before setting service-level objectives.

## 8. Interview exercises

Use these as discussion prompts; the answer cues identify what a strong explanation should cover.

1. **Why use cursor pagination instead of `OFFSET`?** Stable keyset ordering avoids rescanning/discarding growing prefixes and reduces duplicate/skip behavior while new rows arrive.
2. **Why is the cursor `(blockNumber, logIndex)`?** Those fields match the total response ordering; log index is block-global, so the pair identifies the next boundary.
3. **Why use a bounded worker pool?** It increases RPC throughput without unbounded goroutines, memory, provider pressure, or database backlogs.
4. **How can workers run concurrently while commits stay ordered?** Workers publish block-numbered results; the coordinator buffers completed results and persists only the next contiguous number.
5. **Why must transfer data and checkpoint advancement share a transaction?** Advancing first can permanently skip missing data; atomicity makes the durable checkpoint truthful.
6. **Why is idempotency still important after adding atomic transactions?** Replays, retries, overlapping operational runs, and crash recovery can legitimately attempt the same transfer again.
7. **How does an Ethereum reorg affect an indexer?** Previously canonical hashes can become orphaned; derived rows must be removed back to a common ancestor and replayed from the new chain.
8. **Why use confirmation depth if reorg handling exists?** Waiting reduces normal churn and rollback frequency; reconciliation is the safety net, not a substitute for a finality policy.
9. **Why use `big.Int` and strings for token values?** Ethereum integers can exceed native widths and decimal fractions are derived; floating point cannot preserve authoritative values exactly.
10. **Why cache token metadata?** Metadata changes rarely but requires three RPC calls; layered caching reduces latency, cost, and provider pressure without replacing raw transfer truth.
11. **Why parameterize SQL if filters are validated?** Validation is a domain rule, while parameters are the independent injection boundary and preserve query/value separation.
12. **Why propagate `context.Context`?** Cancellation and deadlines must stop RPC, SQL, workers, retries, rate-limit waits, and shutdown work as one request/process tree.
13. **When should a mutex be used instead of an atomic?** Atomics fit independent scalar counters; maps and multi-field invariants require mutual exclusion.
14. **Why are high-cardinality Prometheus labels dangerous?** Every unique label set creates a time series, so addresses, hashes, and request IDs can exhaust memory/storage and degrade queries.
15. **How does graceful shutdown work here?** Signal cancellation reaches the lifecycle group, siblings are canceled, server shutdown hooks run concurrently, and all goroutines are awaited under one timeout.
16. **Which errors should be retried?** Only temporary/provider/rate/deadline failures where another attempt may succeed; validation and permanent protocol failures should surface immediately.
17. **Why does panic recovery not make panics acceptable control flow?** Recovery protects process availability at an HTTP boundary; the panic remains an internal defect logged with a stack and should be fixed.
18. **What is the first change needed for multiple indexer replicas?** Define durable ownership/coordination of ranges and checkpoint advancement before adding replicas; otherwise concurrency crosses process boundaries unsafely.

## Verification contract

Every lesson commit is accepted only after formatting, vet, the full PostgreSQL-enabled test suite, and the race detector pass. Database-changing lessons additionally apply migrations and run integration coverage; deployment and lifecycle lessons include live startup, health, and graceful-shutdown smoke tests. Benchmarks are measurement baselines, not pass/fail performance claims.
