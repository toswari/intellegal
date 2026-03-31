DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS check_run_documents;

ALTER TABLE external_copy_events
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS updated_at;

ALTER TABLE evidence_snippets
    DROP COLUMN IF EXISTS score;

ALTER TABLE check_runs
    DROP CONSTRAINT IF EXISTS check_runs_check_type_check;

ALTER TABLE check_runs
    DROP COLUMN IF EXISTS failure_reason,
    DROP COLUMN IF EXISTS requested_at;

ALTER TABLE check_runs
    ADD CONSTRAINT check_runs_check_type_check
    CHECK (check_type IN ('missing_clause'));

ALTER TABLE documents
    DROP COLUMN IF EXISTS file_order,
    DROP COLUMN IF EXISTS storage_key,
    DROP COLUMN IF EXISTS extracted_text,
    DROP COLUMN IF EXISTS checksum,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS contract_id;

DROP TABLE IF EXISTS contracts;
