import { type FormEvent, useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { addAuditEvent, upsertStoredGuidelineRule } from "../app/localState";
import {
  buildKeywordInstructions,
  parseKeywordTerms,
  type GuidelineRuleType
} from "../app/guidelineRules";

export function GuidelineCreatePage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [ruleName, setRuleName] = useState("Governing law clause");
  const [ruleType, setRuleType] = useState<GuidelineRuleType>("clause_presence");
  const [ruleText, setRuleText] = useState(
    "This Agreement is governed by the laws of Estonia."
  );
  const [requiredTermsText, setRequiredTermsText] = useState("osaühing\naktsiaselts");
  const [forbiddenTermsText, setForbiddenTermsText] = useState("");
  const [autoRunOnNewContract, setAutoRunOnNewContract] = useState(false);
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

    const requiredTerms = parseKeywordTerms(requiredTermsText);
    const forbiddenTerms = parseKeywordTerms(forbiddenTermsText);

    if (ruleType !== "keyword_match" && ruleText.trim().length < 10) {
      setError("Enter clause text or review instructions with at least 10 characters.");
      return;
    }

    if (ruleType === "keyword_match" && requiredTerms.length === 0 && forbiddenTerms.length === 0) {
      setError("Add at least one required or forbidden keyword.");
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
        rule_type: ruleType,
        auto_run_on_new_contract: autoRunOnNewContract,
        instructions:
          ruleType !== "keyword_match"
            ? ruleText.trim()
            : buildKeywordInstructions(requiredTerms, forbiddenTerms),
        required_terms: ruleType === "keyword_match" ? requiredTerms : [],
        forbidden_terms: ruleType === "keyword_match" ? forbiddenTerms : [],
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

      <form className="panel" onSubmit={startCheck}>
        <div className="guideline-form">
          <label className="guideline-field">
            <span className="field-label">Rule Name</span>
            <input value={ruleName} onChange={(event) => setRuleName(event.target.value)} required />
          </label>

          <div className="form-grid guideline-form-grid">
            <label className="guideline-field">
              <span className="field-label">Rule Type</span>
              <select value={ruleType} onChange={(event) => setRuleType(event.target.value as GuidelineRuleType)}>
                <option value="clause_presence">Lexical clause check</option>
                <option value="gemini_review">Gemini contract review</option>
                <option value="keyword_match">Strict keyword check</option>
              </select>
            </label>

            {ruleType === "clause_presence" ? (
              <>
                <label className="guideline-field guideline-field-wide">
                  <span className="field-label">Clause Text to Look For</span>
                  <textarea
                    value={ruleText}
                    onChange={(event) => setRuleText(event.target.value)}
                    rows={7}
                    placeholder="Enter the clause wording you expect to find in the contract."
                    required
                  />
                </label>
                <div className="guideline-type-explainer guideline-type-explainer-rich guideline-field-wide">
                  <strong>How lexical clause check works</strong>
                  <p>
                    This check does not use an LLM. It compares your clause text against indexed document chunks and
                    scores the best matching chunk using phrase containment and token overlap.
                  </p>
                  <div className="guideline-type-grid">
                    <div>
                      <span className="field-label">What to enter</span>
                      <p>
                        Paste the exact clause wording you expect to see, or a short representative version of it. Use
                        the key legal phrasing, not a broad instruction like “check payment terms”.
                      </p>
                    </div>
                    <div>
                      <span className="field-label">What improves confidence</span>
                      <p>
                        Confidence is higher when one chunk contains the same important words and phrases as your input.
                        Exact wording usually performs best.
                      </p>
                    </div>
                    <div>
                      <span className="field-label">Good examples</span>
                      <p>
                        “The Supplier shall maintain confidentiality of all Customer Data.” or “This Agreement is
                        governed by the laws of Estonia.”
                      </p>
                    </div>
                    <div>
                      <span className="field-label">Avoid</span>
                      <p>
                        Long multi-topic instructions, questions, or legal summaries. The check is looking for clause
                        wording, not reasoning about intent.
                      </p>
                    </div>
                  </div>
                </div>
              </>
            ) : ruleType === "gemini_review" ? (
              <>
                <label className="guideline-field guideline-field-wide">
                  <span className="field-label">Review Instructions</span>
                  <textarea
                    value={ruleText}
                    onChange={(event) => setRuleText(event.target.value)}
                    rows={7}
                    placeholder="Explain what Gemini should review across the whole contract."
                    required
                  />
                </label>
                <div className="guideline-type-explainer guideline-type-explainer-rich guideline-field-wide">
                  <strong>How Gemini contract review works</strong>
                  <p>
                    This rule sends the full extracted contract text and your instructions to Gemini. Use it when you
                    need interpretation, judgment, or review that goes beyond phrase matching.
                  </p>
                  <div className="guideline-type-grid">
                    <div>
                      <span className="field-label">What to enter</span>
                      <p>
                        Write a focused review task such as what risk to assess, what requirement to confirm, and when
                        the result should be match, missing, or review.
                      </p>
                    </div>
                    <div>
                      <span className="field-label">Best use cases</span>
                      <p>
                        Use Gemini when wording may vary, the answer depends on legal meaning, or the rule requires
                        weighing multiple parts of the contract together.
                      </p>
                    </div>
                    <div>
                      <span className="field-label">Good example</span>
                      <p>
                        “Review whether the contract gives either party a termination for convenience right. Return
                        match only if the right is clearly present, missing only if clearly absent, and review if the
                        wording is ambiguous.”
                      </p>
                    </div>
                    <div>
                      <span className="field-label">Keep in mind</span>
                      <p>
                        This path is slower and depends on extracted text quality, but it can handle paraphrases and
                        more nuanced legal reasoning than the lexical check.
                      </p>
                    </div>
                  </div>
                </div>
              </>
            ) : (
              <>
                <label className="guideline-field">
                  <span className="field-label">Must Contain Words or Phrases</span>
                  <textarea
                    value={requiredTermsText}
                    onChange={(event) => setRequiredTermsText(event.target.value)}
                    rows={6}
                    placeholder={"payment terms\nEstonian law"}
                  />
                </label>
                <label className="guideline-field">
                  <span className="field-label">Must Not Contain Words or Phrases</span>
                  <textarea
                    value={forbiddenTermsText}
                    onChange={(event) => setForbiddenTermsText(event.target.value)}
                    rows={6}
                    placeholder={"draft only\nunlimited liability"}
                  />
                </label>
                <div className="guideline-type-explainer guideline-field-wide">
                  <strong>Strict keyword matching</strong>
                  <p>
                    Each phrase is matched against the extracted contract text without caring about uppercase or
                    lowercase letters.
                  </p>
                </div>
              </>
            )}
          </div>

          <label className="guideline-field">
            <span className="field-label">Automatic Run</span>
            <span className="checkbox-row">
              <input
                type="checkbox"
                aria-label="Run this rule automatically for every new contract."
                checked={autoRunOnNewContract}
                onChange={(event) => setAutoRunOnNewContract(event.target.checked)}
              />
              Run this rule automatically for every new contract.
            </span>
          </label>
        </div>

        {error ? <p className="error-text">{error}</p> : null}
        <button type="submit" disabled={submitting}>{submitting ? "Saving..." : "Save Rule"}</button>
      </form>
    </section>
  );
}
