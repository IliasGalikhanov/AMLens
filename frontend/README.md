# AMLens frontend

React, TypeScript, Vite, Cytoscape.js and plain CSS. The single-page analyst workspace connects to the Go HTTP API.

## Development

Use Node.js 24.11.0 (see `.nvmrc`) and npm 11.6.1. From `frontend`:

```sh
npm ci
npm run dev
```

Real API mode is the default. Before data is uploaded, the workspace is empty. Backend connection failures are shown with a retry action.

Start Go separately from `backend` (Go 1.25.5+). Example for PowerShell:

```powershell
go mod download
if (-not (Test-Path .env)) { Copy-Item .env.example .env }
go run ./cmd/amlens serve --env-file .env
```

Vite proxies `/api` to http://127.0.0.1:8000 and normally serves the frontend at http://127.0.0.1:5173. For another API origin, copy `.env.example` to `.env` and set `VITE_API_BASE_URL`. Direct cross-origin requests also require the frontend origin in backend `CORS_ORIGINS`. Restart Vite after changing its environment.

Configure AI in `backend/.env` as described in the [backend guide](../backend/README.md). Go only reads that file when `--env-file .env` is supplied. Restart the backend after changes. Never put keys in `VITE_*` variables. Graph analysis works without AI.

AI readiness is checked when opening the assistant and with “Check connection”. This does not reset the graph or selected client. Network errors and missing configuration are distinguished. `ai_configured=true` confirms server settings; the key and model are exercised only when sending a question.

Docker Compose already includes Caddy as a reverse proxy. For another deployment, configure a proxy or set the full `VITE_API_BASE_URL` before building. Vite's development proxy is not part of the production bundle. One API process serves a shared workspace without authentication; SQLite history survives restart.

## Localization

English is the initial language. The header selector provides English, Russian and Kazakh. The choice is stored as `amlens.language` in local storage and synchronized across tabs. Blocked storage does not prevent changing the current session language.

`src/i18n/messages.json` contains the three translations. `core.ts` handles preferences and interpolation; `react.ts` provides the subscription hook. Components translate visible labels and accessible names. Dates, amounts and scores use the active locale; currency remains KZT. The document title and HTML language also update.

API requests carry `Accept-Language`. Switching language refreshes analysis descriptions while preserving the chosen client and list filters. Existing assistant answers are cleared and pending answers are cancelled; no replacement AI request is sent automatically. User questions, filenames and identifiers are not translated.

## API integration

The canonical contract is [backend/docs/data_contract.md](../backend/docs/data_contract.md).

| Endpoint | Purpose |
|---|---|
| GET /api/health | Availability, analysis readiness and AI configuration |
| POST /api/analyze | Three multipart files; response after calculation |
| GET /api/analysis | Full nodes, edges, clusters, rankings and summary |
| GET /api/analyses | Saved analysis history |
| POST /api/analyses/{id}/activate | Restore a saved analysis |
| GET /api/nodes/{gid} | Client metrics, connections, limitations and data gaps |
| GET /api/exports/{name} | nodes_roles.csv, clusters.csv, top_nodes.csv |
| POST /api/ask | Question using the active analysis ID and selected client |

HTTP requests and API environment handling live in `src/shared/api`. Wire DTOs are in `types.ts`; UI contracts are in `src/shared/contracts.ts`. `analysisModel.ts` adapts data to the graph. Int64 identifiers stay strings during search, comparison and transmission. Errors are displayed in the relevant operation; obsolete requests are cancelled.

`VITE_API_MOCK=true` explicitly enables synthetic frontend JSON fixtures. Its graph only contains fixture edges, and its detailed example card uses gid 1005. Upload and AI are disabled. The application never silently switches to mock data on an API failure. The Compose demo instead runs the real Go analysis on generated Parquet.

## Analyst workflow

1. Open file import and choose or drop three Parquet files. Local checks validate extension, the 25 MiB size limit, PAR1 markers and footer boundaries; column validation belongs to the server.
2. “Upload and analyze” sends the complete dataset. The API is synchronous, with no precise progress percentage or job polling. Slots become ready after the service confirms completion. Failed imports preserve the previous analysis.
3. Statistics cover the full analysis, including isolates, and volume comes from all directed edges. List filters do not change these totals or truncate the global graph.
4. Search exact gid values, filter roles/clusters and inspect a client card. Role and priority explanations remain hypotheses to verify. A local neighborhood can be expanded; the global view includes all clients and links.
5. Export the filtered list or a server-generated CSV. The client checks the analysis version before accepting a server export. Questions go to AI only after an explicit send action.

The graph uses a Web Worker and Barnes–Hut repulsion. WebGL 2 has a Canvas fallback. Settled layouts stop computing and resume when dragged or adjusted. Changing the network cancels obsolete work. Large dense graphs may still be slow; zoom in or inspect a local neighborhood for detail.

Side panels collapse independently. Below 1100 px they open one at a time over the graph and close with Escape. Lists and cards scroll locally. Import and assistant expansion extend the page; reduced-motion preferences are respected.

## Verification

```sh
npm test
npm run build
```

Node tests cover HTTP errors, int64 preservation, global graph completeness, disconnected components, local limits, statistics, filters, Parquet envelopes, CSV escaping, translation completeness, language preferences and localized API headers. No external AI requests are made.

From `backend`, also run `go test ./...` and `go vet ./...` for the real API integration tests. Manual checks should cover import, long-ID search, graph/card selection, exports, invalid reimport, history and reload. Verify each language, preference persistence, server errors and AI without configuration. A real AI answer requires a configured key and a deliberate user request.

All fixtures are synthetic. Hackathon materials and results derived from them are excluded.
