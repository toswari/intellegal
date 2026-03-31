import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { formatEuropeanDateTime } from "../app/datetime";
import {
  addAuditEvent,
  getStoredResults,
  listStoredGuidelineRules,
  listStoredRuns,
  setStoredResults,
  type StoredGuidelineRule,
  type StoredCheckRun,
  upsertStoredRun
} from "../app/localState";
import { apiClient, type CheckResultItem, type CheckRunResponse, type CheckType } from "../api/client";

type SelectedRun = {
  check_id: string;
  check_type: CheckType;
  requested_at: string;
  rule_name?: string;
  rule_text?: string;
};

export function GuidelinesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [rules] = useState<StoredGuidelineRule[]>(listStoredGuidelineRules());
  const [trackedRuns, setTrackedRuns] = useState(listStoredRuns());
  const [selected, setSelected] = useState<SelectedRun | null>(null);
  const [run, setRun] = useState<CheckRunResponse | null>(null);
  const [results, setResults] = useState<CheckResultItem[] | null>(null);
  const [loadingRun, setLoadingRun] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const lastLoggedStatusByRunRef = useRef<Record<string, CheckRunResponse["status"]>>({});
  const loggedResultsRunsRef = useRef(new Set<string>());

  useEffect(() => {
    const checkId = searchParams.get("checkId") ?? "";

    if (!checkId) {
      return;
    }

    const found = trackedRuns.find((item) => item.check_id === checkId);
    if (found) {
      setSelected((current) => {
        if (current?.check_id === found.check_id) {
          return current;
        }

        return {
          check_id: found.check_id,
          check_type: found.check_type,
          requested_at: found.requested_at,
          rule_name: found.rule_name,
          rule_text: found.rule_text
        };
      });
      return;
    }

    setSelected({
      check_id: checkId,
      check_type: "clause_presence",
      requested_at: new Date().toISOString()
    });
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

  const flaggedCount = useMemo(() => {
    if (!results) {
      return 0;
    }

    return results.filter((item) => item.outcome === "missing" || item.outcome === "review").length;
  }, [results]);

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Guidelines</h2>
          <p className="muted">Manage reusable rules and review their executions.</p>
        </div>
        <div className="page-actions">
          <Link to="/guidelines/run" className="button-link secondary">
            Run Guideline
          </Link>
          <Link to="/guidelines/new" className="button-link">
            New Rule
          </Link>
        </div>
      </header>

      <section className="split-grid guideline-summary-grid">
        <section className="panel">
          <h3>Rules</h3>
          {rules.length === 0 ? <p className="muted">No rules created yet.</p> : null}
          <ul className="run-list guideline-rule-list">
            {rules.map((rule) => (
              <li key={rule.id}>
                <div className="guideline-rule-item">
                  <div className="guideline-rule-copy">
                    <strong>{rule.name}</strong>
                    <p className="muted">{rule.instructions}</p>
                  </div>
                  <Link to={`/guidelines/run?ruleId=${encodeURIComponent(rule.id)}`} className="button-link secondary">
                    Run
                  </Link>
                </div>
              </li>
            ))}
          </ul>
        </section>

        <section className="panel">
          <h3>Executions</h3>
          <div className="split-grid guideline-execution-grid">
            <section className="guideline-run-list-panel">
              {trackedRuns.length === 0 ? <p className="muted">No executions yet.</p> : null}
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
                          requested_at: item.requested_at,
                          rule_name: item.rule_name,
                          rule_text: item.rule_text
                        });
                      }}
                    >
                      <span>{formatRunLabel(item)}</span>
                      <small>{item.status}</small>
                    </button>
                  </li>
                ))}
              </ul>
            </section>

            <section className="guideline-run-detail-panel">
              {selected === null ? <p className="muted">Select an execution to inspect details.</p> : null}
              {selected !== null && loadingRun ? <p className="muted">Loading run data...</p> : null}
              {error ? <p className="error-text">{error}</p> : null}
              {run ? (
                <div className="detail-stack guideline-detail-card">
                  <p>
                    <strong>Rule:</strong> {selected?.rule_name ?? "Tracked guideline"}
                  </p>
                  <p>
                    <strong>Status:</strong> {run.status}
                  </p>
                  <p>
                    <strong>Requested:</strong> {formatEuropeanDateTime(run.requested_at)}
                  </p>
                  {selected?.rule_text ? (
                    <p>
                      <strong>Instructions:</strong> {selected.rule_text}
                    </p>
                  ) : null}
                  <p>
                    <strong>Execution ID:</strong> <code>{run.check_id}</code>
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
                  <p className="muted">Flagged items: {flaggedCount}</p>
                  <div className="table-wrap guideline-results-table">
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
    </section>
  );
}

function formatRunLabel(item: StoredCheckRun) {
  return item.rule_name?.trim() || "Guideline run";
}
