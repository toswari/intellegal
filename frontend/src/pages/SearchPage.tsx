import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiClient, type ContractSearchResultItem } from "../api/client";

type SearchStrategy = "semantic" | "strict";

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
  const [searched, setSearched] = useState(false);
  const [activeStrategy, setActiveStrategy] = useState<SearchStrategy>("semantic");
  const [semanticSearching, setSemanticSearching] = useState(false);
  const [strictSearching, setStrictSearching] = useState(false);
  const [semanticError, setSemanticError] = useState<string | null>(null);
  const [strictError, setStrictError] = useState<string | null>(null);
  const [semanticResults, setSemanticResults] = useState<ContractSearchResultItem[]>([]);
  const [strictResults, setStrictResults] = useState<ContractSearchResultItem[]>([]);

  const query = searchParams.get("q")?.trim() ?? "";

  useEffect(() => {
    const run = async () => {
      if (query.length < 2) {
        setSearched(false);
        setSemanticError(null);
        setStrictError(null);
        setSemanticResults([]);
        setStrictResults([]);
        setSemanticSearching(false);
        setStrictSearching(false);
        return;
      }

      setSearched(true);
      if (activeStrategy === "semantic") {
        setSemanticSearching(true);
        setSemanticError(null);
        try {
          const response = await apiClient.searchContractSections({
            query_text: query,
            strategy: "semantic",
            limit: 30
          });
          setSemanticResults(response.items);
        } catch (err) {
          setSemanticResults([]);
          const message = err instanceof Error ? err.message : "Semantic search failed.";
          setSemanticError(message);
        } finally {
          setSemanticSearching(false);
        }
        return;
      }

      setStrictSearching(true);
      setStrictError(null);
      try {
        const response = await apiClient.searchContractSections({
          query_text: query,
          strategy: "strict",
          limit: 30
        });
        setStrictResults(response.items);
      } catch (err) {
        setStrictResults([]);
        const message = err instanceof Error ? err.message : "Strict search failed.";
        setStrictError(message);
      } finally {
        setStrictSearching(false);
      }
    };

    void run();
  }, [activeStrategy, query]);

  const selectedResults = activeStrategy === "semantic" ? semanticResults : strictResults;
  const selectedError = activeStrategy === "semantic" ? semanticError : strictError;
  const selectedSearching = activeStrategy === "semantic" ? semanticSearching : strictSearching;

  const resultCountLabel = useMemo(() => {
    if (!searched || query.length < 2) {
      return "Enter at least 2 characters to search contract sections.";
    }
    if (selectedSearching) {
      return activeStrategy === "semantic"
        ? "Searching semantic matches..."
        : "Searching strict text matches...";
    }
    if (selectedError) {
      return "";
    }
    return `${selectedResults.length} section matches`;
  }, [activeStrategy, query, searched, selectedError, selectedResults.length, selectedSearching]);

  return (
    <section className="page">
      <header className="page-header">
        <h2>Search</h2>
      </header>

      <section className="search-view-panel">
        <p className="muted">
          Query: <strong>{query || "-"}</strong>
        </p>
        <div className="compare-tabs search-mode-tabs" role="tablist" aria-label="Search modes">
          <button
            type="button"
            role="tab"
            aria-selected={activeStrategy === "semantic"}
            className={activeStrategy === "semantic" ? "secondary" : undefined}
            onClick={() => setActiveStrategy("semantic")}
          >
            Similarity
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeStrategy === "strict"}
            className={activeStrategy === "strict" ? "secondary" : undefined}
            onClick={() => setActiveStrategy("strict")}
          >
            Strict
          </button>
        </div>
        {resultCountLabel ? <p className="muted">{resultCountLabel}</p> : null}
        {selectedError ? <p className="error-text">{selectedError}</p> : null}

        {selectedResults.length > 0 ? (
          <div className="search-results-grid">
            {selectedResults.map((item, index) => {
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
