# AMLens Go backend

Go 1.25.5+, organized as a modular monolith. Entry point: `cmd/amlens`. Python and NetworkX are not required.

## Commands

From `backend`:

```sh
go run ./cmd/amlens serve
go run ./cmd/amlens serve --demo
go run ./cmd/amlens demo --output-dir data-demo
go run ./cmd/amlens serve --env-file .env --addr 127.0.0.1:8000
go run ./cmd/amlens template --output-dir data
go run ./cmd/amlens validate --data-dir data
go run ./cmd/amlens analyze --data-dir data --output-dir out/result-1
go build -o bin/amlens.exe ./cmd/amlens
go test ./...
go vet ./...
```

`template` creates empty files with the required schema and does not replace existing files. `analyze` writes three CSV files into a new directory and refuses an existing output directory. Exit 1 indicates a data/runtime error; exit 2 indicates invalid arguments.

[Input schemas and data preparation](../README.md#required-input-data) · [HTTP API](docs/data_contract.md).

## Architecture

- `internal/domain`: entities, value validation, roles, priorities, explanations and observation limits.
- `internal/application`: validate/analyze/publish/ask use cases, adapter interfaces and an immutable active snapshot protected by a lock.
- `internal/infrastructure`: Parquet, weighted Louvain, CSV, temporary uploads, SQLite and OpenAI adapters.
- `internal/presentation/httpapi`: HTTP, CORS, multipart, JSON validation, statuses and responses.
- `internal/i18n`: presentation translation of explanations and errors; language negotiation and context.
- `cmd/amlens`: dependency wiring, CLI and server lifecycle.

Dependencies point from presentation/infrastructure to application to domain. Domain uses decimal as a numerical type and knows nothing about HTTP, files or AI. Application depends on interfaces rather than concrete adapters.

## Analysis

Connections and monetary totals come from directed edges. Individual operation counts come from transactions, including duplicate rows; transaction amounts are not added to edge volume a second time. Isolated nodes are preserved. Seed neighbors are counted uniquely across incoming and outgoing connections, excluding the node itself.

Amounts use the decimal representation of the input float64 and decimal addition thereafter. Precision already lost in the source float64 cannot be recovered. HTTP returns amounts as numbers and identifiers as exact int64 strings. Volume outside float64 range is rejected before a snapshot is published.

Roles use the first matching rule. I/O are degrees, S is seed-neighbor count, Vin/Vout are amounts, and r=Vout/Vin applies only to non-seeds with Vin>0.

| Role | Rule | Score |
|---|---|---|
| coordinator | I≥3, O≥3, S≥2 | min(1, 0.6+0.05·min(I+O−6,8)) |
| consolidator | I≥3, Vin≥100000, r≤0.6; for seeds only I and Vin apply | min(1, 0.65+0.05·min(I−3,7)) |
| distributor | O≥5, Vout≥100000 | min(1, 0.65+0.05·min(O−5,7)) |
| transit | I>0, O>0, r∈[0.8,1.2] | 0.9−abs(r−1) |
| terminal | I>0, O=0, depth<4 | min(0.85, 0.55+0.05·min(I,6)) |
| peripheral | otherwise | 0.5 |

100000 is a role heuristic, not a minimum input amount. Scores are not probabilities.

Priority combines normalized log1p features: 0.35 for connection count, 0.25 for operation count, 0.25 for Vin+Vout, 0.15 for seed neighbors. Each feature is normalized by its maximum in the current dataset; a zero maximum contributes zero. The top 20 is sorted by descending priority, then gid.

Clustering uses an undirected projection: opposite-direction amounts are combined, self-loops are retained, and weights are normalized by the maximum edge weight. Louvain improves modularity and aggregates communities. Input order is stable and seed=42. Each isolated node forms its own cluster; zero-weight edges do not merge nodes. Cluster IDs follow the minimum gid order. Results can differ from NetworkX because the implementation and traversal differ.

CSV schemas:

- nodes_roles.csv: gid, role, role_score, cluster_id, priority_score, evidence.
- clusters.csv: cluster_id, n_nodes, n_seed, sum_kzt_internal, top_gids, hypothesis.
- top_nodes.csv: rank, gid, role, priority_score, why.

Internal cluster volume counts each original directed edge once. `top_gids` is a JSON array of integers in CSV and of strings in HTTP.

## Persistence and differences from the Python prototype

HTTP fields and routes, role rules, priority formulas, CSV schemas, exact gid values, AI context limits and failed/concurrent import behavior have been preserved.

Dataset-specific restrictions were removed: July 2026, minimum 5000 KZT and an assertion of within-bank coverage. Calendar years 0001–9999 and finite non-negative amounts are accepted. Cards report COVERAGE_UNKNOWN instead of unsupported INTRABANK_ONLY and AMOUNT_THRESHOLD assumptions.

`serve` stores snapshots and three CSV exports in SQLite and restores the active version on restart. Writing and activation are transactional; failed storage preserves the current result. History lists the latest 100 entries without automatically deleting records. Temporary inputs are removed on success and failure. Swagger `/docs` is not generated; the contract is maintained in Markdown. CSV input is unsupported.

## Languages

HTTP defaults to English and supports Russian and Kazakh through `Accept-Language`. Presentation translation covers API errors, explanatory fields, references and CSV narratives while preserving technical codes, exact identifiers and numeric tokens. Existing stored Russian explanations remain readable without migrating or rewriting snapshots. The CLI uses English help and status text; CLI-generated analytical files retain the canonical source explanations. Use HTTP export to select a language for those files.

## OpenAI

Settings: `OPENAI_API_KEY`, `OPENAI_MODEL`, `OPENAI_BASE_URL` (default https://api.openai.com/v1), `OPENAI_TIMEOUT_SECONDS` (1–120, default 30). The URL must use HTTPS without credentials, query or fragment. Redirects are disabled. Use a trusted endpoint because the key is sent there.

Each question makes one POST to `/responses`, without retries, with `store=false`, a strict JSON schema, up to 2500 output tokens and a 128 KiB response limit. The timeout covers the entire call. Context contains up to five selected nodes, five neighbors and ten edges, at most 32 KiB; each selected node's largest outgoing edge is considered first. Server instructions select English, Russian or Kazakh using the request locale.

Invalid, duplicate or unknown references, unsupported gid mentions, refusals and provider errors become 503 AI_UNAVAILABLE without exposing provider errors or keys. The analysis version is checked before and after the call. Reference facts are generated by the server. Reference validation does not establish the truth of generated prose.

See [Responses API](https://developers.openai.com/api/docs/guides/migrate-to-responses) and [Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs). Tests replace the HTTP transport and make no external model calls.

## Operations

The unauthenticated local server defaults to 127.0.0.1:8000. `CORS_ORIGINS` is a comma-separated list, empty by default; Vite uses a local proxy. `ANALYSIS_STORAGE_DIR` defaults to `out/api` for temporary uploads.

`ANALYSIS_DATABASE` defaults to `out/analyses.db` (`out/demo.db` for `serve --demo`). Database read errors stop startup without deleting data. Schema version is `PRAGMA user_version=1`; snapshots use Gob with exact int64/decimal and internal evidence fields. Incompatible future versions require an explicit migration. Demo databases are marked and cannot be opened in regular mode; private databases cannot be opened as demos.

Storage supports one API process. History keeps computed metrics, connections, explanations and exports, not source Parquet files. These are still financial data; do not publish SQLite files.

Do not upload hackathon data. Run `node scripts/check-publication.mjs` from the root before publishing. All test records are synthetic and Parquet fixtures are created in temporary directories.
