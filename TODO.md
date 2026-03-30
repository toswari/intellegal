# TODO - Legal Document Intelligence Platform

## 0. Project Setup
- [x] Create folder structure: `frontend/`, `go-api/`, `py-ai-api/`, `infra/`, `samples/`, `docs/`
- [x] Add root `docker-compose.yml`
- [x] Add root `.env.example`
- [x] Add root `Makefile` for common commands
- [x] Add `docs/architecture.md` and `docs/demo-script.md`

## 1. API Surface and Contracts
### 1.1 Public API (Go Main API)
- [x] Define REST namespace (`/api/v1`)
- [x] Define endpoints:
- [x] `POST /api/v1/documents`
- [x] `GET /api/v1/documents`
- [x] `GET /api/v1/documents/{id}`
- [x] `POST /api/v1/checks/clause-presence`
- [x] `POST /api/v1/checks/company-name`
- [x] `GET /api/v1/checks/{id}`
- [x] `GET /api/v1/checks/{id}/results`

### 1.2 Internal API (Python AI API)
- [x] Define internal namespace (`/internal/v1`)
- [x] Define endpoints:
- [x] `POST /internal/v1/extract`
- [x] `POST /internal/v1/index`
- [x] `POST /internal/v1/analyze/clause`
- [x] `POST /internal/v1/analyze/company-name`

### 1.3 Shared Contracts
- [x] Define JSON schemas for document, check run, result, and evidence payloads
- [x] Define error model and retry semantics between services
- [x] Add OpenAPI docs for both APIs

## 2. Go Main API
### 2.1 Bootstrap
- [x] Initialize Go project structure
- [x] Add router/middleware stack
- [x] Add config and secrets loading
- [x] Add structured logging and request IDs
- [x] Add health/readiness endpoints

### 2.2 Core Logic
- [x] Implement document upload/list/detail handlers
- [x] Implement check creation and status handlers
- [x] Orchestrate calls to Python AI API
- [x] Persist run states (`queued/running/completed/failed`)
- [x] Add validation and idempotency for check creation

### 2.3 Integrations
- [x] Implement contract copy REST API client
- [x] Add retries, timeouts, and failure tracking
- [x] Emit audit events for all critical actions

## 3. Python AI API (OCR + RAG)
### 3.1 Bootstrap
- [x] Initialize FastAPI project
- [x] Add config, logging, health endpoint
- [x] Add internal auth mechanism (service token/shared secret)

### 3.2 Extraction Pipeline
- [x] Implement PDF extraction path
- [x] Implement JPEG/scanned OCR path
- [x] Add normalization and page boundary preservation
- [x] Return extraction diagnostics and confidence metadata

### 3.3 Indexing Pipeline
- [x] Implement chunking (size/overlap)
- [x] Implement embedding generation
- [x] Upsert chunks + metadata into Qdrant
- [x] Add idempotent reindex by checksum/version

### 3.4 Analysis Pipeline
- [x] Implement clause-presence analysis
- [x] Implement company-name analysis
- [x] Enforce evidence-backed findings
- [x] Add confidence scoring + deterministic fallback

## 4. Data and Storage
### 4.1 PostgreSQL
- [x] Configure DB access for Go API and Python AI API
- [x] Add migration tooling
- [x] Create tables:
- [x] `documents`
- [x] `document_versions`
- [x] `check_runs`
- [x] `check_results`
- [x] `evidence_snippets`
- [x] `audit_events`
- [x] `external_copy_events`

### 4.2 Qdrant
- [x] Add Qdrant service to Docker Compose
- [x] Define collection and payload schema
- [x] Add client wrappers and startup checks

### 4.3 Blob/File Storage
- [x] Define storage adapter interface
- [x] Implement local filesystem adapter (MVP)
- [x] Add Azure Blob adapter placeholder

## 5. Frontend (React + TypeScript)
### 5.1 Foundation
- [x] Initialize React app
- [x] Set up routing and app shell
- [x] Add typed API client for Go Main API
- [x] Add env-based configuration

### 5.2 Pages
- [x] Dashboard page
- [x] Contracts page (upload/list/filter)
- [x] Checks page (new check form/wizard)
- [x] Results page (run list + detail panel)
- [x] Audit log page

### 5.3 Components and UX
- [ ] `ContractTable`
- [ ] `UploadDropzone`
- [ ] `CheckBuilderForm`
- [ ] `RunStatusBadge`
- [ ] `EvidenceSnippetCard`
- [ ] `ResultDetailPanel`
- [ ] `AuditEventTable`
- [ ] Loading/error/empty states

## 6. Security and Compliance
- [ ] Secrets via env/secret manager only
- [ ] Add baseline auth/RBAC in Go API
- [ ] Restrict Python AI API to internal network/service auth
- [ ] Minimize sensitive data in logs
- [ ] Label outputs as AI-assisted + human review required

## 7. Testing and Quality Gates
- [x] Bootstrap test frameworks (Go `go test`, Python `pytest`, Frontend `vitest`)

### 7.1 Go API Tests
- [x] Handler unit tests
- [x] Service orchestration tests
- [x] Integration tests with mocked Python AI API

### 7.2 Python AI API Tests
- [x] Unit tests for extraction/chunking/analysis logic
- [ ] Integration tests for Qdrant indexing + retrieval

### 7.3 Frontend Tests
- [x] Component tests
- [x] Integration tests for check run flow

### 7.4 Test Data
- [ ] Prepare 5-10 sample contracts (PDF + JPEG)
- [ ] Define expected outputs for golden checks

## 8. Infra and Delivery
- [x] Add Dockerfile for Go API
- [x] Add Dockerfile for Python AI API
- [x] Add Dockerfile for frontend
- [x] Wire all services in `docker-compose.yml`
- [x] Add Terraform skeleton for Azure deployment
- [x] Add CI pipeline (lint, test, build)

## 9. Documentation and Demo
- [ ] Keep README aligned with implementation
- [ ] Add local quickstart instructions
- [ ] Add API examples (public and internal)
- [ ] Add known limitations + phase-2 roadmap (GraphRAG optional)
- [ ] Prepare presentation deck (15-20 min)
- [ ] Prepare live demo script + backup screenshots

## 10. Suggested Build Order
- [x] Milestone A: Local stack boots (Go API + Python AI API + React + Postgres + Qdrant)
- [x] Milestone B: Upload -> extract -> index one document end-to-end
- [x] Wire document upload flow to trigger `/internal/v1/extract` then `/internal/v1/index` (currently upload is metadata-only in Go API)
- [x] Milestone C: Missing clause check returns evidence
- [x] Milestone D: Company-name check + run history
- [x] Milestone E: External copy API integration + audit trail
- [ ] Milestone F: Test pass + demo polish
