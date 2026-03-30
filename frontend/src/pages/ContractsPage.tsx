import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  apiClient,
  type ContractResponse,
  type ContractSearchResultItem,
  type DocumentResponse,
  type DocumentStatus
} from "../api/client";
import { formatEuropeanDateTime } from "../app/datetime";

type Filters = {
  status: "all" | DocumentStatus;
  sourceType: "all" | "repository" | "upload" | "api";
  query: string;
  tagsInput: string;
};

export function ContractsPage() {
  const [searchParams] = useSearchParams();
  const [filters, setFilters] = useState<Filters>({ status: "all", sourceType: "all", query: "", tagsInput: "" });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [contracts, setContracts] = useState<ContractResponse[]>([]);
  const [documents, setDocuments] = useState<DocumentResponse[]>([]);
  const [deletingContractId, setDeletingContractId] = useState<string | null>(null);
  const [semanticSearching, setSemanticSearching] = useState(false);
  const [semanticSearched, setSemanticSearched] = useState(false);
  const [semanticSearchError, setSemanticSearchError] = useState<string | null>(null);
  const [semanticResults, setSemanticResults] = useState<ContractSearchResultItem[]>([]);
  const semanticQuery = searchParams.get("semanticQuery")?.trim() ?? "";
  const selectedTags = useMemo(
    () =>
      Array.from(
        new Set(
          filters.tagsInput
            .split(",")
            .map((tag) => tag.trim())
            .filter((tag) => tag.length > 0)
        )
      ),
    [filters.tagsInput]
  );

  const loadDocuments = async () => {
    setLoading(true);
    setError(null);
    try {
      const contractsResponse = await apiClient.listContracts({ limit: 200, offset: 0 });
      const response = await apiClient.listDocuments({
        status: filters.status === "all" ? undefined : filters.status,
        source_type: filters.sourceType === "all" ? undefined : filters.sourceType,
        tags: selectedTags.length > 0 ? selectedTags : undefined,
        limit: 200,
        offset: 0
      });
      setContracts(contractsResponse.items);
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
  }, [filters.status, filters.sourceType, selectedTags]);

  const filteredContracts = useMemo(() => {
    const query = filters.query.trim().toLowerCase();
    const matchingContracts = new Set(
      documents.map((document) => document.contract_id).filter((id): id is string => Boolean(id))
    );

    return contracts.filter((contract) => {
      if (
        (filters.status !== "all" || filters.sourceType !== "all" || selectedTags.length > 0) &&
        !matchingContracts.has(contract.id)
      ) {
        return false;
      }

      if (!query) return true;
      return (
        contract.name.toLowerCase().includes(query) ||
        contract.id.toLowerCase().includes(query) ||
        (contract.source_ref ?? "").toLowerCase().includes(query) ||
        (contract.tags ?? []).some((tag) => tag.toLowerCase().includes(query))
      );
    });
  }, [contracts, documents, filters.query, filters.sourceType, filters.status, selectedTags]);

  const handleDelete = async (contract: ContractResponse) => {
    const confirmed = window.confirm(
      `Delete "${contract.name}" permanently?\n\nThis will hard-delete all files in the contract and related data.`
    );
    if (!confirmed) {
      return;
    }

    setError(null);
    setDeletingContractId(contract.id);
    try {
      await apiClient.deleteContract(contract.id);
      setContracts((prev) => prev.filter((item) => item.id !== contract.id));
      setDocuments((prev) => prev.filter((item) => item.contract_id !== contract.id));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete document.";
      setError(message);
    } finally {
      setDeletingContractId(null);
    }
  };

  const sourceBadgeClass = (sourceType?: string) => {
    if (sourceType === "upload") return "chip chip-source-upload";
    if (sourceType === "repository") return "chip chip-source-repository";
    if (sourceType === "api") return "chip chip-source-api";
    return "chip chip-neutral";
  };

  const handleSemanticSearch = async (queryText: string) => {
    const query = queryText.trim();
    if (query.length < 2) {
      setSemanticSearchError("Enter at least 2 characters for semantic search.");
      return;
    }

    setSemanticSearching(true);
    setSemanticSearched(true);
    setSemanticSearchError(null);
    try {
      const visibleDocumentIds = documents.map((doc) => doc.id);
      const response = await apiClient.searchContractSections({
        query_text: query,
        document_ids: visibleDocumentIds.length > 0 ? visibleDocumentIds : undefined,
        limit: 12
      });
      setSemanticResults(response.items);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Semantic search failed.";
      setSemanticSearchError(message);
    } finally {
      setSemanticSearching(false);
    }
  };

  useEffect(() => {
    if (loading) {
      return;
    }
    if (semanticQuery.length < 2) {
      setSemanticSearched(false);
      setSemanticSearchError(null);
      setSemanticResults([]);
      return;
    }
    void handleSemanticSearch(semanticQuery);
  }, [semanticQuery, loading, documents]);

  const buildResultLink = (item: ContractSearchResultItem): string => {
    const params = new URLSearchParams();
    params.set("snippet", item.snippet_text.slice(0, 600));
    params.set("page", String(item.page_number));
    params.set("score", item.score.toFixed(3));
    if (item.contract_id) {
      params.set("contractId", item.contract_id);
    }
    return `/contracts/files/${encodeURIComponent(item.document_id)}?${params.toString()}`;
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
        {semanticSearchError ? <p className="error-text">{semanticSearchError}</p> : null}
        {!semanticSearchError && semanticResults.length === 0 && semanticSearched && !semanticSearching ? (
          <p className="muted">No semantic matches found for this query.</p>
        ) : null}
        {semanticResults.length > 0 ? (
          <section className="panel">
            <h3>Semantic Matches</h3>
            <div className="semantic-search-results">
              {semanticResults.map((item, index) => (
                <article key={`${item.document_id}-${item.chunk_id ?? index}`} className="semantic-search-item">
                  <div className="semantic-search-item-header">
                    <strong>{item.filename}</strong>
                    <span className="chip chip-neutral">Score {item.score.toFixed(3)}</span>
                  </div>
                  <p className="muted">
                    Page {item.page_number} | Document <code>{item.document_id}</code>
                  </p>
                  <p>{item.snippet_text}</p>
                  <Link className="button-link secondary" to={buildResultLink(item)}>
                    Open Match
                  </Link>
                  {item.contract_id ? (
                    <Link
                      className="button-link secondary"
                      to={`/contracts/${encodeURIComponent(item.contract_id)}/edit`}
                    >
                      Open Contract
                    </Link>
                  ) : null}
                </article>
              ))}
            </div>
          </section>
        ) : null}

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
          <label>
            Tags
            <input
              value={filters.tagsInput}
              onChange={(event) => setFilters((prev) => ({ ...prev, tagsInput: event.target.value }))}
              placeholder="filter by tags (comma-separated)"
            />
          </label>
        </div>

        {error ? <p className="error-text">{error}</p> : null}
        {loading ? <p className="muted">Loading documents...</p> : null}
        {!loading && filteredContracts.length === 0 ? <p className="muted">No contracts found.</p> : null}

        {filteredContracts.length > 0 ? (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Files</th>
                  <th>Source</th>
                  <th>Tags</th>
                  <th>Created</th>
                  <th>Contract ID</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {filteredContracts.map((contract) => (
                  <tr key={contract.id}>
                    <td>
                      <strong>
                        <Link to={`/contracts/${encodeURIComponent(contract.id)}/edit`}>{contract.name}</Link>
                      </strong>
                    </td>
                    <td>{contract.file_count}</td>
                    <td><span className={sourceBadgeClass(contract.source_type)}>{contract.source_type ?? "-"}</span></td>
                    <td>
                      {(contract.tags ?? []).length > 0 ? (
                        <div className="tag-list">
                          {(contract.tags ?? []).map((tag) => (
                            <span key={`${contract.id}-${tag}`} className="chip chip-tag">
                              {tag}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="muted">-</span>
                      )}
                    </td>
                    <td>{formatEuropeanDateTime(contract.created_at)}</td>
                    <td>
                      <code>{contract.id}</code>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="danger"
                        disabled={deletingContractId !== null}
                        onClick={() => void handleDelete(contract)}
                      >
                        {deletingContractId === contract.id ? "Deleting..." : "Delete"}
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
