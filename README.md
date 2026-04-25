# algorhythm-results-api

Read-only HTTP API over ClickHouse for backtest results: run summary, trades, equity curve, compare runs / strategy versions.

**Module:** `github.com/algorhythm-llc/algorhythm-results-api`  
**Repository:** `https://github.com/Algorhythm-LLC/results-api.git`

In the **Algorhythm** meta-repo this service is included as a **git submodule** at `services/results-api`.

## Configuration

Copy `.env.example` to `.env` or set:

| Variable | Description |
|----------|-------------|
| `RESULTS_API_HTTP_PORT` | Listen port (default `8082`) |
| `RESULTS_API_CLICKHOUSE_DSN` | Native ClickHouse DSN for clickhouse-go |
| `RESULTS_API_API_KEYS` | Optional. Comma-separated API keys. When non-empty, all routes except `/healthz` and `/readyz` require header `X-API-Key`. When empty, no auth (dev default). |
| `RESULTS_API_SUMMARY_CACHE_SIZE` | Max entries for the in-memory LRU cache of `GET /api/v1/runs/{run_id}/summary` responses. Default `256`. Set to `0` to disable the cache. |
| `RESULTS_API_SUMMARY_CACHE_TTL` | How long a cached run summary is considered fresh (e.g. `60s`, `2m`). Default `60s`. Invalid or empty values keep the default. |

`cmd/api/summarycache.go` implements LRU (hashicorp/golang-lru/v2) plus TTL per entry. Errors from ClickHouse (including missing run) are not cached.

## API

OpenAPI: [`openapi/openapi.yaml`](openapi/openapi.yaml).

## Run locally

```bash
go run ./cmd/api
```

Or: `make run`

## Build / Docker

```bash
make build
docker build -t results-api:local .
```

## Health

- `GET /healthz` — process up  
- `GET /readyz` — ClickHouse ping  

## Migration note

Imported from Algorhythm meta-repo MVP; submodule wiring: [stage-6-1-results-api-submodule.md](https://github.com/Algorhythm-LLC/Algorhythm/blob/dev/docs/stages/stage-6-1-results-api-submodule.md) (path in meta `docs/stages/`).
