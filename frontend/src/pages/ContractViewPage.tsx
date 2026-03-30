import { useEffect, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { apiClient, type DocumentResponse, type DocumentTextResponse } from "../api/client";
import { formatEuropeanDateTime } from "../app/datetime";

export function ContractViewPage() {
  const { documentId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [document, setDocument] = useState<DocumentResponse | null>(null);
  const [documentText, setDocumentText] = useState<DocumentTextResponse | null>(null);

  useEffect(() => {
    let active = true;
    const load = async () => {
      if (!documentId) {
        setError("Document ID is missing.");
        setLoading(false);
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const [doc, text] = await Promise.all([
          apiClient.getDocument(documentId),
          apiClient.getDocumentText(documentId)
        ]);
        if (!active) {
          return;
        }
        setDocument(doc);
        setDocumentText(text);
      } catch (err) {
        if (!active) {
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load contract.");
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };

    void load();
    return () => {
      active = false;
    };
  }, [documentId]);

  const matchedSnippet = searchParams.get("snippet")?.trim() ?? "";
  const matchedPage = searchParams.get("page")?.trim() ?? "";
  const matchedScore = searchParams.get("score")?.trim() ?? "";
  const contractId = searchParams.get("contractId")?.trim() ?? "";

  return (
    <section className="page">
      <header className="page-header">
        <h2>Contract View</h2>
        <div className="page-actions">
          <Link to="/contracts" className="button-link secondary">
            Back to Contracts
          </Link>
          {contractId ? (
            <Link to={`/contracts/${encodeURIComponent(contractId)}/edit`} className="button-link secondary">
              Open Contract Files
            </Link>
          ) : null}
        </div>
      </header>

      {loading ? <p className="muted">Loading contract...</p> : null}
      {error ? <p className="error-text">{error}</p> : null}

      {!loading && !error && document ? (
        <section className="contract-view">
          <div className="contract-view-meta">
            <p>
              <strong>Filename:</strong> {document.filename}
            </p>
            <p>
              <strong>Status:</strong> {document.status}
            </p>
            <p>
              <strong>Created:</strong> {formatEuropeanDateTime(document.created_at)}
            </p>
            <p>
              <strong>Document ID:</strong> <code>{document.id}</code>
            </p>
          </div>

          {matchedSnippet ? (
            <div className="search-highlight">
              <h3>Matched Section</h3>
              <p>{matchedSnippet}</p>
              <p className="muted">
                Page: {matchedPage || "-"} {matchedScore ? `• Score: ${matchedScore}` : ""}
              </p>
            </div>
          ) : null}

          {!documentText?.has_text ? (
            <p className="muted">No extracted text is available for this contract yet.</p>
          ) : (
            <pre className="contract-text">{documentText.text}</pre>
          )}
        </section>
      ) : null}
    </section>
  );
}
