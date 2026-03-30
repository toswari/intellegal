import { type FormEvent, useEffect, useMemo, useState } from "react";
import { apiClient, type DocumentResponse, type DocumentStatus } from "../api/client";
import { addAuditEvent } from "../app/localState";

type Filters = {
  status: "all" | DocumentStatus;
  sourceType: "all" | "repository" | "upload" | "api";
  query: string;
};

async function toBase64(file: File): Promise<string> {
  const buffer = await file.arrayBuffer();
  let binary = "";
  const bytes = new Uint8Array(buffer);

  for (const value of bytes) {
    binary += String.fromCharCode(value);
  }

  return btoa(binary);
}

export function ContractsPage() {
  const [filters, setFilters] = useState<Filters>({ status: "all", sourceType: "all", query: "" });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [documents, setDocuments] = useState<DocumentResponse[]>([]);

  const [file, setFile] = useState<File | null>(null);
  const [sourceType, setSourceType] = useState<"repository" | "upload" | "api">("upload");
  const [sourceRef, setSourceRef] = useState("");
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);

  const loadDocuments = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await apiClient.listDocuments({
        status: filters.status === "all" ? undefined : filters.status,
        source_type: filters.sourceType === "all" ? undefined : filters.sourceType,
        limit: 200,
        offset: 0
      });
      setDocuments(response.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load documents.";
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadDocuments();
  }, [filters.status, filters.sourceType]);

  const filtered = useMemo(() => {
    const query = filters.query.trim().toLowerCase();
    if (!query) {
      return documents;
    }

    return documents.filter((document) => {
      return (
        document.filename.toLowerCase().includes(query) ||
        document.id.toLowerCase().includes(query) ||
        (document.source_ref ?? "").toLowerCase().includes(query)
      );
    });
  }, [documents, filters.query]);

  const uploadDocument = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (!file) {
      setUploadError("Select a PDF or JPEG file first.");
      return;
    }

    if (file.type !== "application/pdf" && file.type !== "image/jpeg") {
      setUploadError("Only application/pdf and image/jpeg are supported.");
      return;
    }

    setUploading(true);
    setUploadError(null);

    try {
      const contentBase64 = await toBase64(file);
      const response = await apiClient.createDocument(
        {
          filename: file.name,
          mime_type: file.type as "application/pdf" | "image/jpeg",
          source_type: sourceType,
          source_ref: sourceRef.trim() || undefined,
          content_base64: contentBase64
        },
        { idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `upload-${Date.now()}` }
      );

      addAuditEvent({
        type: "document.uploaded",
        message: `Uploaded ${response.filename}`,
        metadata: { document_id: response.id, mime_type: response.mime_type }
      });

      setFile(null);
      setSourceRef("");
      await loadDocuments();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Upload failed.";
      setUploadError(message);
    } finally {
      setUploading(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <h2>Contracts</h2>
        <button type="button" className="secondary" onClick={() => void loadDocuments()}>
          Refresh
        </button>
      </header>

      <form className="panel" onSubmit={uploadDocument}>
        <h3>Upload Contract</h3>
        <div className="form-grid">
          <label>
            File
            <input
              type="file"
              accept="application/pdf,image/jpeg"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              required
            />
          </label>
          <label>
            Source Type
            <select value={sourceType} onChange={(event) => setSourceType(event.target.value as typeof sourceType)}>
              <option value="upload">upload</option>
              <option value="repository">repository</option>
              <option value="api">api</option>
            </select>
          </label>
          <label>
            Source Ref
            <input value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} placeholder="Optional" />
          </label>
        </div>
        {uploadError ? <p className="error-text">{uploadError}</p> : null}
        <button type="submit" disabled={uploading}>{uploading ? "Uploading..." : "Upload"}</button>
      </form>

      <section className="panel">
        <h3>Contract List</h3>
        <div className="filter-row">
          <label>
            Status
            <select
              value={filters.status}
              onChange={(event) => setFilters((prev) => ({ ...prev, status: event.target.value as Filters["status"] }))}
            >
              <option value="all">all</option>
              <option value="ingested">ingested</option>
              <option value="processing">processing</option>
              <option value="indexed">indexed</option>
              <option value="failed">failed</option>
            </select>
          </label>
          <label>
            Source
            <select
              value={filters.sourceType}
              onChange={(event) =>
                setFilters((prev) => ({ ...prev, sourceType: event.target.value as Filters["sourceType"] }))
              }
            >
              <option value="all">all</option>
              <option value="upload">upload</option>
              <option value="repository">repository</option>
              <option value="api">api</option>
            </select>
          </label>
          <label>
            Search
            <input
              value={filters.query}
              onChange={(event) => setFilters((prev) => ({ ...prev, query: event.target.value }))}
              placeholder="filename or id"
            />
          </label>
        </div>

        {error ? <p className="error-text">{error}</p> : null}
        {loading ? <p className="muted">Loading documents...</p> : null}
        {!loading && filtered.length === 0 ? <p className="muted">No documents found.</p> : null}

        {filtered.length > 0 ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Filename</th>
                  <th>Status</th>
                  <th>Source</th>
                  <th>Created</th>
                  <th>Document ID</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((document) => (
                  <tr key={document.id}>
                    <td>{document.filename}</td>
                    <td>{document.status}</td>
                    <td>{document.source_type ?? "-"}</td>
                    <td>{new Date(document.created_at).toLocaleString()}</td>
                    <td>
                      <code>{document.id}</code>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </section>
    </section>
  );
}
