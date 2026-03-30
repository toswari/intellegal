import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiClient, type DocumentResponse, type DocumentTextResponse } from "../api/client";
import { buildLineDiff, buildSemanticDiff } from "./contractCompare";

type CompareMode = "semantic" | "line";

type LoadedContract = {
  meta: DocumentResponse;
  text: DocumentTextResponse;
};

export function CompareContractsPage() {
  const [searchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [left, setLeft] = useState<LoadedContract | null>(null);
  const [right, setRight] = useState<LoadedContract | null>(null);
  const [mode, setMode] = useState<CompareMode>("semantic");

  const leftId = searchParams.get("left") ?? "";
  const rightId = searchParams.get("right") ?? "";

  useEffect(() => {
    let active = true;

    const load = async () => {
      if (!leftId || !rightId || leftId === rightId) {
        setLoading(false);
        setError("Choose two different contracts from the Contracts list.");
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const [leftMeta, leftText, rightMeta, rightText] = await Promise.all([
          apiClient.getDocument(leftId),
          apiClient.getDocumentText(leftId),
          apiClient.getDocument(rightId),
          apiClient.getDocumentText(rightId)
        ]);

        if (!active) {
          return;
        }

        setLeft({ meta: leftMeta, text: leftText });
        setRight({ meta: rightMeta, text: rightText });
      } catch (err) {
        if (!active) {
          return;
        }
        const message = err instanceof Error ? err.message : "Failed to load contract comparison.";
        setError(message);
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };

    void load();

    return () => {
      active = false;
    };
  }, [leftId, rightId]);

  const semanticRows = useMemo(() => {
    if (!left || !right) {
      return [];
    }
    return buildSemanticDiff(left.text.text, right.text.text);
  }, [left, right]);

  const lineDiff = useMemo(() => {
    if (!left || !right) {
      return { left: [], right: [], truncated: false };
    }
    return buildLineDiff(left.text.text, right.text.text);
  }, [left, right]);

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Compare Contracts</h2>
          <p className="muted">Compare extracted text with semantic sections or a line-by-line diff.</p>
        </div>
        <div className="page-actions">
          <Link to="/contracts" className="button-link secondary">
            Back to Contracts
          </Link>
        </div>
      </header>

      {error ? <p className="error-text">{error}</p> : null}
      {loading ? <p className="muted">Loading contracts...</p> : null}

      {!loading && !error && left && right ? (
        <>
          <div className="compare-header-grid">
            <article className="panel">
              <h3>{left.meta.filename}</h3>
              <p className="muted">
                <code>{left.meta.id}</code>
              </p>
              {!left.text.has_text ? <p className="error-text">No extracted text available.</p> : null}
            </article>
            <article className="panel">
              <h3>{right.meta.filename}</h3>
              <p className="muted">
                <code>{right.meta.id}</code>
              </p>
              {!right.text.has_text ? <p className="error-text">No extracted text available.</p> : null}
            </article>
          </div>

          <div className="compare-tabs" role="tablist" aria-label="Comparison modes">
            <button
              type="button"
              role="tab"
              aria-selected={mode === "semantic"}
              className={mode === "semantic" ? "secondary" : undefined}
              onClick={() => setMode("semantic")}
            >
              Semantic Sections
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={mode === "line"}
              className={mode === "line" ? "secondary" : undefined}
              onClick={() => setMode("line")}
            >
              Line Diff
            </button>
          </div>

          {mode === "semantic" ? (
            <div className="compare-semantic-grid">
              {semanticRows.map((row) => (
                <article key={row.key} className={`panel semantic-card semantic-${row.status}`}>
                  <header className="semantic-card-header">
                    <h4>{row.title}</h4>
                    <span className="chip chip-neutral">Match {Math.round(row.similarity * 100)}%</span>
                  </header>
                  <div className="split-grid compare-text-grid">
                    <div>
                      <strong>Contract A</strong>
                      <pre>{row.leftText || "Not clearly identified for this category."}</pre>
                    </div>
                    <div>
                      <strong>Contract B</strong>
                      <pre>{row.rightText || "Not clearly identified for this category."}</pre>
                    </div>
                  </div>
                </article>
              ))}
            </div>
          ) : (
            <section className="panel">
              {lineDiff.truncated ? (
                <p className="muted">Line diff is limited to first 700 non-empty lines per contract for performance.</p>
              ) : null}
              <div className="split-grid compare-diff-grid">
                <div>
                  <h4>Contract A</h4>
                  <div className="diff-block">
                    {lineDiff.left.map((line, index) => (
                      <div key={`left-${index}`} className={`diff-line diff-${line.kind}`}>
                        <code>{line.text || " "}</code>
                      </div>
                    ))}
                  </div>
                </div>
                <div>
                  <h4>Contract B</h4>
                  <div className="diff-block">
                    {lineDiff.right.map((line, index) => (
                      <div key={`right-${index}`} className={`diff-line diff-${line.kind}`}>
                        <code>{line.text || " "}</code>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </section>
          )}
        </>
      ) : null}
    </section>
  );
}
