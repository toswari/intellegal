package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"legal-doc-intel/go-api/internal/ai"
)

type capturingAIClient struct {
	clauseReq  *ai.AnalyzeClauseRequest
	companyReq *ai.AnalyzeCompanyNameRequest
	extractReq *ai.ExtractRequest
	indexReq   *ai.IndexRequest
	clauseErr  error
	companyErr error
	extractErr error
	indexErr   error
}

func (c *capturingAIClient) AnalyzeClause(_ context.Context, req ai.AnalyzeClauseRequest) (ai.AnalysisResult, error) {
	copyReq := req
	copyReq.DocumentIDs = append([]string(nil), req.DocumentIDs...)
	c.clauseReq = &copyReq
	if c.clauseErr != nil {
		return ai.AnalysisResult{}, c.clauseErr
	}
	items := make([]ai.AnalysisResultItem, 0, len(req.DocumentIDs))
	for _, documentID := range req.DocumentIDs {
		items = append(items, ai.AnalysisResultItem{
			DocumentID: documentID,
			Outcome:    "match",
			Confidence: 0.86,
			Summary:    "Clause evidence found.",
			Evidence: []ai.AnalysisEvidenceSnippet{
				{SnippetText: "must include payment terms", PageNumber: 1, ChunkID: "1", Score: 0.91},
			},
		})
	}
	return ai.AnalysisResult{Items: items}, nil
}

func (c *capturingAIClient) AnalyzeCompanyName(_ context.Context, req ai.AnalyzeCompanyNameRequest) (ai.AnalysisResult, error) {
	copyReq := req
	copyReq.DocumentIDs = append([]string(nil), req.DocumentIDs...)
	c.companyReq = &copyReq
	if c.companyErr != nil {
		return ai.AnalysisResult{}, c.companyErr
	}
	items := make([]ai.AnalysisResultItem, 0, len(req.DocumentIDs))
	for _, documentID := range req.DocumentIDs {
		items = append(items, ai.AnalysisResultItem{
			DocumentID: documentID,
			Outcome:    "review",
			Confidence: 0.6,
			Summary:    "Both old and new names found.",
		})
	}
	return ai.AnalysisResult{Items: items}, nil
}

func (c *capturingAIClient) Extract(_ context.Context, req ai.ExtractRequest) (ai.ExtractResult, error) {
	copyReq := req
	c.extractReq = &copyReq
	if c.extractErr != nil {
		return ai.ExtractResult{}, c.extractErr
	}
	return ai.ExtractResult{
		MIMEType: req.MIMEType,
		Text:     "sample text",
		Pages: []ai.ExtractPage{
			{PageNumber: 1, Text: "sample text"},
		},
	}, nil
}

func (c *capturingAIClient) Index(_ context.Context, req ai.IndexRequest) (ai.IndexResult, error) {
	copyReq := req
	c.indexReq = &copyReq
	if c.indexErr != nil {
		return ai.IndexResult{}, c.indexErr
	}
	return ai.IndexResult{
		DocumentID: req.DocumentID,
		Checksum:   req.VersionChecksum,
		ChunkCount: 1,
		Indexed:    true,
	}, nil
}

func TestRunClauseCheckMarksCompletedAndPassesRequest(t *testing.T) {
	aiClient := &capturingAIClient{}
	api := NewAPI(noopLogger{}, aiClient, nil, nil)

	checkID := "00000000-0000-4000-8000-000000000011"
	docID := "00000000-0000-4000-8000-000000000012"
	api.checks[checkID] = checkRun{
		CheckID:     checkID,
		Status:      checkStatusQueued,
		CheckType:   checkTypeClause,
		RequestedAt: time.Now().UTC(),
		DocumentIDs: []string{docID},
	}

	api.runClauseCheck(checkID, clauseCheckRequest{
		RequiredClauseText: "must include payment terms",
		ContextHint:        "scope: fees",
	}, "req-123")

	if aiClient.clauseReq == nil {
		t.Fatal("expected AnalyzeClause to be called")
	}
	if aiClient.clauseReq.CheckID != checkID {
		t.Fatalf("expected check id %q, got %q", checkID, aiClient.clauseReq.CheckID)
	}
	if aiClient.clauseReq.RequestID != "req-123" {
		t.Fatalf("expected request id req-123, got %q", aiClient.clauseReq.RequestID)
	}
	if len(aiClient.clauseReq.DocumentIDs) != 1 || aiClient.clauseReq.DocumentIDs[0] != docID {
		t.Fatalf("unexpected document ids: %#v", aiClient.clauseReq.DocumentIDs)
	}

	run := api.checks[checkID]
	if run.Status != checkStatusCompleted {
		t.Fatalf("expected status completed, got %q", run.Status)
	}
	if run.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}
	if len(run.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(run.Items))
	}
	if run.Items[0].Outcome != "match" {
		t.Fatalf("expected mapped outcome match, got %q", run.Items[0].Outcome)
	}
	if len(run.Items[0].Evidence) != 1 {
		t.Fatalf("expected evidence to be mapped, got %d snippets", len(run.Items[0].Evidence))
	}
}

func TestRunCompanyNameCheckMarksFailedWhenAIClientReturnsError(t *testing.T) {
	aiClient := &capturingAIClient{companyErr: errors.New("upstream timeout")}
	api := NewAPI(noopLogger{}, aiClient, nil, nil)

	checkID := "00000000-0000-4000-8000-000000000021"
	docID := "00000000-0000-4000-8000-000000000022"
	api.checks[checkID] = checkRun{
		CheckID:     checkID,
		Status:      checkStatusQueued,
		CheckType:   checkTypeCompany,
		RequestedAt: time.Now().UTC(),
		DocumentIDs: []string{docID},
	}

	api.runCompanyNameCheck(checkID, companyNameCheckRequest{
		OldCompanyName: "Old Corp",
		NewCompanyName: "New Corp",
	}, "req-789")

	if aiClient.companyReq == nil {
		t.Fatal("expected AnalyzeCompanyName to be called")
	}
	if aiClient.companyReq.CheckID != checkID {
		t.Fatalf("expected check id %q, got %q", checkID, aiClient.companyReq.CheckID)
	}
	if aiClient.companyReq.RequestID != "req-789" {
		t.Fatalf("expected request id req-789, got %q", aiClient.companyReq.RequestID)
	}

	run := api.checks[checkID]
	if run.Status != checkStatusFailed {
		t.Fatalf("expected status failed, got %q", run.Status)
	}
	if run.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}
	if run.FailureReason != "upstream timeout" {
		t.Fatalf("expected failure reason to be propagated, got %q", run.FailureReason)
	}
}
