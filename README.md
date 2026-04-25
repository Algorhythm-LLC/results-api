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

See `cmd/api/main.go` for parsing details.

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
