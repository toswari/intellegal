import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { apiClient, type DocumentResponse, type DocumentStatus } from "../api/client";
import { formatEuropeanDateTime } from "../app/datetime";

type Filters = {
  status: "all" | DocumentStatus;
  sourceType: "all" | "repository" | "upload" | "api";
  query: string;
};

export function ContractsPage() {
  const [filters, setFilters] = useState<Filters>({ status: "all", sourceType: "all", query: "" });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [documents, setDocuments] = useState<DocumentResponse[]>([]);
  const [deletingDocumentId, setDeletingDocumentId] = useState<string | null>(null);

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

  const handleDelete = async (document: DocumentResponse) => {
    const confirmed = window.confirm(
      `Delete "${document.filename}" permanently?\n\nThis will hard-delete the contract file and related data.`
    );
    if (!confirmed) {
      return;
    }

    setError(null);
    setDeletingDocumentId(document.id);
    try {
      await apiClient.deleteDocument(document.id);
      setDocuments((prev) => prev.filter((item) => item.id !== document.id));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete document.";
      setError(message);
    } finally {
      setDeletingDocumentId(null);
    }
  };

  const statusBadgeClass = (status: DocumentStatus) => {
    if (status === "indexed") return "chip chip-success";
    if (status === "failed") return "chip chip-danger";
    if (status === "processing") return "chip chip-warning";
    return "chip chip-neutral";
  };

  const sourceBadgeClass = (sourceType?: string) => {
    if (sourceType === "upload") return "chip chip-source-upload";
    if (sourceType === "repository") return "chip chip-source-repository";
    if (sourceType === "api") return "chip chip-source-api";
    return "chip chip-neutral";
  };

  return (
    <section className="page">
      <header className="page-header">
        <h2>Contracts</h2>
        <div className="page-actions">
          <button type="button" className="secondary" onClick={() => void loadDocuments()}>
            Refresh
          </button>
          <Link to="/contracts/new" className="button-link">
            New Contract
          </Link>
        </div>
      </header>

      <section className="contracts-list">
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
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((document) => (
                  <tr key={document.id}>
                    <td>{document.filename}</td>
                    <td>
                      <span className={statusBadgeClass(document.status)}>{document.status}</span>
                    </td>
                    <td>
                      <span className={sourceBadgeClass(document.source_type)}>{document.source_type ?? "-"}</span>
                    </td>
                    <td>{formatEuropeanDateTime(document.created_at)}</td>
                    <td>
                      <code>{document.id}</code>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="danger"
                        disabled={deletingDocumentId !== null}
                        onClick={() => void handleDelete(document)}
                      >
                        {deletingDocumentId === document.id ? "Deleting..." : "Delete"}
                      </button>
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
