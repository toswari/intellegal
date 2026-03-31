import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { formatEuropeanDateTime } from "../app/datetime";
import {
  addAuditEvent,
  deleteStoredResultsMany,
  deleteStoredGuidelineRule,
  deleteStoredRuns,
  getStoredResults,
  listStoredGuidelineRules,
  listStoredRuns,
  setStoredResults,
  type StoredGuidelineRule,
  type StoredCheckRun,
  upsertStoredRun
} from "../app/localState";
import { ApiError, apiClient, type CheckResultItem, type CheckRunResponse, type CheckType } from "../api/client";
import { describeGuidelineRule, getGuidelineRuleTypeDisplay, normalizeGuidelineRule } from "../app/guidelineRules";
import { formatGuidelineRuleType, formatGuidelineRunStatusEmoji } from "../app/guidelineRunFlow";

type SelectedRun = {
  check_id: string;
  check_type: CheckType;
  execution_mode?: "remote" | "local";
  requested_at: string;
  rule_name?: string;
  rule_type?: StoredCheckRun["rule_type"];
  rule_text?: string;
};

export function GuidelinesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [rules, setRules] = useState<StoredGuidelineRule[]>(listStoredGuidelineRules());
  const [trackedRuns, setTrackedRuns] = useState(listStoredRuns());
  const [selectedRunIds, setSelectedRunIds] = useState<string[]>([]);
  const [selected, setSelected] = useState<SelectedRun | null>(null);
  const [run, setRun] = useState<CheckRunResponse | null>(null);
  const [results, setResults] = useState<CheckResultItem[] | null>(null);
  const [contractNamesById, setContractNamesById] = useState<Record<string, string>>({});
  const [contractIdsByDocumentId, setContractIdsByDocumentId] = useState<Record<string, string>>({});
  const [loadingRun, setLoadingRun] = useState(false);
  const [deletingRuns, setDeletingRuns] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [runActionError, setRunActionError] = useState<string | null>(null);
  const [runActionMessage, setRunActionMessage] = useState<string | null>(null);
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
          execution_mode: found.execution_mode,
          requested_at: found.requested_at,
          rule_name: found.rule_name,
          rule_type: found.rule_type,
          rule_text: found.rule_text
        };
      });
      return;
    }

    setSelected({
      check_id: checkId,
      check_type: "clause_presence",
      execution_mode: "remote",
      requested_at: new Date().toISOString()
    });
  }, [searchParams, trackedRuns]);

  useEffect(() => {
    let cancelled = false;

    const loadLookups = async () => {
      try {
        const [contractsResponse, documentsResponse] = await Promise.all([
          apiClient.listContracts({ limit: 200, offset: 0 }),
          apiClient.listDocuments({ limit: 200, offset: 0 })
        ]);

        if (cancelled) {
          return;
        }

        setContractNamesById(
          Object.fromEntries(contractsResponse.items.map((contract) => [contract.id, contract.name]))
        );
        setContractIdsByDocumentId(
          Object.fromEntries(
            documentsResponse.items
              .filter((document) => Boolean(document.contract_id))
              .map((document) => [document.id, document.contract_id as string])
          )
        );
      } catch {
        if (!cancelled) {
          setContractNamesById({});
          setContractIdsByDocumentId({});
        }
      }
    };

    void loadLookups();

    return () => {
      cancelled = true;
    };
  }, []);

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
        if (selected.execution_mode === "local") {
          const cached = getStoredResults(selected.check_id);
          setRun({
            check_id: selected.check_id,
            status: cached?.status ?? "completed",
            check_type: selected.check_type,
            requested_at: selected.requested_at,
            finished_at: selected.requested_at
          });
          setResults(cached?.items ?? []);
          setLoadingRun(false);
          return;
        }

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

  const resultRows = useMemo(
    () =>
      (results ?? []).map((item) => {
        const contractId = contractIdsByDocumentId[item.document_id];
        const contractName = contractId ? contractNamesById[contractId] : undefined;

        return {
          ...item,
          contractId,
          contractName
        };
      }),
    [contractIdsByDocumentId, contractNamesById, results]
  );

  const allRunsSelected = trackedRuns.length > 0 && selectedRunIds.length === trackedRuns.length;

  const selectedRuns = useMemo(
    () => trackedRuns.filter((item) => selectedRunIds.includes(item.check_id)),
    [selectedRunIds, trackedRuns]
  );

  const handleDeleteRule = (rule: StoredGuidelineRule) => {
    if (!window.confirm(`Delete guideline rule "${rule.name}"?`)) {
      return;
    }

    deleteStoredGuidelineRule(rule.id);
    setRules(listStoredGuidelineRules());
  };

  const removeRunsFromStorage = (checkIds: string[]) => {
    deleteStoredRuns(checkIds);
    deleteStoredResultsMany(checkIds);
    setTrackedRuns(listStoredRuns());
    setSelectedRunIds((current) => current.filter((checkId) => !checkIds.includes(checkId)));

    if (selected && checkIds.includes(selected.check_id)) {
      setSelected(null);
      setRun(null);
      setResults(null);
      setError(null);
      const nextParams = new URLSearchParams(searchParams);
      nextParams.delete("checkId");
      setSearchParams(nextParams);
    }
  };

  const deleteGuidelineRuns = async (runs: StoredCheckRun[]) => {
    if (runs.length === 0 || deletingRuns) {
      return;
    }

    const remoteRuns = runs.filter((item) => item.execution_mode !== "local");
    const confirmMessage =
      runs.length === 1
        ? `Delete "${runs[0].rule_name ?? "this guideline check"}"?`
        : `Delete ${runs.length} selected guideline checks?`;

    if (!window.confirm(`${confirmMessage}\n\nThis cannot be undone.`)) {
      return;
    }

    setDeletingRuns(true);
    setRunActionError(null);
    setRunActionMessage(null);

    try {
      if (remoteRuns.length === 1) {
        await apiClient.deleteCheckRun(remoteRuns[0].check_id);
      } else if (remoteRuns.length > 1) {
        await apiClient.deleteCheckRuns({
          check_ids: remoteRuns.map((item) => item.check_id)
        });
      }

      removeRunsFromStorage(runs.map((item) => item.check_id));
      setRunActionMessage(runs.length === 1 ? "Guideline check removed." : `${runs.length} guideline checks removed.`);
    } catch (err) {
      if (isMissingCheckError(err)) {
        removeRunsFromStorage(runs.map((item) => item.check_id));
        setRunActionMessage(runs.length === 1 ? "Guideline check removed." : `${runs.length} guideline checks removed.`);
        return;
      }
      setRunActionError(err instanceof Error ? err.message : "Failed to delete guideline checks.");
    } finally {
      setDeletingRuns(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Guidelines</h2>
          <p className="muted">Manage reusable rules separately from running and reviewing guideline checks.</p>
        </div>
      </header>

      <div className="guideline-sections">
        <section className="panel">
          <header className="guideline-section-header">
            <div>
              <h3>Rules</h3>
              <p className="muted">Create and maintain your reusable guideline rules here.</p>
            </div>
            <Link to="/guidelines/new" className="button-link">
              New Rule
            </Link>
          </header>
          {rules.length === 0 ? <p className="muted">No rules created yet.</p> : null}
          <ul className="run-list guideline-rule-list">
            {rules.map((rule) => {
              const normalizedRule = normalizeGuidelineRule(rule);
              const typeDisplay = getGuidelineRuleTypeDisplay(normalizedRule.rule_type);

              return (
                <li key={rule.id}>
                  <div className="guideline-rule-item">
                    <div className={`guideline-rule-icon guideline-rule-icon-${typeDisplay.tone}`} aria-hidden="true">
                      <span>{typeDisplay.icon}</span>
                    </div>
                    <div className="guideline-rule-copy">
                      <div className="guideline-rule-heading">
                        <strong>{rule.name}</strong>
                        <span className={`guideline-rule-type-badge guideline-rule-type-badge-${typeDisplay.tone}`}>
                          {typeDisplay.label}
                        </span>
                      </div>
                      <p className="muted">{describeGuidelineRule(rule)}</p>
                      {rule.auto_run_on_new_contract ? (
                        <p className="muted">Runs automatically for new contracts.</p>
                      ) : null}
                    </div>
                    <div className="guideline-rule-actions">
                      <Link to={`/guidelines/run?ruleId=${encodeURIComponent(rule.id)}`} className="button-link secondary">
                        Run
                      </Link>
                      <button type="button" className="danger" onClick={() => handleDeleteRule(rule)}>
                        Delete
                      </button>
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        </section>

        <section className="panel">
          <header className="guideline-section-header">
            <div>
              <h3>Guideline Checks</h3>
              <p className="muted">Run rules against contracts and inspect the outcome details separately from rule setup.</p>
            </div>
            <div className="guideline-rule-actions">
              {trackedRuns.length > 0 ? (
                <button
                  type="button"
                  className="secondary"
                  onClick={() => {
                    setSelectedRunIds(allRunsSelected ? [] : trackedRuns.map((item) => item.check_id));
                  }}
                  disabled={deletingRuns}
                >
                  {allRunsSelected ? "Clear Selection" : "Select All"}
                </button>
              ) : null}
              <button
                type="button"
                className="secondary"
                onClick={() => void deleteGuidelineRuns(selectedRuns)}
                disabled={deletingRuns || selectedRuns.length === 0}
              >
                {deletingRuns ? "Deleting..." : `Delete Selected (${selectedRuns.length})`}
              </button>
              <Link to="/guidelines/run" className="button-link secondary">
                Run Guideline
              </Link>
            </div>
          </header>
          <div className="split-grid guideline-execution-grid">
            <section className="guideline-run-list-panel">
              {trackedRuns.length === 0 ? <p className="muted">No executions yet.</p> : null}
              <ul className="run-list">
                {trackedRuns.map((item) => {
                  const isSelected = selectedRunIds.includes(item.check_id);
                  const contractPreview = getRunContractPreview(item, contractIdsByDocumentId, contractNamesById);

                  return (
                    <li key={item.check_id}>
                      <div className="run-row">
                        <label className="run-select">
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={(event) => {
                              setSelectedRunIds((current) =>
                                event.target.checked
                                  ? [...current, item.check_id]
                                  : current.filter((checkId) => checkId !== item.check_id)
                              );
                            }}
                            aria-label={`Select ${formatRunLabel(item)}`}
                            disabled={deletingRuns}
                          />
                        </label>
                        <button
                          type="button"
                          className={selected?.check_id === item.check_id ? "run-item active" : "run-item"}
                          onClick={() => {
                            setSearchParams({ checkId: item.check_id });
                            setSelected({
                              check_id: item.check_id,
                              check_type: item.check_type,
                              execution_mode: item.execution_mode,
                              requested_at: item.requested_at,
                              rule_name: item.rule_name,
                              rule_type: item.rule_type,
                              rule_text: item.rule_text
                            });
                          }}
                        >
                          <span className="guideline-run-item-copy">
                            <span className="guideline-run-item-title">
                              <span className="guideline-run-status-emoji" aria-hidden="true">
                                {formatGuidelineRunStatusEmoji(item.status)}
                              </span>
                              <span>{formatRunLabel(item)}</span>
                            </span>
                            {contractPreview ? <small>{contractPreview}</small> : null}
                            <small>Created {formatEuropeanDateTime(item.requested_at)}</small>
                          </span>
                        </button>
                        <button
                          type="button"
                          className="danger icon-button"
                          onClick={() => void deleteGuidelineRuns([item])}
                          disabled={deletingRuns}
                          aria-label={`Delete ${formatRunLabel(item)}`}
                          title={`Delete ${formatRunLabel(item)}`}
                        >
                          <svg aria-hidden="true" viewBox="0 0 24 24" width="16" height="16" fill="none">
                            <path
                              d="M4 7h16"
                              stroke="currentColor"
                              strokeWidth="1.8"
                              strokeLinecap="round"
                            />
                            <path
                              d="M9 4h6"
                              stroke="currentColor"
                              strokeWidth="1.8"
                              strokeLinecap="round"
                            />
                            <path
                              d="M7 7l1 11a2 2 0 0 0 2 1.8h4a2 2 0 0 0 2-1.8L17 7"
                              stroke="currentColor"
                              strokeWidth="1.8"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                            />
                            <path
                              d="M10 11v5M14 11v5"
                              stroke="currentColor"
                              strokeWidth="1.8"
                              strokeLinecap="round"
                            />
                          </svg>
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
              {runActionMessage ? <p className="muted">{runActionMessage}</p> : null}
              {runActionError ? <p className="error-text">{runActionError}</p> : null}
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
                  {selected?.rule_type ? (
                    <p>
                      <strong>Type:</strong> {formatGuidelineRuleType(selected.rule_type)}
                    </p>
                  ) : null}
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
                          <th>Contract</th>
                          <th>Summary</th>
                        </tr>
                      </thead>
                      <tbody>
                        {resultRows.map((item) => (
                          <tr
                            key={`${item.document_id}-${item.outcome}`}
                            className={`guideline-result-row guideline-result-row-${item.outcome}`}
                          >
                            <td>
                              <div className="guideline-result-contract">
                                <div className="guideline-result-contract-name">
                                  {item.contractId && item.contractName ? (
                                    <Link to={`/contracts/${encodeURIComponent(item.contractId)}/edit`}>
                                      {item.contractName}
                                    </Link>
                                  ) : item.contractName ? (
                                    item.contractName
                                  ) : (
                                    <code>{item.document_id}</code>
                                  )}
                                </div>
                                <div className="guideline-result-contract-meta">
                                  <span className={`chip chip-compact ${getGuidelineOutcomeChipClassName(item.outcome)}`}>
                                    {item.outcome}
                                  </span>
                                  <span className="chip chip-compact chip-neutral">
                                    {Math.round(item.confidence * 100)}% confidence
                                  </span>
                                </div>
                              </div>
                            </td>
                            <td>
                              <div className="guideline-result-summary">{item.summary ?? "-"}</div>
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
      </div>
    </section>
  );
}

function formatRunLabel(item: StoredCheckRun) {
  return item.rule_name?.trim() || "Guideline run";
}

function getRunContractPreview(
  item: StoredCheckRun,
  contractIdsByDocumentId: Record<string, string>,
  contractNamesById: Record<string, string>
) {
  const contractNames = Array.from(
    new Set(
      (item.document_ids ?? [])
        .map((documentId) => contractIdsByDocumentId[documentId])
        .filter((contractId): contractId is string => Boolean(contractId))
        .map((contractId) => contractNamesById[contractId])
        .filter((contractName): contractName is string => Boolean(contractName?.trim()))
    )
  );

  if (contractNames.length === 0) {
    return "";
  }

  const [firstContractName, ...otherContractNames] = contractNames;
  const preview = truncateTextStart(firstContractName, 42);

  if (otherContractNames.length === 0) {
    return `Contract: ${preview}`;
  }

  return `Contracts: ${preview} +${otherContractNames.length} more`;
}

function truncateTextStart(value: string, maxLength: number) {
  const trimmed = value.trim();
  if (trimmed.length <= maxLength) {
    return trimmed;
  }

  return `${trimmed.slice(0, Math.max(0, maxLength - 1)).trimEnd()}...`;
}

function isMissingCheckError(err: unknown) {
  return err instanceof ApiError && err.status === 404 && err.code === "not_found";
}

function getGuidelineOutcomeChipClassName(outcome: CheckResultItem["outcome"]) {
  if (outcome === "match") {
    return "chip-success";
  }

  if (outcome === "review") {
    return "chip-warning";
  }

  return "chip-danger";
}
