import type { CheckResultItem, CheckRunResponse, CheckType } from "../api/client";

const CHECK_RUNS_KEY = "ldi.checkRuns";
const AUDIT_EVENTS_KEY = "ldi.auditEvents";
const RUN_RESULTS_KEY = "ldi.runResults";

type RunStatus = CheckRunResponse["status"];

export type StoredCheckRun = {
  check_id: string;
  check_type: CheckType;
  status: RunStatus;
  requested_at: string;
  finished_at?: string;
  failure_reason?: string;
};

export type StoredRunResults = {
  check_id: string;
  status: RunStatus;
  items: CheckResultItem[];
  updated_at: string;
};

export type AuditEvent = {
  id: string;
  timestamp: string;
  type: "document.uploaded" | "check.started" | "check.updated" | "results.loaded" | "run.tracked";
  message: string;
  metadata?: Record<string, string>;
};

function readJson<T>(key: string, fallback: T): T {
  if (typeof window === "undefined") {
    return fallback;
  }

  const value = window.localStorage.getItem(key);
  if (!value) {
    return fallback;
  }

  try {
    return JSON.parse(value) as T;
  } catch {
    return fallback;
  }
}

function writeJson<T>(key: string, value: T) {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.setItem(key, JSON.stringify(value));
}

export function listStoredRuns(): StoredCheckRun[] {
  const runs = readJson<StoredCheckRun[]>(CHECK_RUNS_KEY, []);
  return [...runs].sort((a, b) => b.requested_at.localeCompare(a.requested_at));
}

export function upsertStoredRun(run: StoredCheckRun) {
  const runs = readJson<StoredCheckRun[]>(CHECK_RUNS_KEY, []);
  const next = runs.filter((item) => item.check_id !== run.check_id);
  next.push(run);
  writeJson(CHECK_RUNS_KEY, next);
}

export function getStoredResults(checkId: string): StoredRunResults | null {
  const resultsMap = readJson<Record<string, StoredRunResults>>(RUN_RESULTS_KEY, {});
  return resultsMap[checkId] ?? null;
}

export function setStoredResults(value: StoredRunResults) {
  const resultsMap = readJson<Record<string, StoredRunResults>>(RUN_RESULTS_KEY, {});
  resultsMap[value.check_id] = value;
  writeJson(RUN_RESULTS_KEY, resultsMap);
}

export function listAuditEvents(): AuditEvent[] {
  const events = readJson<AuditEvent[]>(AUDIT_EVENTS_KEY, []);
  return [...events].sort((a, b) => b.timestamp.localeCompare(a.timestamp));
}

export function addAuditEvent(event: Omit<AuditEvent, "id" | "timestamp">) {
  const events = readJson<AuditEvent[]>(AUDIT_EVENTS_KEY, []);
  const next: AuditEvent = {
    ...event,
    id: globalThis.crypto?.randomUUID?.() ?? `evt-${Date.now()}`,
    timestamp: new Date().toISOString()
  };

  events.push(next);
  writeJson(AUDIT_EVENTS_KEY, events);
}
