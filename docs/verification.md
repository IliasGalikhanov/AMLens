# Verification and measurements

## Reproduce

```sh
cd backend
go test ./...
go vet ./...
cd ../frontend
npm ci
npm test
npm run build
cd ..
node --test scripts/check-publication.test.mjs
node scripts/check-publication.mjs
node scripts/smoke-compose.mjs
```

GitHub Actions has two jobs: tests/build and a container workflow with clean volumes. Linux CI also runs the Go race detector. Check the actual status in [GitHub Actions](https://github.com/IliasGalikhanov/AMLens/actions/workflows/ci.yml); a workflow definition alone is not evidence of a successful run.

Tests use no hackathon data or paid AI calls. Synthetic cases cover directed transfers, long int64 values, decimal amounts, internal explanations, invalid schemas, size limits, failed SQLite writes, history, refusal to open a private database in demo mode and the demo write restriction.

The container smoke test creates `amlens-check-*` projects, imports three generated Parquet files, checks client details and all CSV files, creates a second analysis, activates the first, rejects an invalid import and restarts the API. It compares graph/card/export results, checks database backup and restore, and verifies the isolated demo. It cleans up only its own containers and volumes. HTTP probes use fresh connections across synchronous container restarts.

Localization tests cover English fallback, saved language preferences, blocked browser storage, all three catalogs, interpolation placeholders, API language headers, weighted language negotiation, every backend message template, CSV translation and preservation of numeric JSON tokens, identifiers, user text and stored snapshots.

## Baseline results, 7 October 2026

Environment: Windows, AMD Ryzen 5 5600H, Node.js 24.11.0, Go 1.25.5; Docker Desktop with Linux Engine 29.4.0. Go tests, go vet, frontend build and the container workflow passed. A synthetic import of 60 nodes / 81 edges / 162 transactions through Caddy took 29 ms in one run. This is a small functional dataset, not a load guarantee.

Browser checks covered real three-file import, results and history, repeated analysis and opening a previous version. The initial release passed 55 frontend tests and five publication checks. Stopped-SQLite backup/restore was tested through Compose. The synthetic demo showed its fictional-data notice, hid import and opened graph/client details without browser console errors.

The published baseline also passed both [GitHub Actions jobs](https://github.com/IliasGalikhanov/AMLens/actions/runs/37612475090), including Go race detection and the Linux container scenario. Subsequent revisions have their own workflow results.

### Server calculation

```sh
cd backend
go test ./tests/integration -run "^$" -bench BenchmarkAnalysis -benchtime=3x -benchmem
```

Synthetic chains of 100 nodes, one transfer per edge, three repetitions. This measures domain validation, Louvain, metrics/roles and CSV preparation. Parquet reading, HTTP localization, network and SQLite are excluded.

| Nodes | Edges / transactions | Mean time | Allocated memory per calculation |
|---:|---:|---:|---:|
| 1,000 | 990 | 16.6 ms | 9.4 MB |
| 5,000 | 4,950 | 82.6 ms | 46.7 MB |
| 10,000 | 9,900 | 159.2 ms | 95.2 MB |

Allocated memory is cumulative benchmark allocation, not peak process memory. Dense networks can behave differently.

### Graph layout physics

```sh
node scripts/benchmark-graph.mjs
```

Synthetic chain, five warm-up and 30 measured steps:

| Nodes | Edges | Median step | P95 step |
|---:|---:|---:|---:|
| 1,000 | 999 | 3.59 ms | 5.33 ms |
| 5,000 | 4,999 | 27.28 ms | 34.31 ms |
| 10,000 | 9,999 | 52.91 ms | 72.19 ms |

This Node measurement reflects layout CPU work. It excludes GPU rendering, Cytoscape drawing, labels, browser overhead and input response; it cannot be interpreted as FPS. A Web Worker keeps layout work off the main thread, but large graphs can still be slow.

## Localization checks, 7 October 2026

The localized revision passed 59 frontend tests, Go tests and go vet, the frontend build and five publication checks locally. Browser checks covered the English default, Russian and Kazakh labels and server explanations, a preserved selected client and search filter, persistence after reload, rapid switching during an in-flight analysis request, and the mobile language selector without horizontal overflow. No browser page errors were observed. AI language instructions were verified with a mock transport; no paid provider request was made.
## Checks requiring an external environment

A real HTTPS certificate requires a domain and server; local checks validate Caddy configuration only. A public deployment has not been performed. A real AI answer requires a user-provided key and a separate, deliberate question submission.
