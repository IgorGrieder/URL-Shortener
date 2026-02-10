# Encurtador URL (High TPS + Outbox + Kafka)

Implementation-first README for the current codebase.

## What Is Implemented

This repository currently ships 3 Go binaries:

- `cmd/api_hightps`: HTTP API for create/delete/stats/redirect
- `cmd/outbox_worker`: publishes pending click events from Mongo outbox to Kafka
- `cmd/click_consumer`: consumes click events from Kafka and updates aggregates in Mongo

Core behavior:

- Strong consistency for redirect target lookup (`GET /{slug}`)
- Eventual consistency for click metrics (`GET /api/links/{slug}/stats`)
- Optional link expiration (`expiresAt`)
- Optional API key protection for write routes (`POST`/`DELETE`) when `API_KEYS` is set
- Observability: Prometheus metrics, structured logs, OpenTelemetry traces

## Runtime Flow

```mermaid
flowchart LR
  C["Client"] --> G["Kong :8080"]
  G --> N["Nginx"]
  N --> A["API instances (x5)"]
  A --> M1[("Mongo: links")]
  A --> O[("Mongo: click_outbox")]
  O --> W["outbox-worker"]
  W --> K[("Kafka: clicks.recorded")]
  K --> C2["click-consumer"]
  C2 --> M1
  C2 --> M2[("Mongo: clicks_daily")]
```

Redirect click path:

1. `GET /{slug}` resolves active link and responds immediately with redirect (`301` or `302`).
2. API appends click event to `click_outbox` in Mongo.
3. Outbox worker publishes event to Kafka.
4. Consumer increments `links.clicks` and `clicks_daily`.

## HTTP API

Routes registered in `internal/transport/http/router.go`:

- `GET /health`
- `GET /metrics`
- `POST /api/links`
- `DELETE /api/links/{slug}`
- `GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `GET /{slug}`

Notes:

- `POST /api/links` and `DELETE /api/links/{slug}` require `X-API-Key` only when `API_KEYS` is configured.

### Create Link

`POST /api/links`

Request:

```json
{
  "url": "https://example.com/page",
  "notes": "campaign-a",
  "expiresAt": "2026-12-31T23:59:59Z"
}
```

Validation:

- `url` must be `http` or `https`
- `expiresAt` (if provided) must be in the future

### Redirect

`GET /{slug}`

Responses:

- `301` or `302` with `Location` header on success (configurable by `REDIRECT_STATUS`)
- `404` if slug does not exist
- `410` if link exists but is expired

### Stats

`GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`

Rules:

- `from` and `to` are required
- date format must be `YYYY-MM-DD`
- `from <= to`
- returns zero-filled days in range

## Response Envelope

Most API routes (`POST /api/links`, `DELETE /api/links/{slug}`, `GET /api/links/{slug}/stats`) use:

```json
{
  "responseTime": "2026-02-10T12:00:00Z",
  "correlationId": "uuid",
  "code": "LINK_CREATED",
  "data": {}
}
```

Error shape:

```json
{
  "responseTime": "2026-02-10T12:00:00Z",
  "correlationId": "uuid",
  "error": "INVALID_URL",
  "message": "Invalid URL (must be http or https)"
}
```

`GET /health` is a simpler legacy shape (`{"data": {...}}`), and `GET /metrics` returns Prometheus text format.

## Quick Start (Docker Compose)

Starts:

- Kong + Nginx
- API (`app1`..`app5`)
- outbox worker
- click consumer
- MongoDB
- Kafka
- Jaeger
- Prometheus
- Grafana
- Redis (present in compose, currently unused by application code)

Commands:

```bash
make docker-build
# or: docker compose up -d --build
```

Main ports:

- API gateway: `http://localhost:8080`
- MongoDB: `localhost:27017`
- Kafka: `localhost:9092`
- Jaeger UI: `http://localhost:16686`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`

Smoke check:

```bash
curl -i http://localhost:8080/health

curl -i -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

## Local Run (Without Compose)

Prerequisites:

- Go `1.25+`
- MongoDB running locally
- Kafka only needed for async metrics pipeline (outbox worker + consumer)

Run binaries:

```bash
make run
make run-outbox-worker
make run-click-consumer
```

Equivalent:

```bash
go run ./cmd/api_hightps
go run ./cmd/outbox_worker
go run ./cmd/click_consumer
```

## Configuration

### API (`cmd/api_hightps`)

| Variable | Default | Description |
| --- | --- | --- |
| `APP_NAME` | `encurtador-url` | Service name |
| `APP_VERSION` | `0.1.0` | Service version |
| `APP_ENV` | `development` | Logger mode |
| `LOG_LEVEL` | `info` | Reserved (not used to change zap level yet) |
| `APP_HOST` | `localhost` | Host metadata |
| `APP_PORT` | `8080` | HTTP listen port |
| `MONGODB_URI` | `mongodb://localhost:27017` | Mongo URI |
| `MONGODB_DATABASE` | `encurtador` | Mongo DB name |
| `SHORTENER_BASE_URL` | `http://localhost:8080` | Base URL used in `shortUrl` response |
| `SLUG_LENGTH` | `6` | Slug size (`4..32`) |
| `REDIRECT_STATUS` | `302` | Redirect code (`301` or `302`) |
| `API_KEYS` | _(empty)_ | CSV of allowed API keys for write endpoints |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` | OTLP HTTP endpoint |

### Outbox Worker (`cmd/outbox_worker`)

| Variable | Default |
| --- | --- |
| `APP_ENV` | `production` |
| `APP_NAME` | `encurtador-url` |
| `APP_VERSION` | `0.1.0` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://jaeger:4318` |
| `MONGODB_URI` | `mongodb://localhost:27017` |
| `MONGODB_DATABASE` | `encurtador` |
| `KAFKA_BROKERS` | `kafka:9092` |
| `KAFKA_CLICK_TOPIC` | `clicks.recorded` |
| `OUTBOX_POLL_INTERVAL` | `250ms` |
| `OUTBOX_BATCH_SIZE` | `200` |
| `OUTBOX_WRITE_TIMEOUT` | `5s` |
| `OUTBOX_RETRY_BASE_DELAY` | `1s` |
| `OUTBOX_RETRY_MAX_DELAY` | `30s` |
| `OUTBOX_IDLE_WAIT` | `50ms` |

### Click Consumer (`cmd/click_consumer`)

| Variable | Default |
| --- | --- |
| `APP_ENV` | `production` |
| `APP_NAME` | `encurtador-url` |
| `APP_VERSION` | `0.1.0` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://jaeger:4318` |
| `MONGODB_URI` | `mongodb://localhost:27017` |
| `MONGODB_DATABASE` | `encurtador` |
| `KAFKA_BROKERS` | `kafka:9092` |
| `KAFKA_CLICK_TOPIC` | `clicks.recorded` |
| `KAFKA_CLICK_GROUP_ID` | `click-analytics` |
| `KAFKA_CONSUMER_MAX_WAIT` | `500ms` |
| `KAFKA_CONSUMER_OPERATION_TIMEOUT` | `5s` |
| `KAFKA_CONSUMER_BACKOFF` | `500ms` |

## Data Model (Mongo)

- `links`: source of truth for slug -> URL, expiration, notes, API key, total clicks
- `click_outbox`: pending/sent click events for async delivery
- `clicks_daily`: aggregate daily click counters (`slug`, `date`, `count`)

## Load/Functional Test (k6)

Script: `tests/k6/api_gateway_crud.js`

Commands:

```bash
make k6-crud-smoke
make k6-crud
```

Useful variables:

- `LT_BASE_URL` (default `http://localhost:8080`)
- `LT_X_USER` (default `k6-crud`)
- `LT_API_KEY` (set if `API_KEYS` is configured)
- `LT_VUS`, `LT_ITERATIONS`, `LT_MAX_DURATION`, `LT_HTTP_TIMEOUT`

## Repository Map

- `cmd/`: binary entrypoints
- `internal/processing/links`: domain service
- `internal/storage/mongo`: repositories
- `internal/transport/http`: handlers, router, middlewares
- `monitoring/`: Prometheus and Grafana provisioning
- `tests/k6/`: functional load scripts

## Current Status

- `go test ./...` passes (there are currently no Go test files in the repository).
