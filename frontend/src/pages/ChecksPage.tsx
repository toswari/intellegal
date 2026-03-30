import { type FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { apiClient, type CheckType, type DocumentResponse } from "../api/client";
import { addAuditEvent, upsertStoredRun } from "../app/localState";

type Scope = "all" | "selected";

export function ChecksPage() {
  const navigate = useNavigate();

  const [documents, setDocuments] = useState<DocumentResponse[]>([]);
  const [scope, setScope] = useState<Scope>("all");
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [checkType, setCheckType] = useState<CheckType>("clause_presence");
  const [requiredClauseText, setRequiredClauseText] = useState("Termination for convenience");
  const [contextHint, setContextHint] = useState("");
  const [oldCompanyName, setOldCompanyName] = useState("Old Company GmbH");
  const [newCompanyName, setNewCompanyName] = useState("New Company GmbH");
  const [submitting, setSubmitting] = useState(false);
  const [loadingDocs, setLoadingDocs] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (scope !== "selected" || documents.length > 0) {
      setLoadingDocs(false);
      return;
    }

    let cancelled = false;

    const loadDocuments = async () => {
      setLoadingDocs(true);
      try {
        const response = await apiClient.listDocuments({ limit: 200, offset: 0 });
        if (!cancelled) {
          setDocuments(response.items);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load documents.");
        }
      } finally {
        if (!cancelled) {
          setLoadingDocs(false);
        }
      }
    };

    void loadDocuments();

    return () => {
      cancelled = true;
    };
  }, [documents.length, scope]);

  const toggleDocument = (id: string) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]));
  };

  const startCheck = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (scope === "selected" && selectedIds.length === 0) {
      setError("Select at least one document when using selected scope.");
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      const documentIds = scope === "selected" ? selectedIds : undefined;
      const idempotencyKey = globalThis.crypto?.randomUUID?.() ?? `check-${Date.now()}`;

      const response =
        checkType === "clause_presence"
          ? await apiClient.startClausePresenceCheck(
              {
                document_ids: documentIds,
                required_clause_text: requiredClauseText,
                context_hint: contextHint.trim() || undefined
              },
              { idempotencyKey }
            )
          : await apiClient.startCompanyNameCheck(
              {
                document_ids: documentIds,
                old_company_name: oldCompanyName,
                new_company_name: newCompanyName.trim() || undefined
              },
              { idempotencyKey }
            );

      upsertStoredRun({
        check_id: response.check_id,
        check_type: response.check_type,
        status: response.status,
        requested_at: new Date().toISOString()
      });

      addAuditEvent({
        type: "check.started",
        message: `Started ${response.check_type} check`,
        metadata: {
          check_id: response.check_id,
          scope,
          document_count: String(documentIds?.length ?? documents.length)
        }
      });

      navigate(`/results?checkId=${encodeURIComponent(response.check_id)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start check.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section className="page">
      <h2>Checks</h2>
      <form className="panel" onSubmit={startCheck}>
        <h3>New Check Wizard</h3>

        <div className="wizard-steps">
          <div className="step">
            <strong>Step 1</strong>
            <label>
              Scope
              <select value={scope} onChange={(event) => setScope(event.target.value as Scope)}>
                <option value="all">All contracts</option>
                <option value="selected">Selected contracts</option>
              </select>
            </label>
            {scope === "selected" ? (
              <div className="checkbox-list">
                {loadingDocs ? <p className="muted">Loading documents...</p> : null}
                {documents.map((document) => (
                  <label key={document.id}>
                    <input
                      type="checkbox"
                      checked={selectedIds.includes(document.id)}
                      onChange={() => toggleDocument(document.id)}
                    />
                    {document.filename} <code>{document.id.slice(0, 8)}</code>
                  </label>
                ))}
              </div>
            ) : null}
          </div>

          <div className="step">
            <strong>Step 2</strong>
            <label>
              Check Type
              <select value={checkType} onChange={(event) => setCheckType(event.target.value as CheckType)}>
                <option value="clause_presence">Missing Clause</option>
                <option value="company_name">Company Name</option>
              </select>
            </label>
          </div>

          <div className="step">
            <strong>Step 3</strong>
            {checkType === "clause_presence" ? (
              <div className="form-grid">
                <label>
                  Required Clause Text
                  <textarea
                    value={requiredClauseText}
                    onChange={(event) => setRequiredClauseText(event.target.value)}
                    rows={4}
                    required
                  />
                </label>
                <label>
                  Context Hint
                  <input value={contextHint} onChange={(event) => setContextHint(event.target.value)} />
                </label>
              </div>
            ) : (
              <div className="form-grid">
                <label>
                  Old Company Name
                  <input value={oldCompanyName} onChange={(event) => setOldCompanyName(event.target.value)} required />
                </label>
                <label>
                  New Company Name
                  <input value={newCompanyName} onChange={(event) => setNewCompanyName(event.target.value)} />
                </label>
              </div>
            )}
          </div>
        </div>

        {error ? <p className="error-text">{error}</p> : null}
        <button type="submit" disabled={submitting}>{submitting ? "Starting..." : "Start Check"}</button>
      </form>
    </section>
  );
}
