# k6 Functional CRUD Guide

This guide documents the current k6 functional test for the API exposed through Kong.

## Script

- `tests/k6/api_gateway_crud.js`

## Operations Covered Per Iteration

- `POST /api/links` (create)
- `GET /{slug}` (read redirect)
- `GET /api/links/{slug}/stats?from=YYYY-MM-DD&to=YYYY-MM-DD` (read stats)
- `DELETE /api/links/{slug}` (delete)
- Read checks after delete expecting `404`

## Run Commands

From project root:

```bash
make k6-crud-smoke
make k6-crud
```

## Default Profile

`make k6-crud` defaults:

- `LT_VUS=5`
- `LT_ITERATIONS=30`
- `LT_HTTP_TIMEOUT=10s`
- `LT_MAX_DURATION=2m`

`make k6-crud-smoke` defaults:

- `LT_VUS=1`
- `LT_ITERATIONS=5`
- `LT_HTTP_TIMEOUT=10s`
- `LT_MAX_DURATION=1m`

## Environment Variables

- `LT_BASE_URL` (default `http://localhost:8080`)
- `LT_API_KEY` (optional)
- `LT_VUS`
- `LT_ITERATIONS`
- `LT_MAX_DURATION`
- `LT_HTTP_TIMEOUT`
- `LT_EXPECTED_REDIRECT_STATUSES` (default `301,302`)
- `LT_EXPECTED_DELETED_STATUSES` (default `404`)

## Important Notes

- If `API_KEYS` is configured, set `LT_API_KEY` so create/delete requests are authorized.
