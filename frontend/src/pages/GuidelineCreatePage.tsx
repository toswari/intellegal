import { type FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { addAuditEvent, upsertStoredGuidelineRule } from "../app/localState";

export function GuidelineCreatePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [ruleName, setRuleName] = useState("Estonian legal entity");
  const [ruleText, setRuleText] = useState(
    "Check whether the contracting company is clearly identified as an entity operating in the Estonian legal space. Review the company details, legal form, registration references, governing law, and any wording that confirms the company belongs to the Estonian legal framework."
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (searchParams.get("name")) {
      setRuleName(searchParams.get("name") ?? "");
    }
  }, [searchParams]);

  const startCheck = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (ruleName.trim().length < 3) {
      setError("Enter a rule name with at least 3 characters.");
      return;
    }

    if (ruleText.trim().length < 10) {
      setError("Enter rule instructions with at least 10 characters.");
      return;
    }

    setSubmitting(true);
    setError(null);

    try {
      const now = new Date().toISOString();
      const ruleId = globalThis.crypto?.randomUUID?.() ?? `rule-${Date.now()}`;
      upsertStoredGuidelineRule({
        id: ruleId,
        name: ruleName.trim(),
        instructions: ruleText.trim(),
        created_at: now,
        updated_at: now
      });

      addAuditEvent({
        type: "run.tracked",
        message: `Created guideline rule "${ruleName.trim()}"`,
        metadata: {
          rule_name: ruleName.trim()
        }
      });

      const params = new URLSearchParams();
      params.set("ruleId", ruleId);
      if (searchParams.get("scope") === "selected") {
        params.set("scope", "selected");
        for (const documentId of searchParams.getAll("documentId")) {
          params.append("documentId", documentId);
        }
      }

      navigate(`/guidelines/run?${params.toString()}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create guideline rule.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>New Guideline Rule</h2>
          <p className="muted">Create a reusable rule that you can run against different contract selections.</p>
        </div>
        <div className="page-actions">
          <Link to="/guidelines" className="button-link secondary">
            Back to Guidelines
          </Link>
        </div>
      </header>

      <section className="panel guideline-info-panel">
        <h3>How Guideline Rules Work</h3>
        <p className="muted">
          A guideline rule captures what should be checked in a contract. Once saved, it can be executed repeatedly
          against all contracts or a selected subset.
        </p>

        <div className="guideline-flow">
          <article className="guideline-flow-step">
            <strong>1. Name the rule</strong>
            <p>Give the rule a short title that explains what it verifies.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>2. Describe the check</strong>
            <p>Write the detailed instructions the system should use when reviewing contract text.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>3. Save for reuse</strong>
            <p>The rule becomes available in the Guidelines view and can be executed whenever needed.</p>
          </article>
          <article className="guideline-flow-step">
            <strong>4. Execute later</strong>
            <p>Choose the rule and run it against a specific contract set to produce an execution record.</p>
          </article>
        </div>
      </section>

      <form className="panel" onSubmit={startCheck}>
        <h3>Rule Details</h3>

        <div className="wizard-steps guideline-wizard">
          <div className="step">
            <strong>Step 1</strong>
            <label className="guideline-field">
              <span className="field-label">Rule Name</span>
              <input value={ruleName} onChange={(event) => setRuleName(event.target.value)} required />
            </label>
          </div>

          <div className="step">
            <strong>Step 2</strong>
            <div className="form-grid guideline-form-grid">
              <label className="guideline-field guideline-field-wide">
                <span className="field-label">Rule Instructions</span>
                <textarea
                  value={ruleText}
                  onChange={(event) => setRuleText(event.target.value)}
                  rows={7}
                  required
                />
              </label>
            </div>
          </div>
        </div>

        {error ? <p className="error-text">{error}</p> : null}
        <button type="submit" disabled={submitting}>{submitting ? "Saving..." : "Save Rule"}</button>
      </form>
    </section>
  );
}
