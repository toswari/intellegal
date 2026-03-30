import { appConfig } from "../config/env";

export type DocumentStatus = "ingested" | "processing" | "indexed" | "failed";
export type CheckRunStatus = "queued" | "running" | "completed" | "failed";
export type CheckType = "clause_presence" | "company_name";

export type ErrorEnvelope = {
  error: {
    code: string;
    message: string;
    retriable: boolean;
    details?: Record<string, unknown>;
  };
};

export type CreateDocumentRequest = {
  source_type?: "repository" | "upload" | "api";
  source_ref?: string;
  filename: string;
  mime_type: "application/pdf" | "image/jpeg";
  content_base64: string;
  tags?: string[];
};

export type DocumentResponse = {
  id: string;
  source_type?: string;
  source_ref?: string;
  filename: string;
  mime_type: string;
  status: DocumentStatus;
  checksum?: string;
  created_at: string;
  updated_at: string;
};

export type DocumentListResponse = {
  items: DocumentResponse[];
  limit: number;
  offset: number;
  total: number;
};

export type ClauseCheckRequest = {
  document_ids?: string[];
  required_clause_text: string;
  context_hint?: string;
};

export type CompanyNameCheckRequest = {
  document_ids?: string[];
  old_company_name: string;
  new_company_name?: string;
};

export type CheckAcceptedResponse = {
  check_id: string;
  status: CheckRunStatus;
  check_type: CheckType;
};

export type CheckRunResponse = {
  check_id: string;
  status: CheckRunStatus;
  check_type: CheckType;
  requested_at: string;
  finished_at?: string;
  failure_reason?: string;
};

export type EvidenceSnippet = {
  snippet_text: string;
  page_number: number;
  chunk_id?: string;
  score?: number;
};

export type CheckResultItem = {
  document_id: string;
  outcome: "match" | "missing" | "review";
  confidence: number;
  summary?: string;
  evidence?: EvidenceSnippet[];
};

export type CheckResultsResponse = {
  check_id: string;
  status: CheckRunStatus;
  items: CheckResultItem[];
};

export type RequestOptions = {
  idempotencyKey?: string;
  signal?: AbortSignal;
};

export class ApiError extends Error {
  readonly status: number;
  readonly retriable: boolean;
  readonly code: string;
  readonly details?: Record<string, unknown>;

  constructor(status: number, payload: ErrorEnvelope) {
    super(payload.error.message);
    this.name = "ApiError";
    this.status = status;
    this.retriable = payload.error.retriable;
    this.code = payload.error.code;
    this.details = payload.error.details;
  }
}

export type FetchLike = typeof fetch;

export class ApiClient {
  private readonly baseUrl: string;
  private readonly fetchFn: FetchLike;

  constructor(baseUrl = appConfig.apiBaseUrl, fetchFn: FetchLike = fetch) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    // Preserve a stable global receiver for native fetch implementations.
    this.fetchFn = ((input: RequestInfo | URL, init?: RequestInit) =>
      Reflect.apply(fetchFn, globalThis, [input, init])) as FetchLike;
  }

  createDocument(body: CreateDocumentRequest, options?: RequestOptions) {
    return this.request<DocumentResponse>("POST", "/api/v1/documents", body, options);
  }

  listDocuments(params?: {
    status?: DocumentStatus;
    source_type?: "repository" | "upload" | "api";
    limit?: number;
    offset?: number;
  }) {
    const query = new URLSearchParams();
    if (params?.status) query.set("status", params.status);
    if (params?.source_type) query.set("source_type", params.source_type);
    if (params?.limit !== undefined) query.set("limit", String(params.limit));
    if (params?.offset !== undefined) query.set("offset", String(params.offset));

    const suffix = query.size > 0 ? `?${query.toString()}` : "";
    return this.request<DocumentListResponse>("GET", `/api/v1/documents${suffix}`);
  }

  getDocument(documentId: string) {
    return this.request<DocumentResponse>("GET", `/api/v1/documents/${encodeURIComponent(documentId)}`);
  }

  startClausePresenceCheck(body: ClauseCheckRequest, options?: RequestOptions) {
    return this.request<CheckAcceptedResponse>(
      "POST",
      "/api/v1/checks/clause-presence",
      body,
      options
    );
  }

  startCompanyNameCheck(body: CompanyNameCheckRequest, options?: RequestOptions) {
    return this.request<CheckAcceptedResponse>(
      "POST",
      "/api/v1/checks/company-name",
      body,
      options
    );
  }

  getCheckRun(checkId: string) {
    return this.request<CheckRunResponse>("GET", `/api/v1/checks/${encodeURIComponent(checkId)}`);
  }

  getCheckResults(checkId: string) {
    return this.request<CheckResultsResponse>(
      "GET",
      `/api/v1/checks/${encodeURIComponent(checkId)}/results`
    );
  }

  private async request<T>(
    method: "GET" | "POST",
    path: string,
    body?: unknown,
    options?: RequestOptions
  ): Promise<T> {
    const headers: Record<string, string> = {
      Accept: "application/json"
    };

    if (options?.idempotencyKey) {
      headers["Idempotency-Key"] = options.idempotencyKey;
    }

    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
    }

    const response = await this.fetchFn(`${this.baseUrl}${path}`, {
      method,
      headers,
      signal: options?.signal,
      body: body === undefined ? undefined : JSON.stringify(body)
    });

    if (!response.ok) {
      const payload = (await response.json()) as ErrorEnvelope;
      throw new ApiError(response.status, payload);
    }

    return (await response.json()) as T;
  }
}

export const apiClient = new ApiClient();
