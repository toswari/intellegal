//go:build !integration

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnalyzeClause_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var seenPath string
	var seenAuth string
	var seenInternalToken string
	var seenBody AnalyzeClauseRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		seenInternalToken = r.Header.Get("X-Internal-Service-Token")
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-1",
			"status":   "completed",
			"job_type": "analyze_clause",
			"result": map[string]any{
				"items": []map[string]any{
					{
						"document_id": "doc-1",
						"outcome":     "match",
						"confidence":  0.9,
					},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "secret-token", time.Second)

	// Act
	result, err := client.AnalyzeClause(context.Background(), AnalyzeClauseRequest{
		JobID:              "job-1",
		RequestID:          "req-1",
		CheckID:            "check-1",
		DocumentIDs:        []string{"doc-1"},
		RequiredClauseText: "must include payment terms",
		ContextHint:        "billing",
	})
	if err != nil {
		t.Fatalf("AnalyzeClause returned error: %v", err)
	}

	// Assert
	if seenPath != "/internal/v1/analyze/clause" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if seenAuth != "Bearer secret-token" {
		t.Fatalf("unexpected authorization header: %q", seenAuth)
	}
	if seenInternalToken != "secret-token" {
		t.Fatalf("unexpected internal service token header: %q", seenInternalToken)
	}
	if seenBody.CheckID != "check-1" || seenBody.RequiredClauseText == "" {
		t.Fatalf("unexpected body payload: %#v", seenBody)
	}
	if len(result.Items) != 1 || result.Items[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected analyze result items: %#v", result.Items)
	}
}

func TestAnalyzeLLMReview_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	// Arrange
	var seenPath string
	var seenBody AnalyzeLLMReviewRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-llm-1",
			"status":   "completed",
			"job_type": "analyze_llm_review",
			"result": map[string]any{
				"items": []map[string]any{
					{
						"document_id": "doc-1",
						"outcome":     "review",
						"confidence":  0.77,
					},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	// Act
	result, err := client.AnalyzeLLMReview(context.Background(), AnalyzeLLMReviewRequest{
		JobID:        "job-llm-1",
		CheckID:      "check-llm-1",
		DocumentIDs:  []string{"doc-1"},
		Instructions: "Review the entire contract for termination for convenience.",
		Documents: []AnalyzeDocument{
			{DocumentID: "doc-1", Filename: "contract.pdf", Text: "Contract text"},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeLLMReview returned error: %v", err)
	}

	// Assert
	if seenPath != "/internal/v1/analyze/llm-review" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if seenBody.Instructions == "" || len(seenBody.Documents) != 1 || seenBody.Documents[0].Text == "" {
		t.Fatalf("unexpected body payload: %#v", seenBody)
	}
	if len(result.Items) != 1 || result.Items[0].Outcome != "review" {
		t.Fatalf("unexpected llm review result items: %#v", result.Items)
	}
}
