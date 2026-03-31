CREATE TABLE IF NOT EXISTS contracts (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_ref TEXT,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS contract_id UUID REFERENCES contracts(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS extracted_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storage_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS file_order INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    ALTER TABLE check_runs DROP CONSTRAINT IF EXISTS check_runs_check_type_check;
EXCEPTION
    WHEN undefined_table THEN NULL;
END $$;

ALTER TABLE check_runs
    ADD COLUMN IF NOT EXISTS requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS failure_reason TEXT NOT NULL DEFAULT '';

UPDATE check_runs
SET requested_at = COALESCE(requested_at, created_at, NOW())
WHERE requested_at IS NULL;

ALTER TABLE check_runs
    ADD CONSTRAINT check_runs_check_type_check
    CHECK (check_type IN ('clause_presence', 'llm_review'));

CREATE TABLE IF NOT EXISTS check_run_documents (
    check_run_id UUID NOT NULL REFERENCES check_runs(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (check_run_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_check_run_documents_document_id
    ON check_run_documents (document_id);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    idempotency_key TEXT PRIMARY KEY,
    check_run_id UUID NOT NULL REFERENCES check_runs(id) ON DELETE CASCADE,
    payload_hash TEXT NOT NULL
);

ALTER TABLE evidence_snippets
    ADD COLUMN IF NOT EXISTS score DOUBLE PRECISION;

ALTER TABLE external_copy_events
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0;
