CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS fuzzystrmatch;

CREATE TABLE IF NOT EXISTS indexed_document_chunks (
    document_id TEXT NOT NULL,
    checksum TEXT NOT NULL,
    chunk_id INTEGER NOT NULL,
    page_number INTEGER NOT NULL DEFAULT 1,
    snippet_text TEXT NOT NULL,
    search_vector TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple', COALESCE(snippet_text, ''))) STORED,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (document_id, checksum, chunk_id)
);

CREATE INDEX IF NOT EXISTS idx_indexed_document_chunks_document_id
    ON indexed_document_chunks (document_id);

CREATE INDEX IF NOT EXISTS idx_indexed_document_chunks_search_vector
    ON indexed_document_chunks USING GIN (search_vector);

CREATE INDEX IF NOT EXISTS idx_indexed_document_chunks_snippet_trgm
    ON indexed_document_chunks USING GIN (snippet_text gin_trgm_ops);
