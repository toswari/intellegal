//go:build !integration

package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/externalcopy"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListContracts_ReturnsServiceUnavailableWithoutReader(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)

	api.ListContracts(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "service_unavailable", body.Error.Code)
}

func TestListContracts_ReturnsPaginatedContracts(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	now := time.Now().UTC()
	api.contracts["00000000-0000-4000-8000-000000000201"] = contract{ID: "00000000-0000-4000-8000-000000000201", Name: "Older", SourceType: "upload", FileIDs: []string{"a"}, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)}
	api.contracts["00000000-0000-4000-8000-000000000202"] = contract{ID: "00000000-0000-4000-8000-000000000202", Name: "Newer", SourceType: "api", FileIDs: []string{"b", "c"}, CreatedAt: now, UpdatedAt: now}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?limit=1&offset=0", nil)

	api.ListContracts(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body contractListResponse
	decodeJSONBodyInto(t, rec, &body)
	assert.Equal(t, 2, body.Total)
	assert.Equal(t, 1, body.Limit)
	require.Len(t, body.Items, 1)
	assert.Equal(t, "00000000-0000-4000-8000-000000000202", body.Items[0].ID)
	assert.Equal(t, 2, body.Items[0].FileCount)
}

func TestGetDocument_ReturnsDocument(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	documentID := "00000000-0000-4000-8000-000000000211"
	now := time.Now().UTC()
	api.documents[documentID] = document{
		ID:         documentID,
		Filename:   "contract.pdf",
		MIMEType:   "application/pdf",
		Status:     documentStatusIndexed,
		SourceType: "upload",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+documentID, nil)
	req.SetPathValue("document_id", documentID)

	api.GetDocument(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body documentResponse
	decodeJSONBodyInto(t, rec, &body)
	assert.Equal(t, documentID, body.ID)
	assert.Equal(t, documentStatusIndexed, body.Status)
}

func TestGetDocument_ReturnsNotFound(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/00000000-0000-4000-8000-000000000212", nil)
	req.SetPathValue("document_id", "00000000-0000-4000-8000-000000000212")

	api.GetDocument(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetDocumentText_ReturnsHasTextFalseWhenBlank(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	documentID := "00000000-0000-4000-8000-000000000221"
	now := time.Now().UTC()
	api.documents[documentID] = document{
		ID:            documentID,
		Filename:      "contract.pdf",
		MIMEType:      "application/pdf",
		ExtractedText: "   ",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+documentID+"/text", nil)
	req.SetPathValue("document_id", documentID)

	api.GetDocumentText(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body documentTextResponse
	decodeJSONBodyInto(t, rec, &body)
	assert.False(t, body.HasText)
	assert.Empty(t, body.Text)
}

func TestGetDocumentContent_ReturnsFileContent(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, &stubDocumentStore{getBody: []byte("file-body")}, nil)
	useInMemoryReaders(api)

	documentID := "00000000-0000-4000-8000-000000000231"
	now := time.Now().UTC()
	api.documents[documentID] = document{
		ID:         documentID,
		Filename:   "contract.pdf",
		MIMEType:   "application/pdf",
		StorageKey: "documents/contract.pdf",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+documentID+"/content", nil)
	req.SetPathValue("document_id", documentID)

	api.GetDocumentContent(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "file-body", rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
}

func TestGetDocumentContent_ReturnsBadGatewayWhenStoreReadFails(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, &stubDocumentStore{getErr: errors.New("boom")}, nil)
	useInMemoryReaders(api)

	documentID := "00000000-0000-4000-8000-000000000232"
	now := time.Now().UTC()
	api.documents[documentID] = document{
		ID:         documentID,
		Filename:   "contract.pdf",
		MIMEType:   "application/pdf",
		StorageKey: "documents/contract.pdf",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/"+documentID+"/content", nil)
	req.SetPathValue("document_id", documentID)

	api.GetDocumentContent(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	body := decodeJSONBody(t, rec)
	assert.Equal(t, "storage_unavailable", body.Error.Code)
}

func TestHealth_ReturnsOKResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	Health(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body livenessResponse
	decodeJSONBodyInto(t, rec, &body)
	assert.Equal(t, "ok", body.Status)
	assert.NotEmpty(t, body.Timestamp)
}

func TestReadiness_ReportsProbeStatus(t *testing.T) {
	handler := Readiness(
		NewDependencyProbe("db", func(context.Context) error { return nil }),
		NewDependencyProbe("search", func(context.Context) error { return errors.New("offline") }),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	handler(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var body readinessResponse
	decodeJSONBodyInto(t, rec, &body)
	assert.Equal(t, "not_ready", body.Status)
	assert.Equal(t, "up", body.Dependencies["db"].Status)
	assert.Equal(t, "down", body.Dependencies["search"].Status)
}

func TestCreateCheck_ReusesIdempotentRequestAndRejectsConflicts(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	docID := "00000000-0000-4000-8000-000000000241"
	api.documents[docID] = document{ID: docID, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}

	req1 := httptest.NewRequest(http.MethodPost, "/checks", nil)
	req1.Header.Set("Idempotency-Key", "idem-key-1")
	checkID, status, reused, err := api.createCheck(req1, checkTypeClause, clauseCheckRequest{RequiredClauseText: "payment terms"}, []string{docID})
	require.NoError(t, err)
	assert.False(t, reused)
	assert.Equal(t, checkStatusQueued, status)
	assert.NotEmpty(t, checkID)

	req2 := httptest.NewRequest(http.MethodPost, "/checks", nil)
	req2.Header.Set("Idempotency-Key", "idem-key-1")
	reusedID, reusedStatus, reusedFlag, err := api.createCheck(req2, checkTypeClause, clauseCheckRequest{RequiredClauseText: "payment terms"}, []string{docID})
	require.NoError(t, err)
	assert.True(t, reusedFlag)
	assert.Equal(t, checkID, reusedID)
	assert.Equal(t, checkStatusQueued, reusedStatus)

	req3 := httptest.NewRequest(http.MethodPost, "/checks", nil)
	req3.Header.Set("Idempotency-Key", "idem-key-1")
	_, _, _, err = api.createCheck(req3, checkTypeClause, clauseCheckRequest{RequiredClauseText: "different terms"}, []string{docID})
	assert.ErrorIs(t, err, errIdempotencyConflict)
}

func TestResolveDocumentIDs_UsesAllDocumentsAndDeduplicatesExplicitIDs(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	useInMemoryReaders(api)

	firstID := "00000000-0000-4000-8000-000000000251"
	secondID := "00000000-0000-4000-8000-000000000252"
	api.documents[firstID] = document{ID: firstID}
	api.documents[secondID] = document{ID: secondID}

	all, err := api.resolveDocumentIDs(nil)
	require.NoError(t, err)
	assert.Equal(t, []string{firstID, secondID}, all)

	explicit, err := api.resolveDocumentIDs([]string{secondID, firstID, secondID})
	require.NoError(t, err)
	assert.Equal(t, []string{firstID, secondID}, explicit)
}

func TestMarkCheckFailed_StoresFailureReason(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	checkID := "00000000-0000-4000-8000-000000000261"
	api.checks[checkID] = checkRun{
		CheckID:     checkID,
		Status:      checkStatusRunning,
		CheckType:   checkTypeClause,
		RequestedAt: time.Now().UTC(),
	}

	api.markCheckFailed(checkID, errors.New("upstream failed"))

	run := api.checks[checkID]
	assert.Equal(t, checkStatusFailed, run.Status)
	assert.NotNil(t, run.FinishedAt)
	assert.Contains(t, run.FailureReason, "upstream failed")
}

func TestNoopImplementations_ReturnDefaults(t *testing.T) {
	var aiClient noopAIClient
	_, err := aiClient.AnalyzeClause(context.Background(), ai.AnalyzeClauseRequest{})
	require.NoError(t, err)
	_, err = aiClient.AnalyzeLLMReview(context.Background(), ai.AnalyzeLLMReviewRequest{})
	require.NoError(t, err)
	out, err := aiClient.ContractChat(context.Background(), ai.ContractChatRequest{})
	require.NoError(t, err)
	assert.Empty(t, out.Answer)
	searchOut, err := aiClient.SearchSections(context.Background(), ai.SearchSectionsRequest{})
	require.NoError(t, err)
	assert.Empty(t, searchOut.Items)

	var store noopDocumentStore
	reader, err := store.Get(context.Background(), "ignored")
	require.NoError(t, err)
	defer reader.Close()
	data, _ := io.ReadAll(reader)
	assert.Empty(t, string(data))
	require.NoError(t, store.Delete(context.Background(), "ignored"))

	var copier noopExternalCopyClient
	assert.False(t, copier.Enabled())
	_, err = copier.CopyDocument(context.Background(), externalcopy.CopyRequest{})
	assert.Error(t, err)
}
