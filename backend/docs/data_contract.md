# AMLens HTTP contract (Go)

The API defaults to http://127.0.0.1:8000. In all HTTP JSON, `gid`, `src`, `dst` and `clusters.top_gids` are **decimal int64 strings**. Never convert them through JavaScript Number. Counts, depth, rank and cluster IDs are numbers; monetary amounts are KZT numbers for display.

## Language

Send `Accept-Language: en`, `ru` or `kk`. Regional tags and quality weights are supported, for example `kk-KZ,ru;q=0.5`. Missing or unsupported preferences default to English. Responses include `Content-Language` and `Vary: Accept-Language` alongside the existing origin variation.

Messages, evidence, hypotheses, limitations, data gap descriptions, suggested requests and reference facts follow the selected language. The assistant is instructed to use it too. Stored analyses are not rewritten. Field names, role/error codes, identifiers and numeric values are language-independent. CSV narrative columns are translated; column names and numeric representation stay stable.

## Routes

| Method | Path | Response |
|---|---|---|
| GET | /api/health | status: "ok", analysis_ready: boolean, ai_configured: boolean, demo_mode: boolean |
| POST | /api/analyze | analysis_id, status: "ready", summary, analysis_url: "/api/analysis" |
| GET | /api/analysis | analysis_id, summary, nodes, edges, clusters, top_nodes |
| GET | /api/analyses | Latest 100 records: analysis_id, created_at (UTC ISO 8601), n_nodes, n_edges |
| POST | /api/analyses/{id}/activate | Full selected analysis snapshot; activation is persisted in SQLite |
| GET | /api/nodes/{gid} | analysis_id, node, incoming, outgoing, limitations, data_gaps, next_requests |
| GET | /api/exports/{name} | CSV: nodes_roles.csv, clusters.csv or top_nodes.csv |
| POST | /api/ask | analysis_id, answer, references, limitations |

`ai_configured` confirms valid key/model/URL/timeout settings, not provider availability.

With `demo_mode=true`, only GET/HEAD is available. Writes return 403 `DEMO_READ_ONLY`, including analyze, ask and activation. Demo mode never calls an external model.

## Upload

Use multipart/form-data with exactly three file fields: `nodes`, `edges`, `transactions`. Each filename must end in `.parquet`. Duplicate/extra fields, missing files and invalid contents are rejected. Client filenames are never used as storage paths.

The server enforces 25 MiB per file and 76 MiB per body by actual bytes read. Row counts and decoded size are also limited; see the [README](../../README.md). Types and constraints are in [input-schema.json](../../docs/input-schema.json).

The response is returned after calculation, all CSV preparation and the SQLite transaction. The new analysis ID is published atomically. Failures preserve the previous snapshot; concurrent calculation or activation returns 409 `ANALYSIS_BUSY`. GET requests continue reading the previous snapshot. The active version is restored after restart.

## Result fields

- summary: n_nodes, n_edges, n_transactions, n_seed, n_clusters, edge_volume_kzt.
- node: gid, depth, is_seed, role, role_score, cluster_id, priority_score, evidence, in_deg, out_deg, in_kzt, out_kzt, truncated_by_depth.
- edge: src, dst, sum_kzt, n_tx.
- cluster: cluster_id, n_nodes, n_seed, sum_kzt_internal, top_gids, hypothesis.
- top_node: rank, gid, role, priority_score, why.

Role codes: consolidator, transit, distributor, terminal, coordinator, peripheral. Scores are in [0,1]. Arrays are always JSON arrays, including empty `[]`. All nodes and directed edges are included; there is no pagination.

Client cards include incoming edges (`dst=gid`), outgoing edges (`src=gid`) and string limitations. `data_gaps` contains code/description/evidence; `next_requests` contains gap_code/request/reason, paired one-to-one in the same order. Codes: DEPTH_BOUNDARY, SEED_INCOMING_INCOMPLETE, ISOLATED_NODE, OUTFLOW_EXCEEDS_INFLOW, COVERAGE_UNKNOWN, LIMITED_PERIOD, BALANCES_UNAVAILABLE. Missing transactions are not asserted to exist.

## Export

Content-Type is `text/csv; charset=utf-8`. Content-Disposition provides the filename. X-Analysis-Id identifies the snapshot; the frontend must compare it with the displayed analysis. Configured CORS origins can read these headers.

CSV schemas are stable and gid values are exact. In `clusters.csv`, top_gids is a JSON array of integers; JavaScript consumers must preserve them as strings without passing through Number.

## AI question

Example shape; identifier 1 is illustrative and must be replaced with a node from the current analysis:

```json
{
  "analysis_id": "id-from-api-analysis",
  "question": "Why did this node receive this priority?",
  "context_gids": ["1"]
}
```

`analysis_id` is a non-empty string of up to 128 characters. Trimmed `question` has 1–2000 characters. `context_gids` contains 1–5 distinct int64 strings present in the current analysis. Numeric gid values and unknown request fields are rejected.

The response includes answer text, references with gid and server-generated fact strings, and limitations. Render the answer as text, not HTML. If the analysis changes before or during the call, the API returns 409 `STALE_ANALYSIS`. Missing AI configuration returns 503 while the rest of the API remains available.

## Errors

```json
{"error":{"code":"NO_ANALYSIS","message":"Upload three files and run an analysis first","details":{}}}
```

| HTTP | Codes |
|---|---|
| 403 | DEMO_READ_ONLY |
| 404 | NO_ANALYSIS, ANALYSIS_NOT_FOUND, GID_NOT_FOUND, EXPORT_NOT_FOUND, NOT_FOUND |
| 409 | ANALYSIS_BUSY, STALE_ANALYSIS |
| 413 | FILE_TOO_LARGE |
| 422 | INVALID_SCHEMA, INVALID_QUESTION |
| 503 | AI_UNAVAILABLE |
| 500 | INTERNAL_ERROR |

Validation identifies the file, field and row when known. The server does not return secrets or raw AI provider errors. Cache-Control is `no-store`. Authentication is not implemented; regular mode is intended for local use.
