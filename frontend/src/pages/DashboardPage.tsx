import { useEffect, useMemo, useState } from "react";
import { apiClient, type CheckResultItem, type CheckRunResponse, type DocumentResponse } from "../api/client";
import { listStoredRuns, upsertStoredRun } from "../app/localState";

type DashboardState = {
  loading: boolean;
  error: string | null;
  documents: DocumentResponse[];
  runs: CheckRunResponse[];
  flaggedDocuments: number;
};

const initialState: DashboardState = {
  loading: true,
  error: null,
  documents: [],
  runs: [],
  flaggedDocuments: 0
};

export function DashboardPage() {
  const [state, setState] = useState<DashboardState>(initialState);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setState((prev) => ({ ...prev, loading: true, error: null }));

      try {
        const documentsResp = await apiClient.listDocuments({ limit: 200, offset: 0 });
        const storedRuns = listStoredRuns().slice(0, 8);

        const runs = await Promise.all(
          storedRuns.map(async (stored) => {
            try {
              const live = await apiClient.getCheckRun(stored.check_id);
              upsertStoredRun(live);
              return live;
            } catch {
              return {
                check_id: stored.check_id,
                check_type: stored.check_type,
                status: stored.status,
                requested_at: stored.requested_at,
                finished_at: stored.finished_at,
                failure_reason: stored.failure_reason
              } satisfies CheckRunResponse;
            }
          })
        );

        const completedRuns = runs.filter((run) => run.status === "completed");
        const flaggedResultBatches = await Promise.all(
          completedRuns.slice(0, 4).map(async (run) => {
            try {
              const result = await apiClient.getCheckResults(run.check_id);
              return result.items;
            } catch {
              return [] as CheckResultItem[];
            }
          })
        );

        const flagged = new Set<string>();
        for (const batch of flaggedResultBatches) {
          for (const item of batch) {
            if (item.outcome === "missing" || item.outcome === "review") {
              flagged.add(item.document_id);
            }
          }
        }

        if (cancelled) {
          return;
        }

        setState({
          loading: false,
          error: null,
          documents: documentsResp.items,
          runs,
          flaggedDocuments: flagged.size
        });
      } catch (error) {
        if (cancelled) {
          return;
        }

        const message = error instanceof Error ? error.message : "Failed to load dashboard data.";
        setState((prev) => ({ ...prev, loading: false, error: message }));
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, []);

  const documentStatusSummary = useMemo(() => {
    const summary: Record<string, number> = {
      ingested: 0,
      processing: 0,
      indexed: 0,
      failed: 0
    };

    for (const document of state.documents) {
      summary[document.status] = (summary[document.status] ?? 0) + 1;
    }

    return summary;
  }, [state.documents]);

  return (
    <section className="page">
      <header className="page-header">
        <h2>Dashboard</h2>
        <button type="button" className="secondary" onClick={() => window.location.reload()}>
          Refresh
        </button>
      </header>

      {state.error ? <p className="error-text">{state.error}</p> : null}

      <div className="kpi-grid" aria-busy={state.loading}>
        <article className="kpi-card">
          <h3>Contracts Ingested</h3>
          <strong>{state.documents.length}</strong>
        </article>
        <article className="kpi-card">
          <h3>Checks Run</h3>
          <strong>{state.runs.length}</strong>
        </article>
        <article className="kpi-card">
          <h3>Flagged Contracts</h3>
          <strong>{state.flaggedDocuments}</strong>
        </article>
        <article className="kpi-card">
          <h3>Failed Jobs</h3>
          <strong>{state.runs.filter((run) => run.status === "failed").length}</strong>
        </article>
      </div>

      <div className="split-grid">
        <article>
          <h3>Document Status</h3>
          <ul className="summary-list">
            <li>Ingested: {documentStatusSummary.ingested}</li>
            <li>Processing: {documentStatusSummary.processing}</li>
            <li>Indexed: {documentStatusSummary.indexed}</li>
            <li>Failed: {documentStatusSummary.failed}</li>
          </ul>
        </article>

        <article>
          <h3>Recent Runs</h3>
          {state.runs.length === 0 ? (
            <p className="muted">No check runs tracked yet. Start one from the Checks page.</p>
          ) : (
            <ul className="summary-list">
              {state.runs.slice(0, 5).map((run) => (
                <li key={run.check_id}>
                  <code>{run.check_id.slice(0, 8)}</code> {run.check_type} - {run.status}
                </li>
              ))}
            </ul>
          )}
        </article>
      </div>
    </section>
  );
}
