# k6 Guide for This Project

This guide explains how to run and understand the load test in:

- `tests/k6/api_gateway_tps.js`

## What This Test Does

The script can generate traffic for 4 endpoint types:

- `create`: `POST /api/links`
- `redirect`: `GET /{slug}`
- `stats`: `GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD`
- `health`: `GET /health`

In `mixed` mode, it splits total TPS across these endpoints by percentage.

## Quick Start (Recommended)

1. Start the stack:

```bash
docker compose up -d --build
```

2. Run the local-safe mixed test:

```bash
make k6-gateway
```

Current local-safe profile in `make k6-gateway`:

- `LT_TARGET_TPS=1000`
- `LT_PRE_ALLOCATED_VUS=400`
- `LT_MAX_VUS=4000`
- `LT_MIXED_CREATE_PCT=0`
- `LT_MIXED_REDIRECT_PCT=90`
- `LT_MIXED_STATS_PCT=10`

## Other Ready Commands

- `make k6-gateway-create`
- `make k6-gateway-redirect`
- `make k6-gateway-stats`
- `make k6-gateway-health`
- `make k6-gateway-100k` (aggressive; not recommended on one localhost machine)

## Important Variables

- `LT_MODE`: `mixed|create|redirect|stats|health`
- `LT_BASE_URL`: target URL (default `http://localhost:8080`)
- `LT_DURATION`: test duration (default `1m`)
- `LT_TARGET_TPS`: total target iterations per second
- `LT_PRE_ALLOCATED_VUS`: initial VU pool
- `LT_MAX_VUS`: maximum VUs k6 may allocate
- `LT_HTTP_TIMEOUT`: request timeout (default `5s`)
- `LT_SEED_LINKS`: links created in setup for redirect/stats

Mixed-mode split:

- `LT_MIXED_CREATE_PCT`
- `LT_MIXED_REDIRECT_PCT`
- `LT_MIXED_STATS_PCT`
- `LT_MIXED_HEALTH_PCT`

## How to Read k6 Results

Main health signals:

- `checks`: should stay near 100%
- `http_req_failed`: should stay near 0%
- `http_req_duration p(95)`: latency p95
- `dropped_iterations`: non-zero means k6 could not keep up with requested arrival rate
- `vus` and `vus_max`: if `vus` keeps climbing and hits `vus_max`, the profile is too aggressive

Per-endpoint counters in this script:

- `endpoint_create_requests`
- `endpoint_redirect_requests`
- `endpoint_stats_requests`

## Common Problems and Fixes

### 1) `connect: can't assign requested address`

Cause:

- Load generator (k6 machine) exhausted local sockets/ephemeral ports.

Fix:

- Lower `LT_TARGET_TPS` and `LT_MAX_VUS`.
- Keep localhost tests in low/medium ranges.
- For very high TPS, use distributed generators (multiple hosts/IPs).

### 2) `request timeout`

Cause:

- App/gateway/database saturated, or timeout too short.

Fix:

- Reduce TPS.
- Increase `LT_HTTP_TIMEOUT` (example: `10s` or `15s`).
- Check container CPU/memory and logs.

### 3) `401` responses

Cause:

- Missing `X-User` or API key requirements.

Fix:

- Ensure header is present (`LT_X_USER` is set by default in script).
- Set `LT_API_KEY` if your environment requires it.

### 4) `429` on create

Cause:

- Create endpoint rate limit (default `CREATE_RATE_LIMIT_PER_MINUTE=60`).

Fix:

- For create stress tests, raise limit before starting compose:

```bash
CREATE_RATE_LIMIT_PER_MINUTE=1000000 docker compose up -d --build
```

## Suggested Learning Progression

Use these in order:

1. `LT_MODE=health`, low TPS (sanity check)
2. `LT_MODE=redirect`, then `stats`
3. `LT_MODE=mixed` at `200 TPS`
4. `LT_MODE=mixed` at `1000 TPS`
5. Increase gradually while watching `http_req_failed`, `p95`, and `dropped_iterations`

Example gradual run:

```bash
LT_MODE=mixed LT_TARGET_TPS=300 LT_PRE_ALLOCATED_VUS=120 LT_MAX_VUS=1200 k6 run ./tests/k6/api_gateway_tps.js
```

## Safety Guard in Script

The script intentionally blocks extreme localhost profiles by default.

If you really want to force an extreme localhost run:

```bash
LT_ALLOW_EXTREME_LOCALHOST=true k6 run ./tests/k6/api_gateway_tps.js
```

Only do this if you understand the host/network limits.
