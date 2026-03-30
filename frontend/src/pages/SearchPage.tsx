import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiClient, type ContractSearchResultItem } from "../api/client";

function inferSectionHint(snippet: string): string | null {
  const normalized = snippet.replace(/\s+/g, " ").trim();
  if (!normalized) {
    return null;
  }

  const namedMatch = normalized.match(/\b(?:section|clause)\s+([A-Za-z0-9.-]{1,20})\b/i);
  if (namedMatch) {
    return `Section ${namedMatch[1]}`;
  }

  const numericMatch = normalized.match(/\b(\d+(?:\.\d+){1,5})\b/);
  if (numericMatch) {
    return `Section ${numericMatch[1]}`;
  }

  return null;
}

function buildResultLink(item: ContractSearchResultItem): string {
  const params = new URLSearchParams();
  params.set("snippet", item.snippet_text.slice(0, 600));
  params.set("page", String(item.page_number));
  params.set("score", item.score.toFixed(3));
  if (item.contract_id) {
    params.set("contractId", item.contract_id);
  }
  return `/contracts/files/${encodeURIComponent(item.document_id)}?${params.toString()}`;
}

export function SearchPage() {
  const [searchParams] = useSearchParams();
  const [searching, setSearching] = useState(false);
  const [searched, setSearched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [results, setResults] = useState<ContractSearchResultItem[]>([]);

  const query = searchParams.get("q")?.trim() ?? "";

  useEffect(() => {
    const run = async () => {
      if (query.length < 2) {
        setSearched(false);
        setError(null);
        setResults([]);
        return;
      }

      setSearching(true);
      setSearched(true);
      setError(null);
      try {
        const response = await apiClient.searchContractSections({
          query_text: query,
          limit: 30
        });
        setResults(response.items);
      } catch (err) {
        const message = err instanceof Error ? err.message : "Semantic search failed.";
        setError(message);
      } finally {
        setSearching(false);
      }
    };

    void run();
  }, [query]);

  const resultCountLabel = useMemo(() => {
    if (!searched || query.length < 2) {
      return "Enter at least 2 characters to search contract sections.";
    }
    if (searching) {
      return "Searching contract sections...";
    }
    if (error) {
      return "";
    }
    return `${results.length} section matches`;
  }, [error, query, results.length, searched, searching]);

  return (
    <section className="page">
      <header className="page-header">
        <h2>Search</h2>
      </header>

      <section className="panel search-view-panel">
        <p className="muted">
          Query: <strong>{query || "-"}</strong>
        </p>
        {resultCountLabel ? <p className="muted">{resultCountLabel}</p> : null}
        {error ? <p className="error-text">{error}</p> : null}

        {results.length > 0 ? (
          <div className="search-results-grid">
            {results.map((item, index) => {
              const sectionHint = inferSectionHint(item.snippet_text);
              return (
                <article key={`${item.document_id}-${item.chunk_id ?? index}`} className="search-result-card">
                  <div className="search-result-head">
                    <strong>{item.filename}</strong>
                    <span className="chip chip-neutral">Score {item.score.toFixed(3)}</span>
                  </div>
                  <p className="muted">
                    Page {item.page_number} | Document <code>{item.document_id}</code>
                  </p>
                  {sectionHint ? <p className="search-section-hint">{sectionHint}</p> : null}
                  <p>{item.snippet_text}</p>
                  <div className="search-result-actions">
                    <Link className="button-link secondary" to={buildResultLink(item)}>
                      Open Match
                    </Link>
                    {item.contract_id ? (
                      <Link className="button-link secondary" to={`/contracts/${encodeURIComponent(item.contract_id)}/edit`}>
                        Open Contract
                      </Link>
                    ) : null}
                  </div>
                </article>
              );
            })}
          </div>
        ) : null}
      </section>
    </section>
  );
}
