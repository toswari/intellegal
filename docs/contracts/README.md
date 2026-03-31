# API Surface and Contracts

This folder defines API contracts for Step 1:

- `public-api.openapi.yaml`: Go Main API (client-facing)
- `internal-api.openapi.yaml`: Python AI API (service-to-service)

## Contract Principles
- All API payloads are JSON (`application/json`).
- IDs are UUID strings.
- Timestamps use RFC3339 UTC format.
- Public write endpoints accept `Idempotency-Key` header.
- Errors are returned in a standard shape:
  - `error.code`
  - `error.message`
  - `error.retriable`
  - `error.details`

## Retry and Idempotency Semantics

### Public API (client -> Go API)
- `POST /api/v1/documents` and `POST /api/v1/guidelines/*` support `Idempotency-Key`.
- If the same `Idempotency-Key` and same payload are replayed, API returns the original response.
- Retry guidance:
  - Retry on `429`, `502`, `503`, `504` with exponential backoff + jitter.
  - Do not auto-retry on `400`, `401`, `403`, `404`, `409`, `422`.

### Internal API (Go API -> Python AI API)
- Internal calls include `Authorization: Bearer <INTERNAL_SERVICE_TOKEN>` and `X-Request-ID`.
- Internal request body includes `job_id` for idempotent processing.
- Retry guidance:
  - Retry on network errors and `5xx` except repeated deterministic failures.
  - Recommended backoff: 200ms, 500ms, 1s, 2s (max 4 attempts).
- Python API returns `202` for accepted async jobs.

## Versioning
- Public API path version: `/api/v1`.
- Internal API path version: `/internal/v1`.
- Breaking changes require a new path version.
