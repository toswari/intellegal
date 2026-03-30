import { type FormEvent, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { apiClient, type CheckResultItem, type CheckRunResponse, type CheckType } from "../api/client";
import { addAuditEvent, getStoredResults, listStoredRuns, setStoredResults, upsertStoredRun } from "../app/localState";

type SelectedRun = {
  check_id: string;
  check_type: CheckType;
  requested_at: string;
};

export function ResultsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [trackedRuns, setTrackedRuns] = useState(listStoredRuns());
  const [manualCheckId, setManualCheckId] = useState(searchParams.get("checkId") ?? "");
  const [selected, setSelected] = useState<SelectedRun | null>(null);
  const [run, setRun] = useState<CheckRunResponse | null>(null);
  const [results, setResults] = useState<CheckResultItem[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const checkId = searchParams.get("checkId");
    if (!checkId) {
      return;
    }

    const found = trackedRuns.find((item) => item.check_id === checkId);
    if (found) {
      setSelected({ check_id: found.check_id, check_type: found.check_type, requested_at: found.requested_at });
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
      setLoading(true);
      setError(null);
      try {
        const runResponse = await apiClient.getCheckRun(selected.check_id);
        if (cancelled) {
          return;
        }

        setRun(runResponse);
        upsertStoredRun(runResponse);

        addAuditEvent({
          type: "check.updated",
          message: `Fetched check status (${runResponse.status})`,
          metadata: { check_id: runResponse.check_id }
        });

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

          addAuditEvent({
            type: "results.loaded",
            message: `Loaded results for ${response.check_id}`,
            metadata: { item_count: String(response.items.length) }
          });
        } else {
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
          setLoading(false);
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
      <h2>Results</h2>

      <form className="panel inline-form" onSubmit={trackRun}>
        <label>
          Track Check ID
          <input
            value={manualCheckId}
            onChange={(event) => setManualCheckId(event.target.value)}
            placeholder="Paste check UUID"
          />
        </label>
        <button type="submit">Track</button>
      </form>

      <div className="split-grid">
        <section className="panel">
          <h3>Run List</h3>
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

        <section className="panel">
          <h3>Detail Panel</h3>
          {selected === null ? <p className="muted">Select a run to inspect details.</p> : null}
          {selected !== null && loading ? <p className="muted">Loading run data...</p> : null}
          {error ? <p className="error-text">{error}</p> : null}

          {run ? (
            <div className="detail-stack">
              <p>
                <strong>Check ID:</strong> <code>{run.check_id}</code>
              </p>
              <p>
                <strong>Status:</strong> {run.status}
              </p>
              <p>
                <strong>Check Type:</strong> {run.check_type}
              </p>
              <p>
                <strong>Requested:</strong> {new Date(run.requested_at).toLocaleString()}
              </p>
              {run.finished_at ? (
                <p>
                  <strong>Finished:</strong> {new Date(run.finished_at).toLocaleString()}
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
  );
}
