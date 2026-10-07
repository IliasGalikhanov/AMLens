# AMLens — transfer network analysis

AMLens helps analysts explore financial transfer networks: import three Parquet files, inspect the entire network or a client's neighborhood, review heuristic roles, clusters and priorities, open client details, export CSV, and optionally ask an AI assistant. Analyses are saved and can be reopened from history.

**Stack:** Go, React, TypeScript, Cytoscape, SQLite, Docker Compose and Caddy. Python is not required. The project grew out of a team prototype at HackAlem AI; the Go migration, global graph and standalone deployment work were developed after the hackathon.

**Languages:** English (default), Russian and Kazakh. Choose a language in the header; the browser remembers your choice. Interface text, dates, number formatting, API explanations and CSV narrative fields follow that choice. Currency remains KZT. Documentation is maintained in English.

Source code: [MIT](LICENSE), copyright AMLens contributors. Third-party notices are preserved separately.

## Quick start with Docker

Install Docker Engine with Compose v2, or Docker Desktop using Linux containers. Run from the repository root:

```sh
docker compose up --build -d --wait
```

Open http://localhost:8080 and upload your three Parquet files using the schema below. The backend is reached through the website; its port is not published separately. The site binds to localhost by default.

### Demo without preparing data

```sh
docker compose -f compose.demo.yaml up --build -d --wait
```

Open http://localhost:8081. On first startup, Go generates **fully synthetic** Parquet files and runs the actual analysis: 60 nodes, 81 edges and 162 transactions. The dataset includes disconnected components, chains, cycles, isolated nodes and long int64 identifiers. No original hackathon data is used. The server disables imports, analysis activation and AI; graph exploration, client details and CSV downloads remain available.

The regular application and demo use separate Compose projects and volumes. Do not combine their Compose files. `VITE_API_MOCK` is a separate frontend development mode and is not used by this demo.

Stop while keeping saved analyses:

```sh
docker compose down
docker compose -f compose.demo.yaml down
```

`down --volumes` deletes saved analyses. Ordinary restart/down preserves them.

For AI in Docker, copy the root `.env.example` to `.env`, set `OPENAI_API_KEY` and `OPENAI_MODEL`, then run `docker compose up -d` again. Direct Go execution uses **backend/.env** and **--env-file .env** instead.

[HTTPS deployment, updates and backups](docs/deployment.md) · [Architecture](docs/architecture.md) · [Verification and performance](docs/verification.md) · [Dependency notices](THIRD_PARTY_NOTICES.md)

## Development without Docker

Requirements: Go 1.25.5+ and Node.js 24.11–24.x.

First terminal, starting from the repository root:

```sh
cd backend
go mod download
go run ./cmd/amlens serve
```

Second terminal:

```sh
cd frontend
npm ci
npm run dev
```

Open the Vite address, normally http://localhost:5173. The backend listens at http://127.0.0.1:8000; Vite proxies `/api`. Health endpoint: http://127.0.0.1:8000/api/health.

Upload your `nodes.parquet`, `edges.parquet` and `transactions.parquet`. The workspace is empty until an analysis is loaded. AI is optional; graph analysis and export work without it.

For a synthetic demo, run `go run ./cmd/amlens serve --demo` instead. To generate files you can import yourself, run `go run ./cmd/amlens demo --output-dir data-demo` from `backend`. Existing files are not replaced.

## Required input data

**data.zip**, **starter.zip**, renamed copies, extracted hackathon materials and results derived from those materials **are not included or required**. Do not add them to this repository. Use your own data that you are authorized to process.

Provide three Apache Parquet files:

| File | Column | Type | Meaning |
|---|---|---|---|
| nodes.parquet | gid | int64 | Unique pseudonymous node identifier |
| | depth | signed int8/int16/int32/int64 | Traversal depth, 0–4 |
| | is_seed | bool | true exactly when depth=0 |
| edges.parquet | src, dst | int64 | Sender and recipient referencing nodes |
| | sum_kzt | float64 | Finite non-negative total for the directed pair, KZT |
| | n_tx | int64 | Positive transaction count for the pair |
| | depth | signed int8 | Edge depth, 1–4 |
| transactions.parquet | src, dst | int64 | A pair present in edges |
| | date | Parquet DATE | Calendar date; physical INT32, days since 1970-01-01 |
| | sum_kzt | float64 | Finite non-negative amount for an individual transaction, KZT |

Required values cannot be null. Node `gid` values and directed `src`/`dst` pairs in edges must be unique. Duplicate individual transactions are allowed. Additional columns are ignored. Numeric strings and TIMESTAMP columns are not converted automatically. Arrow date64 written as standard Parquet DATE also uses physical INT32.

Any reporting period is accepted: the former July 2026 and minimum 5,000 KZT restrictions were removed. Bank coverage, export filters and account balances are unknown. Edge amounts and `n_tx` are not reconciled against transactions automatically; prepare consistent tables yourself.

### Empty templates

From `backend`:

```sh
go run ./cmd/amlens template --output-dir data
```

This creates **three empty Parquet files with the required types**, without clients or transfers. Populate your own tables using a Parquet-capable tool. Existing files are not overwritten. The `data` directory and all `.parquet` files are excluded from Git.

The [input schema reference](docs/input-schema.json) describes types and constraints only, without customer records. API inputs remain Parquet files.

## CLI and build

```sh
cd backend
go run ./cmd/amlens validate --data-dir data
go run ./cmd/amlens analyze --data-dir data --output-dir out/result-1
go build -o bin/amlens ./cmd/amlens
```

On Windows, use `bin/amlens.exe` for the executable. `analyze` creates a new directory containing `nodes_roles.csv`, `clusters.csv` and `top_nodes.csv`. It does not replace an existing results directory. Exit codes: 0 success, 1 data/runtime error, 2 invalid arguments. The CLI defaults to English; the website supports all three languages.

## AI and secrets

Create `backend/.env` from `backend/.env.example` and set `OPENAI_API_KEY` and `OPENAI_MODEL`. Choose a model available to your API project that supports the Responses API with Structured Outputs. Keys belong on the backend, never in frontend code or `VITE_*` variables.

```sh
cd backend
go run ./cmd/amlens serve --env-file .env
```

The file is read only when explicitly specified, without executing commands. Existing environment variables take precedence. AI is disabled without configuration.

AI receives the question, up to five selected nodes, five neighbors and ten nearby edges. Raw transactions and the full graph are not sent. The server validates references, but cannot guarantee every statement in the generated answer. The assistant is instructed to answer in the selected language. Changing language clears the displayed answer and cancels waiting; it does not send a new paid request. [OpenAI format documentation](https://developers.openai.com/api/docs/guides/structured-outputs).

## Publication checks

```sh
node scripts/check-publication.mjs
```

The check looks for known key formats, non-empty example secrets and forbidden files among publication candidates. With Git, it checks tracked and non-ignored files, including staged content and a previously added `.env`. It does not print secret values.

The root `.gitignore` excludes `.env`, keys, archives, Parquet, spreadsheets, databases, input data and results, including `data.zip` and `starter.zip`. It does not remove earlier commits or prevent `git add -f`. Revoke and replace any token that was previously published.

This repository has its own history. See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution and commit conventions.

## Checks and limitations

```sh
cd backend
go test ./...
go vet ./...
cd ../frontend
npm test
npm run build
cd ..
node --test scripts/check-publication.test.mjs
node scripts/check-publication.mjs
```

Tests generate synthetic Parquet in temporary directories and mock AI. They use neither hackathon data nor paid model calls.

Run `node scripts/smoke-compose.mjs` for the complete container workflow. It creates temporary Compose projects, checks import, CSV, client details, history, restart recovery, backup/restore and demo isolation, then removes its own containers and volumes. Ports 18080/18081 must be free, or override them with `SMOKE_APP_PORT`/`SMOKE_DEMO_PORT`.

The application supports one shared workspace and one API process. Authentication and user data isolation are not implemented. Keep the regular workspace local or behind a VPN; use the synthetic demo for a public portfolio. Snapshots and CSV are stored in SQLite, and the active analysis is restored on restart. History lists the latest 100 entries; old records are not deleted automatically. Temporary inputs are removed after processing.

Input limits: 25 MiB per file, 76 MiB per request, 100,000 nodes, 250,000 edges, 1,000,000 transactions and 256 MiB decoded data per file. These are admission limits, not a promise of smooth rendering at the maximum size on every device.

Roles and priorities are review heuristics, not probabilities of wrongdoing or findings of guilt. The Go Louvain implementation uses a fixed seed and stable ordering; cluster boundaries and numbers can differ from the former NetworkX implementation. See the [backend guide](backend/README.md) and [HTTP contract](backend/docs/data_contract.md).
