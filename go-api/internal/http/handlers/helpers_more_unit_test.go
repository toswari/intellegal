//go:build !integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/db"

	"github.com/go-chi/chi/v5"
)

func TestDecodeJSON_RejectsUnknownFields(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/checks/clause-presence", bytes.NewReader([]byte(`{
		"required_clause_text":"payment terms",
		"unexpected":"value"
	}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.CreateClauseCheck(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := decodeJSONBody(t, rec)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument, got %q", body.Error.Code)
	}
}

func TestExtensionForFilename_UsesFilenameThenMIMEFallback(t *testing.T) {
	if got := extensionForFilename("contract.PDF", "image/png"); got != ".pdf" {
		t.Fatalf("expected filename extension to win, got %q", got)
	}
	if got := extensionForFilename("contract", "image/png"); got != ".png" {
		t.Fatalf("expected png fallback, got %q", got)
	}
	if got := extensionForFilename("contract", "image/jpeg"); got != ".jpg" {
		t.Fatalf("expected jpg default, got %q", got)
	}
}

func TestPathParam_UsesChiParamBeforePathValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ignored", nil)
	req.SetPathValue("document_id", " path-value ")

	routeCtx := chiRouteContextWithParam("document_id", " chi-value ")
	req = req.WithContext(routeCtx)

	if got := pathParam(req, "document_id"); got != "chi-value" {
		t.Fatalf("expected chi route param, got %q", got)
	}
}

func chiRouteContextWithParam(key, value string) context.Context {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return context.WithValue(context.Background(), chi.RouteCtxKey, routeCtx)
}

func TestSearchCandidateLimit_OverfetchesForContractMode(t *testing.T) {
	if got := searchCandidateLimit(4, "sections"); got != 4 {
		t.Fatalf("expected unchanged limit, got %d", got)
	}
	if got := searchCandidateLimit(4, "contracts"); got != 20 {
		t.Fatalf("expected overfetch limit 20, got %d", got)
	}
	if got := searchCandidateLimit(20, "contracts"); got != 50 {
		t.Fatalf("expected capped limit 50, got %d", got)
	}
}

func TestCollapseContractSearchResults_KeepsBestItemPerGroup(t *testing.T) {
	items := collapseContractSearchResults([]contractSearchResultItem{
		{DocumentID: "doc-1", ContractID: "contract-1", Score: 0.61, PageNumber: 5},
		{DocumentID: "doc-2", ContractID: "contract-1", Score: 0.91, PageNumber: 2},
		{DocumentID: "doc-3", ContractID: "", Score: 0.88, PageNumber: 1},
		{DocumentID: "doc-4", ContractID: "", Score: 0.77, PageNumber: 1},
	}, 3)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].DocumentID != "doc-2" {
		t.Fatalf("expected best contract hit first, got %q", items[0].DocumentID)
	}
	if items[1].DocumentID != "doc-3" {
		t.Fatalf("expected best standalone document next, got %q", items[1].DocumentID)
	}
}

func TestMapAnalysisItems_FallsBackForMissingDocuments(t *testing.T) {
	items := mapAnalysisItems(
		[]string{"doc-1", "doc-2"},
		[]ai.AnalysisResultItem{
			{
				DocumentID: "doc-1",
				Outcome:    "match",
				Confidence: 0.94,
				Summary:    "Found it",
				Evidence: []ai.AnalysisEvidenceSnippet{
					{SnippetText: "payment clause", PageNumber: 2, ChunkID: "chunk-1", Score: 0.88},
				},
			},
		},
		"fallback",
	)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Outcome != "match" || len(items[0].Evidence) != 1 {
		t.Fatalf("unexpected mapped item: %#v", items[0])
	}
	if items[1].Outcome != "review" || items[1].Summary != "fallback" {
		t.Fatalf("expected fallback item, got %#v", items[1])
	}
}

func TestHandleCreateCheckError_MapsExpectedStatuses(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		key  string
	}{
		{name: "db not configured", err: db.ErrNotConfigured, code: http.StatusServiceUnavailable, key: "service_unavailable"},
		{name: "idempotency conflict", err: errIdempotencyConflict, code: http.StatusConflict, key: "idempotency_conflict"},
		{name: "document missing", err: errors.New("document not found"), code: http.StatusBadRequest, key: "invalid_argument"},
		{name: "missing scope", err: errors.New("at least one document is required"), code: http.StatusUnprocessableEntity, key: "invalid_scope"},
		{name: "unexpected", err: errors.New("boom"), code: http.StatusInternalServerError, key: "internal_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			handleCreateCheckError(rec, tc.err)

			if rec.Code != tc.code {
				t.Fatalf("expected status %d, got %d", tc.code, rec.Code)
			}
			body := decodeJSONBody(t, rec)
			if body.Error.Code != tc.key {
				t.Fatalf("expected error code %q, got %q", tc.key, body.Error.Code)
			}
		})
	}
}

func TestCreateLLMReviewCheck_ReturnsBadRequestForShortInstructions(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	rec := performJSONRequest(t, http.MethodPost, "/api/v1/checks/llm-review", map[string]any{
		"instructions": "abc",
	}, api.CreateLLMReviewCheck)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := decodeJSONBody(t, rec)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument, got %q", body.Error.Code)
	}
}

func TestGetCheck_ReturnsStatusPayloadForCompletedCheck(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	checkID := "00000000-0000-4000-8000-000000000141"
	finishedAt := time.Now().UTC()
	api.checks[checkID] = checkRun{
		CheckID:       checkID,
		Status:        checkStatusCompleted,
		CheckType:     checkTypeClause,
		RequestedAt:   finishedAt.Add(-time.Minute),
		FinishedAt:    &finishedAt,
		FailureReason: "ignored once completed",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checks/"+checkID, nil)
	req.SetPathValue("check_id", checkID)
	rec := httptest.NewRecorder()

	api.GetCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body checkRunResponse
	decodeJSONBodyInto(t, rec, &body)
	if body.CheckID != checkID || body.Status != checkStatusCompleted || body.FinishedAt == nil {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestDeleteChecks_ReturnsBadRequestForEmptyList(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	rec := performJSONRequest(t, http.MethodDelete, "/api/v1/checks", map[string]any{
		"check_ids": []string{},
	}, api.DeleteChecks)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	body := decodeJSONBody(t, rec)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument, got %q", body.Error.Code)
	}
}

func TestSearchContracts_ReturnsEmptyWhenNoDocumentsResolved(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	rec := performJSONRequest(t, http.MethodPost, "/api/v1/contracts/search", map[string]any{
		"query_text": "payment terms",
	}, api.SearchContracts)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body contractSearchResponse
	decodeJSONBodyInto(t, rec, &body)
	if len(body.Items) != 0 {
		t.Fatalf("expected empty items, got %#v", body.Items)
	}
}

func TestContractChatDocuments_ReturnsExpectedValidationErrors(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	if _, err := api.contractChatDocuments("contract-1"); !errors.Is(err, db.ErrNotConfigured) {
		t.Fatalf("expected db err, got %v", err)
	}

	useInMemoryReaders(api)

	if _, err := api.contractChatDocuments("missing"); err == nil || err.Error() != "contract not found" {
		t.Fatalf("expected contract not found, got %v", err)
	}

	contractID := "00000000-0000-4000-8000-000000000101"
	now := time.Now().UTC()
	api.contracts[contractID] = contract{ID: contractID, FileIDs: nil, CreatedAt: now, UpdatedAt: now}

	if _, err := api.contractChatDocuments(contractID); err == nil || err.Error() != "no contract files" {
		t.Fatalf("expected no contract files, got %v", err)
	}

	documentID := "00000000-0000-4000-8000-000000000102"
	api.contracts[contractID] = contract{ID: contractID, FileIDs: []string{documentID}, CreatedAt: now, UpdatedAt: now}
	api.documents[documentID] = document{ID: documentID, Filename: "empty.pdf", ExtractedText: "   "}

	if _, err := api.contractChatDocuments(contractID); err == nil || err.Error() != "no extracted text" {
		t.Fatalf("expected no extracted text, got %v", err)
	}
}

func TestContractChatDocuments_TrimsAndFiltersDocuments(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	contractID := "00000000-0000-4000-8000-000000000111"
	firstDocumentID := "00000000-0000-4000-8000-000000000112"
	secondDocumentID := "00000000-0000-4000-8000-000000000113"
	now := time.Now().UTC()

	api.contracts[contractID] = contract{
		ID:        contractID,
		FileIDs:   []string{firstDocumentID, secondDocumentID},
		CreatedAt: now,
		UpdatedAt: now,
	}
	api.documents[firstDocumentID] = document{ID: firstDocumentID, Filename: "alpha.pdf", ExtractedText: "  Alpha text  "}
	api.documents[secondDocumentID] = document{ID: secondDocumentID, Filename: "blank.pdf", ExtractedText: ""}

	documents, err := api.contractChatDocuments(contractID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}
	if documents[0].Text != "Alpha text" {
		t.Fatalf("expected trimmed text, got %q", documents[0].Text)
	}
}

func TestChatContract_ReturnsBadGatewayWhenAIClientFails(t *testing.T) {
	aiClient := &capturingAIClient{chatErr: errors.New("upstream down")}
	api := NewAPI(noopLogger{}, aiClient, nil, nil)
	useInMemoryReaders(api)

	contractID := "00000000-0000-4000-8000-000000000121"
	documentID := "00000000-0000-4000-8000-000000000122"
	now := time.Now().UTC()
	api.contracts[contractID] = contract{ID: contractID, FileIDs: []string{documentID}, CreatedAt: now, UpdatedAt: now}
	api.documents[documentID] = document{ID: documentID, Filename: "alpha.pdf", ExtractedText: "Some text"}

	rec := performJSONRequest(t, http.MethodPost, "/api/v1/contracts/"+contractID+"/chat", map[string]any{
		"messages": []map[string]any{{"role": "user", "content": "Question?"}},
	}, func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("contract_id", contractID)
		api.ChatContract(w, r)
	})

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
	body := decodeJSONBody(t, rec)
	if body.Error.Code != "contract_chat_unavailable" {
		t.Fatalf("expected contract_chat_unavailable, got %q", body.Error.Code)
	}
}

func TestChatContract_ValidatesMessages(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{name: "missing messages", payload: map[string]any{"messages": []map[string]any{}}},
		{name: "invalid role", payload: map[string]any{"messages": []map[string]any{{"role": "system", "content": "Question?"}}}},
		{name: "empty content", payload: map[string]any{"messages": []map[string]any{{"role": "user", "content": "   "}}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := performJSONRequest(t, http.MethodPost, "/api/v1/contracts/not-a-uuid/chat", tc.payload, func(w http.ResponseWriter, r *http.Request) {
				r.SetPathValue("contract_id", "00000000-0000-4000-8000-000000000151")
				api.ChatContract(w, r)
			})

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			body := decodeJSONBody(t, rec)
			if body.Error.Code != "invalid_argument" {
				t.Fatalf("expected invalid_argument, got %q", body.Error.Code)
			}
		})
	}
}

func TestChatContract_FiltersBlankCitationsAndTrimsAnswer(t *testing.T) {
	aiClient := &capturingAIClient{}
	api := NewAPI(noopLogger{}, aiClient, nil, nil)
	useInMemoryReaders(api)

	contractID := "00000000-0000-4000-8000-000000000131"
	documentID := "00000000-0000-4000-8000-000000000132"
	now := time.Now().UTC()
	api.contracts[contractID] = contract{ID: contractID, FileIDs: []string{documentID}, CreatedAt: now, UpdatedAt: now}
	api.documents[documentID] = document{ID: documentID, Filename: "alpha.pdf", ExtractedText: "Some text"}
	aiClient.chatErr = nil

	rec := httptest.NewRecorder()
	reqBody, _ := json.Marshal(contractChatRequest{
		Messages: []contractChatMessageRequest{{Role: "user", Content: "Question?"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+contractID+"/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("contract_id", contractID)

	aiClient.chatErr = nil
	aiClient.chatReq = nil
	aiClient.chatErr = nil

	original := api.ai
	api.ai = contractChatFilteringStub{}
	defer func() { api.ai = original }()

	api.ChatContract(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body contractChatResponse
	decodeJSONBodyInto(t, rec, &body)
	if body.Answer != "Trim me" {
		t.Fatalf("expected trimmed answer, got %q", body.Answer)
	}
	if len(body.Citations) != 1 || body.Citations[0].DocumentID != documentID {
		t.Fatalf("unexpected citations: %#v", body.Citations)
	}
}

type contractChatFilteringStub struct{}

func (contractChatFilteringStub) AnalyzeClause(_ context.Context, _ ai.AnalyzeClauseRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}

func (contractChatFilteringStub) AnalyzeLLMReview(_ context.Context, _ ai.AnalyzeLLMReviewRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}

func (contractChatFilteringStub) ContractChat(_ context.Context, req ai.ContractChatRequest) (ai.ContractChatResult, error) {
	return ai.ContractChatResult{
		Answer: "  Trim me  ",
		Citations: []ai.ContractChatCitation{
			{DocumentID: req.Documents[0].DocumentID, SnippetText: "  useful snippet  ", Reason: "  because  "},
			{DocumentID: "", SnippetText: "skip"},
			{DocumentID: req.Documents[0].DocumentID, SnippetText: "   "},
		},
	}, nil
}

func (contractChatFilteringStub) Extract(_ context.Context, _ ai.ExtractRequest) (ai.ExtractResult, error) {
	return ai.ExtractResult{}, nil
}

func (contractChatFilteringStub) Index(_ context.Context, _ ai.IndexRequest) (ai.IndexResult, error) {
	return ai.IndexResult{}, nil
}

func (contractChatFilteringStub) SearchSections(_ context.Context, _ ai.SearchSectionsRequest) (ai.SearchSectionsResult, error) {
	return ai.SearchSectionsResult{}, nil
}
