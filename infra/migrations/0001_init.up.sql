CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type TEXT NOT NULL,
    source_ref TEXT,
    filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    storage_uri TEXT NOT NULL,
    ocr_required BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL CHECK (status IN ('ingested', 'processing', 'indexed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_status ON documents (status);
CREATE INDEX IF NOT EXISTS idx_documents_created_at ON documents (created_at DESC);

CREATE TABLE IF NOT EXISTS document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_label TEXT,
    checksum TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_document_versions_document_checksum
    ON document_versions (document_id, checksum);
CREATE INDEX IF NOT EXISTS idx_document_versions_document_id
    ON document_versions (document_id);

CREATE TABLE IF NOT EXISTS check_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_type TEXT NOT NULL CHECK (check_type IN ('missing_clause', 'company_name')),
    input_payload JSONB NOT NULL,
    requested_by TEXT,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_check_runs_status ON check_runs (status);
CREATE INDEX IF NOT EXISTS idx_check_runs_created_at ON check_runs (created_at DESC);

CREATE TABLE IF NOT EXISTS check_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_run_id UUID NOT NULL REFERENCES check_runs(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    outcome TEXT NOT NULL CHECK (outcome IN ('match', 'missing', 'review')),
    confidence NUMERIC(5,4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    summary TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_check_results_check_run_id ON check_results (check_run_id);
CREATE INDEX IF NOT EXISTS idx_check_results_document_id ON check_results (document_id);

CREATE TABLE IF NOT EXISTS evidence_snippets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_result_id UUID NOT NULL REFERENCES check_results(id) ON DELETE CASCADE,
    page_number INTEGER,
    chunk_ref TEXT,
    snippet_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evidence_snippets_check_result_id ON evidence_snippets (check_result_id);

CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type TEXT NOT NULL,
    actor_id TEXT,
    event_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_entity ON audit_events (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events (created_at DESC);

CREATE TABLE IF NOT EXISTS external_copy_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    target_system TEXT NOT NULL,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_payload JSONB,
    status TEXT NOT NULL CHECK (status IN ('queued', 'succeeded', 'failed')),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_external_copy_events_document_id ON external_copy_events (document_id);
CREATE INDEX IF NOT EXISTS idx_external_copy_events_status ON external_copy_events (status);
