import { type FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { formatEuropeanDateTime } from "../app/datetime";
import { addAuditEvent, getStoredResults, listStoredRuns, setStoredResults, upsertStoredRun } from "../app/localState";
import { apiClient, type CheckResultItem, type CheckRunResponse, type CheckType, type DocumentResponse } from "../api/client";

type Scope = "all" | "selected";

type SelectedRun = {
  check_id: string;
  check_type: CheckType;
  requested_at: string;
};

export function ChecksPage() {
  const [searchParams, setSearchParams] = useSearchParams();

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

  const [trackedRuns, setTrackedRuns] = useState(listStoredRuns());
  const [manualCheckId, setManualCheckId] = useState(searchParams.get("checkId") ?? "");
  const [selected, setSelected] = useState<SelectedRun | null>(null);
  const [run, setRun] = useState<CheckRunResponse | null>(null);
  const [results, setResults] = useState<CheckResultItem[] | null>(null);
  const [loadingRun, setLoadingRun] = useState(false);
  const lastLoggedStatusByRunRef = useRef<Record<string, CheckRunResponse["status"]>>({});
  const loggedResultsRunsRef = useRef(new Set<string>());

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

  useEffect(() => {
    const checkId = searchParams.get("checkId") ?? "";
    setManualCheckId(checkId);

    if (!checkId) {
      return;
    }

    const found = trackedRuns.find((item) => item.check_id === checkId);
    if (found) {
      setSelected((current) => {
        if (current?.check_id === found.check_id) {
          return current;
        }

        return { check_id: found.check_id, check_type: found.check_type, requested_at: found.requested_at };
      });
    }
  }, [searchParams, trackedRuns]);

  useEffect(() => {
    if (!selected) {
      setRun(null);
      setResults(null);
      return;
    }

    let cancelled = false;

    const load = async () => {
      setLoadingRun(true);
      setError(null);
      try {
        const runResponse = await apiClient.getCheckRun(selected.check_id);
        if (cancelled) {
          return;
        }

        setRun(runResponse);
        upsertStoredRun(runResponse);

        if (lastLoggedStatusByRunRef.current[runResponse.check_id] !== runResponse.status) {
          addAuditEvent({
            type: "check.updated",
            message: `Fetched guideline status (${runResponse.status})`,
            metadata: { check_id: runResponse.check_id }
          });
          lastLoggedStatusByRunRef.current[runResponse.check_id] = runResponse.status;
        }

        if (runResponse.status === "completed") {
          const response = await apiClient.getCheckResults(selected.check_id);
          if (cancelled) {
            return;
          }

          setResults(response.items);
          setStoredResults({
            check_id: response.check_id,
            status: response.status,
            items: response.items,
            updated_at: new Date().toISOString()
          });

          if (!loggedResultsRunsRef.current.has(response.check_id)) {
            addAuditEvent({
              type: "results.loaded",
              message: `Loaded results for ${response.check_id}`,
              metadata: { item_count: String(response.items.length) }
            });
            loggedResultsRunsRef.current.add(response.check_id);
          }
        } else {
          loggedResultsRunsRef.current.delete(runResponse.check_id);
          const cached = getStoredResults(selected.check_id);
          setResults(cached?.items ?? null);
        }

        setTrackedRuns(listStoredRuns());
      } catch (err) {
        if (cancelled) {
          return;
        }

        const cached = getStoredResults(selected.check_id);
        if (cached) {
          setResults(cached.items);
        }

        setError(err instanceof Error ? err.message : "Failed to load run details.");
      } finally {
        if (!cancelled) {
          setLoadingRun(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, [selected]);

  const toggleDocument = (id: string) => {
    setSelectedIds((prev) => (prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]));
  };

  const guidelineTypeCopy =
    checkType === "clause_presence"
      ? {
          title: "Missing Clause",
          description:
            "Use this when you want to confirm that every selected contract contains a required clause or wording.",
          execution:
            "The system compares each contract's extracted text against the clause text and optional context hint, then flags contracts where the clause appears missing or needs review."
        }
      : {
          title: "Company Name",
          description:
            "Use this when contracts should reference a new legal entity name instead of an old one.",
          execution:
            "The system scans the selected contracts for the old company name, checks whether the updated name is present, and highlights contracts that may still need remediation."
        };

  const flaggedCount = useMemo(() => {
    if (!results) {
      return 0;
    }

    return results.filter((item) => item.outcome === "missing" || item.outcome === "review").length;
  }, [results]);

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
        message: `Started ${response.check_type} guideline`,
        metadata: {
          check_id: response.check_id,
          scope,
          document_count: String(documentIds?.length ?? documents.length)
        }
      });

      const nextSelected = {
        check_id: response.check_id,
        check_type: response.check_type,
        requested_at: new Date().toISOString()
      };

      setTrackedRuns(listStoredRuns());
      setSearchParams({ checkId: response.check_id });
      setManualCheckId(response.check_id);
      setSelected(nextSelected);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start guideline.");
    } finally {
      setSubmitting(false);
    }
  };

  const trackRun = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const checkId = manualCheckId.trim();
    if (!checkId) {
      return;
    }

    const existing = trackedRuns.find((item) => item.check_id === checkId);
    if (!existing) {
      upsertStoredRun({
        check_id: checkId,
        check_type: "clause_presence",
        status: "queued",
        requested_at: new Date().toISOString()
      });

      addAuditEvent({
        type: "run.tracked",
        message: `Tracking run ${checkId}`,
        metadata: { check_id: checkId }
      });
      setTrackedRuns(listStoredRuns());
    }

    setSearchParams({ checkId });
    setSelected(
      existing
        ? {
            check_id: existing.check_id,
            check_type: existing.check_type,
            requested_at: existing.requested_at
          }
        : {
            check_id: checkId,
            check_type: "clause_presence",
            requested_at: new Date().toISOString()
          }
    );
  };

  return (
    <section className="page">
      <h2>Guidelines</h2>
      <section className="panel guideline-info-panel">
        <h3>How Guideline Runs Work</h3>
        <p className="muted">
          A guideline run is a repeatable review across one or more contracts. You define the rule, the system checks the
          selected contract text, and this same workspace shows which contracts passed, failed, or need manual review.
        </p>

        <div className="guideline-flow">
          <article className="guideline-flow-step">
            <strong>1. Select scope</strong>
            <p>Run the guideline against all contracts or focus on a hand-picked set of documents.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>2. Define the rule</strong>
            <p>Choose the guideline type and provide the exact clause text or company names to validate.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>3. Execute the run</strong>
            <p>The run is queued, processed in the background, and added to the history list below.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>4. Review outcomes</strong>
            <p>Select any run to inspect status, flagged contracts, confidence scores, and evidence snippets.</p>
          </article>
        </div>

        <div className="guideline-type-explainer">
          <strong>Current guideline: {guidelineTypeCopy.title}</strong>
          <p>{guidelineTypeCopy.description}</p>
          <p className="muted">{guidelineTypeCopy.execution}</p>
        </div>
      </section>

      <form className="panel" onSubmit={startCheck}>
        <h3>New Guideline Run</h3>

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
              Guideline Type
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
        <button type="submit" disabled={submitting}>{submitting ? "Starting..." : "Start Guideline"}</button>
      </form>

      <section className="panel">
        <h3>Past Guideline Runs</h3>
        <p className="muted">
          Create a new guideline execution above, or reopen any previous run here to inspect the latest results.
        </p>

        <form className="inline-form guideline-track-form" onSubmit={trackRun}>
          <label>
            Track Guideline ID
            <input
              value={manualCheckId}
              onChange={(event) => setManualCheckId(event.target.value)}
              placeholder="Paste guideline UUID"
            />
          </label>
          <button type="submit">Track</button>
        </form>

        <div className="split-grid">
          <section className="panel guideline-run-list-panel">
            <h4>Run List</h4>
            {trackedRuns.length === 0 ? <p className="muted">No runs tracked yet.</p> : null}
            <ul className="run-list">
              {trackedRuns.map((item) => (
                <li key={item.check_id}>
                  <button
                    type="button"
                    className={selected?.check_id === item.check_id ? "run-item active" : "run-item"}
                    onClick={() => {
                      setSearchParams({ checkId: item.check_id });
                      setSelected({
                        check_id: item.check_id,
                        check_type: item.check_type,
                        requested_at: item.requested_at
                      });
                    }}
                  >
                    <span>{item.check_type}</span>
                    <code>{item.check_id.slice(0, 8)}</code>
                    <small>{item.status}</small>
                  </button>
                </li>
              ))}
            </ul>
          </section>

          <section className="panel guideline-run-detail-panel">
            <h4>Run Details</h4>
            {selected === null ? <p className="muted">Select a run to inspect details.</p> : null}
            {selected !== null && loadingRun ? <p className="muted">Loading run data...</p> : null}
            {run ? (
              <div className="detail-stack">
                <p>
                  <strong>Guideline ID:</strong> <code>{run.check_id}</code>
                </p>
                <p>
                  <strong>Status:</strong> {run.status}
                </p>
                <p>
                  <strong>Guideline Type:</strong> {run.check_type}
                </p>
                <p>
                  <strong>Requested:</strong> {formatEuropeanDateTime(run.requested_at)}
                </p>
                {run.finished_at ? (
                  <p>
                    <strong>Finished:</strong> {formatEuropeanDateTime(run.finished_at)}
                  </p>
                ) : null}
                {run.failure_reason ? (
                  <p>
                    <strong>Failure:</strong> {run.failure_reason}
                  </p>
                ) : null}
              </div>
            ) : null}

            {results ? (
              <>
                <h4>Result Items ({results.length})</h4>
                <p className="muted">Flagged items: {flaggedCount}</p>
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Document ID</th>
                        <th>Outcome</th>
                        <th>Confidence</th>
                        <th>Summary</th>
                      </tr>
                    </thead>
                    <tbody>
                      {results.map((item) => (
                        <tr key={`${item.document_id}-${item.outcome}`}>
                          <td>
                            <code>{item.document_id}</code>
                          </td>
                          <td>{item.outcome}</td>
                          <td>{Math.round(item.confidence * 100)}%</td>
                          <td>
                            {item.summary ?? "-"}
                            {item.evidence?.map((snippet, index) => (
                              <div key={`${snippet.page_number}-${index}`} className="evidence-block">
                                <small>Page {snippet.page_number}</small>
                                <p>{snippet.snippet_text}</p>
                              </div>
                            ))}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            ) : null}
          </section>
        </div>
      </section>
    </section>
  );
}
