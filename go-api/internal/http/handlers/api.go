package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/externalcopy"
	"legal-doc-intel/go-api/internal/http/middleware"
)

const (
	documentStatusIngested   = "ingested"
	documentStatusProcessing = "processing"
	documentStatusIndexed    = "indexed"
	documentStatusFailed     = "failed"
	checkStatusQueued        = "queued"
	checkStatusRunning       = "running"
	checkStatusCompleted     = "completed"
	checkStatusFailed        = "failed"
	checkTypeClause          = "clause_presence"
	checkTypeCompany         = "company_name"
)

var (
	uuidRx             = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	validDocumentMimes = map[string]struct{}{"application/pdf": {}, "image/jpeg": {}}
	validSourceTypes   = map[string]struct{}{"repository": {}, "upload": {}, "api": {}}
	validDocStatuses   = map[string]struct{}{documentStatusIngested: {}, documentStatusProcessing: {}, documentStatusIndexed: {}, documentStatusFailed: {}}
)

type aiClient interface {
	AnalyzeClause(ctx context.Context, req ai.AnalyzeClauseRequest) (ai.AnalysisResult, error)
	AnalyzeCompanyName(ctx context.Context, req ai.AnalyzeCompanyNameRequest) (ai.AnalysisResult, error)
	Extract(ctx context.Context, req ai.ExtractRequest) (ai.ExtractResult, error)
	Index(ctx context.Context, req ai.IndexRequest) (ai.IndexResult, error)
}

type documentStore interface {
	Put(ctx context.Context, key string, body io.Reader) (string, error)
}

type externalCopyClient interface {
	Enabled() bool
	CopyDocument(ctx context.Context, req externalcopy.CopyRequest) (externalcopy.CopyResult, error)
}

type noopAIClient struct{}

func (noopAIClient) AnalyzeClause(context.Context, ai.AnalyzeClauseRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (noopAIClient) AnalyzeCompanyName(context.Context, ai.AnalyzeCompanyNameRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (noopAIClient) Extract(_ context.Context, req ai.ExtractRequest) (ai.ExtractResult, error) {
	return ai.ExtractResult{
		MIMEType: req.MIMEType,
		Text:     "",
	}, nil
}
func (noopAIClient) Index(_ context.Context, req ai.IndexRequest) (ai.IndexResult, error) {
	return ai.IndexResult{
		DocumentID: req.DocumentID,
		Checksum:   req.VersionChecksum,
		Indexed:    true,
	}, nil
}

type noopDocumentStore struct{}

func (noopDocumentStore) Put(_ context.Context, key string, _ io.Reader) (string, error) {
	return "file:///" + key, nil
}

type noopExternalCopyClient struct{}

func (noopExternalCopyClient) Enabled() bool { return false }

func (noopExternalCopyClient) CopyDocument(context.Context, externalcopy.CopyRequest) (externalcopy.CopyResult, error) {
	return externalcopy.CopyResult{}, &externalcopy.CallError{Retriable: false, Cause: errors.New("external copy is disabled")}
}

type API struct {
	logger slogLogger
	ai     aiClient
	store  documentStore
	copier externalCopyClient

	mu          sync.RWMutex
	documents   map[string]document
	checks      map[string]checkRun
	idempotency map[string]idempotencyRecord
	copyEvents  map[string]externalCopyEvent
}

type slogLogger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type idempotencyRecord struct {
	PayloadHash string
	CheckID     string
}

type document struct {
	ID         string
	SourceType string
	SourceRef  string
	Filename   string
	MIMEType   string
	Status     string
	Checksum   string
	StorageURI string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type checkRun struct {
	CheckID       string
	Status        string
	CheckType     string
	RequestedAt   time.Time
	FinishedAt    *time.Time
	FailureReason string
	DocumentIDs   []string
	Items         []checkResultItem
}

type checkResultItem struct {
	DocumentID string            `json:"document_id"`
	Outcome    string            `json:"outcome"`
	Confidence float64           `json:"confidence"`
	Summary    string            `json:"summary,omitempty"`
	Evidence   []evidenceSnippet `json:"evidence,omitempty"`
}

type evidenceSnippet struct {
	SnippetText string  `json:"snippet_text"`
	PageNumber  int     `json:"page_number"`
	ChunkID     string  `json:"chunk_id,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

type externalCopyEvent struct {
	ID             string
	DocumentID     string
	TargetSystem   string
	Status         string
	RequestPayload map[string]any
	ResponseBody   map[string]any
	Attempts       int
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type createDocumentRequest struct {
	SourceType    string   `json:"source_type,omitempty"`
	SourceRef     string   `json:"source_ref,omitempty"`
	Filename      string   `json:"filename"`
	MIMEType      string   `json:"mime_type"`
	ContentBase64 string   `json:"content_base64"`
	Tags          []string `json:"tags,omitempty"`
}

type documentResponse struct {
	ID         string `json:"id"`
	SourceType string `json:"source_type,omitempty"`
	SourceRef  string `json:"source_ref,omitempty"`
	Filename   string `json:"filename"`
	MIMEType   string `json:"mime_type"`
	Status     string `json:"status"`
	Checksum   string `json:"checksum,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type documentListResponse struct {
	Items  []documentResponse `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
	Total  int                `json:"total"`
}

type clauseCheckRequest struct {
	DocumentIDs        []string `json:"document_ids,omitempty"`
	RequiredClauseText string   `json:"required_clause_text"`
	ContextHint        string   `json:"context_hint,omitempty"`
}

type companyNameCheckRequest struct {
	DocumentIDs    []string `json:"document_ids,omitempty"`
	OldCompanyName string   `json:"old_company_name"`
	NewCompanyName string   `json:"new_company_name,omitempty"`
}

type checkAcceptedResponse struct {
	CheckID   string `json:"check_id"`
	Status    string `json:"status"`
	CheckType string `json:"check_type"`
}

type checkRunResponse struct {
	CheckID       string  `json:"check_id"`
	Status        string  `json:"status"`
	CheckType     string  `json:"check_type"`
	RequestedAt   string  `json:"requested_at"`
	FinishedAt    *string `json:"finished_at,omitempty"`
	FailureReason string  `json:"failure_reason,omitempty"`
}

type checkResultsResponse struct {
	CheckID string            `json:"check_id"`
	Status  string            `json:"status"`
	Items   []checkResultItem `json:"items"`
}

type errorEnvelope struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Retriable bool           `json:"retriable"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewAPI(logger slogLogger, aiClient aiClient, store documentStore, copier externalCopyClient) *API {
	if aiClient == nil {
		aiClient = noopAIClient{}
	}
	if store == nil {
		store = noopDocumentStore{}
	}
	if copier == nil {
		copier = noopExternalCopyClient{}
	}

	return &API{
		logger:      logger,
		ai:          aiClient,
		store:       store,
		copier:      copier,
		documents:   map[string]document{},
		checks:      map[string]checkRun{},
		idempotency: map[string]idempotencyRecord{},
		copyEvents:  map[string]externalCopyEvent{},
	}
}

func (a *API) CreateDocument(w http.ResponseWriter, r *http.Request) {
	var req createDocumentRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.Filename) == "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", "filename is required", false, nil)
		return
	}
	if _, ok := validDocumentMimes[req.MIMEType]; !ok {
		writeError(w, http.StatusBadRequest, "invalid_argument", "unsupported mime_type", false, nil)
		return
	}
	payload, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "content_base64 must be valid base64", false, nil)
		return
	}

	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "upload"
	}
	if _, ok := validSourceTypes[sourceType]; !ok {
		writeError(w, http.StatusBadRequest, "invalid_argument", "unsupported source_type", false, nil)
		return
	}

	now := time.Now().UTC()
	docID := newUUID()
	checksum := sha256Hex(payload)
	objectKey := fmt.Sprintf("documents/%s%s", docID, extensionForFilename(req.Filename, req.MIMEType))
	storageURI, err := a.store.Put(r.Context(), objectKey, bytes.NewReader(payload))
	if err != nil {
		a.logger.Error("document storage failed", "document_id", docID, "error", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "failed to persist document", true, nil)
		return
	}

	doc := document{
		ID:         docID,
		SourceType: sourceType,
		SourceRef:  req.SourceRef,
		Filename:   req.Filename,
		MIMEType:   req.MIMEType,
		Status:     documentStatusProcessing,
		Checksum:   checksum,
		StorageURI: storageURI,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	a.mu.Lock()
	a.documents[docID] = doc
	a.mu.Unlock()
	a.emitAuditEvent("document.created", "document", docID, map[string]any{
		"source_type": sourceType,
		"mime_type":   req.MIMEType,
		"checksum":    checksum,
	})

	extractResult, err := a.ai.Extract(r.Context(), ai.ExtractRequest{
		JobID:      newUUID(),
		RequestID:  middleware.GetRequestID(r.Context()),
		DocumentID: docID,
		StorageURI: storageURI,
		MIMEType:   req.MIMEType,
	})
	if err != nil {
		a.markDocumentFailed(docID, err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "failed to extract document text", true, nil)
		return
	}

	pages := make([]ai.IndexPageInput, 0, len(extractResult.Pages))
	for _, page := range extractResult.Pages {
		pages = append(pages, ai.IndexPageInput{
			PageNumber: page.PageNumber,
			Text:       page.Text,
		})
	}

	if _, err := a.ai.Index(r.Context(), ai.IndexRequest{
		JobID:           newUUID(),
		RequestID:       middleware.GetRequestID(r.Context()),
		DocumentID:      docID,
		VersionChecksum: checksum,
		ExtractedText:   extractResult.Text,
		Pages:           pages,
		SourceURI:       storageURI,
		Reindex:         false,
	}); err != nil {
		a.markDocumentFailed(docID, err)
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "failed to index document text", true, nil)
		return
	}

	doc = a.markDocumentIndexed(docID)
	a.emitAuditEvent("document.indexed", "document", docID, map[string]any{"status": doc.Status})
	a.enqueueExternalCopy(doc, middleware.GetRequestID(r.Context()))
	writeJSON(w, http.StatusCreated, mapDocument(doc))
}

func (a *API) ListDocuments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" {
		if _, ok := validDocStatuses[status]; !ok {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid status filter", false, nil)
			return
		}
	}

	sourceType := strings.TrimSpace(q.Get("source_type"))
	if sourceType != "" {
		if _, ok := validSourceTypes[sourceType]; !ok {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid source_type filter", false, nil)
			return
		}
	}

	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "limit must be between 1 and 200", false, nil)
			return
		}
		limit = n
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "offset must be >= 0", false, nil)
			return
		}
		offset = n
	}

	a.mu.RLock()
	items := make([]document, 0, len(a.documents))
	for _, doc := range a.documents {
		if status != "" && doc.Status != status {
			continue
		}
		if sourceType != "" && doc.SourceType != sourceType {
			continue
		}
		items = append(items, doc)
	}
	a.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	respItems := make([]documentResponse, 0, end-offset)
	for _, doc := range items[offset:end] {
		respItems = append(respItems, mapDocument(doc))
	}

	writeJSON(w, http.StatusOK, documentListResponse{Items: respItems, Limit: limit, Offset: offset, Total: total})
}

func (a *API) GetDocument(w http.ResponseWriter, r *http.Request) {
	documentID := strings.TrimSpace(r.PathValue("document_id"))
	if !isUUID(documentID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "document_id must be a valid UUID", false, nil)
		return
	}

	a.mu.RLock()
	doc, ok := a.documents[documentID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "document not found", false, nil)
		return
	}

	writeJSON(w, http.StatusOK, mapDocument(doc))
}

func (a *API) CreateClauseCheck(w http.ResponseWriter, r *http.Request) {
	var req clauseCheckRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if len(strings.TrimSpace(req.RequiredClauseText)) < 5 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "required_clause_text must be at least 5 characters", false, nil)
		return
	}

	checkID, status, reused, err := a.createCheck(r, checkTypeClause, req, req.DocumentIDs)
	if err != nil {
		handleCreateCheckError(w, err)
		return
	}
	if reused {
		a.logger.Info("idempotent check request reused", "check_id", checkID, "check_type", checkTypeClause)
		writeJSON(w, http.StatusAccepted, checkAcceptedResponse{CheckID: checkID, Status: status, CheckType: checkTypeClause})
		return
	}

	go a.runClauseCheck(checkID, req, middleware.GetRequestID(r.Context()))
	writeJSON(w, http.StatusAccepted, checkAcceptedResponse{CheckID: checkID, Status: status, CheckType: checkTypeClause})
}

func (a *API) CreateCompanyNameCheck(w http.ResponseWriter, r *http.Request) {
	var req companyNameCheckRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if len(strings.TrimSpace(req.OldCompanyName)) < 2 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "old_company_name must be at least 2 characters", false, nil)
		return
	}

	checkID, status, reused, err := a.createCheck(r, checkTypeCompany, req, req.DocumentIDs)
	if err != nil {
		handleCreateCheckError(w, err)
		return
	}
	if reused {
		a.logger.Info("idempotent check request reused", "check_id", checkID, "check_type", checkTypeCompany)
		writeJSON(w, http.StatusAccepted, checkAcceptedResponse{CheckID: checkID, Status: status, CheckType: checkTypeCompany})
		return
	}

	go a.runCompanyNameCheck(checkID, req, middleware.GetRequestID(r.Context()))
	writeJSON(w, http.StatusAccepted, checkAcceptedResponse{CheckID: checkID, Status: status, CheckType: checkTypeCompany})
}

func (a *API) GetCheck(w http.ResponseWriter, r *http.Request) {
	checkID := strings.TrimSpace(r.PathValue("check_id"))
	if !isUUID(checkID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "check_id must be a valid UUID", false, nil)
		return
	}

	a.mu.RLock()
	check, ok := a.checks[checkID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "check not found", false, nil)
		return
	}

	resp := checkRunResponse{
		CheckID:     check.CheckID,
		Status:      check.Status,
		CheckType:   check.CheckType,
		RequestedAt: check.RequestedAt.Format(time.RFC3339),
	}
	if check.FinishedAt != nil {
		v := check.FinishedAt.Format(time.RFC3339)
		resp.FinishedAt = &v
	}
	if check.FailureReason != "" {
		resp.FailureReason = check.FailureReason
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *API) GetCheckResults(w http.ResponseWriter, r *http.Request) {
	checkID := strings.TrimSpace(r.PathValue("check_id"))
	if !isUUID(checkID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "check_id must be a valid UUID", false, nil)
		return
	}

	a.mu.RLock()
	check, ok := a.checks[checkID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "check not found", false, nil)
		return
	}
	if check.Status != checkStatusCompleted {
		writeError(w, http.StatusConflict, "results_not_ready", "results are not available for this check status", false, map[string]any{"status": check.Status})
		return
	}

	writeJSON(w, http.StatusOK, checkResultsResponse{CheckID: check.CheckID, Status: check.Status, Items: check.Items})
}

var errIdempotencyConflict = errors.New("idempotency conflict")

func (a *API) createCheck(r *http.Request, checkType string, payload any, documentIDs []string) (checkID string, status string, reused bool, err error) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" && (len(idempotencyKey) < 8 || len(idempotencyKey) > 128) {
		return "", "", false, fmt.Errorf("invalid idempotency key")
	}

	resolvedDocIDs, err := a.resolveDocumentIDs(documentIDs)
	if err != nil {
		return "", "", false, err
	}

	payloadHash, err := hashPayload(payload, resolvedDocIDs)
	if err != nil {
		return "", "", false, fmt.Errorf("hash payload: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if idempotencyKey != "" {
		idempotencyLookupKey := checkType + ":" + idempotencyKey
		if rec, exists := a.idempotency[idempotencyLookupKey]; exists {
			if rec.PayloadHash != payloadHash {
				return "", "", false, errIdempotencyConflict
			}
			run := a.checks[rec.CheckID]
			return run.CheckID, run.Status, true, nil
		}
	}

	checkID = newUUID()
	now := time.Now().UTC()
	status = checkStatusQueued
	a.checks[checkID] = checkRun{
		CheckID:     checkID,
		Status:      status,
		CheckType:   checkType,
		RequestedAt: now,
		DocumentIDs: resolvedDocIDs,
	}

	if idempotencyKey != "" {
		a.idempotency[checkType+":"+idempotencyKey] = idempotencyRecord{PayloadHash: payloadHash, CheckID: checkID}
	}
	a.emitAuditEvent("check.created", "check", checkID, map[string]any{
		"check_type":     checkType,
		"document_count": len(resolvedDocIDs),
	})

	return checkID, status, false, nil
}

func (a *API) resolveDocumentIDs(explicit []string) ([]string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ids := explicit
	if len(ids) == 0 {
		ids = make([]string, 0, len(a.documents))
		for id := range a.documents {
			ids = append(ids, id)
		}
		sort.Strings(ids)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one document is required")
	}

	seen := make(map[string]struct{}, len(ids))
	resolved := make([]string, 0, len(ids))
	for _, id := range ids {
		if !isUUID(id) {
			return nil, fmt.Errorf("document_id must be a valid UUID: %s", id)
		}
		if _, ok := a.documents[id]; !ok {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		resolved = append(resolved, id)
	}
	sort.Strings(resolved)
	return resolved, nil
}

func (a *API) runClauseCheck(checkID string, req clauseCheckRequest, requestID string) {
	if !a.markCheckRunning(checkID) {
		return
	}

	a.mu.RLock()
	run := a.checks[checkID]
	a.mu.RUnlock()

	result, err := a.ai.AnalyzeClause(context.Background(), ai.AnalyzeClauseRequest{
		JobID:              newUUID(),
		RequestID:          requestID,
		CheckID:            checkID,
		DocumentIDs:        run.DocumentIDs,
		RequiredClauseText: req.RequiredClauseText,
		ContextHint:        req.ContextHint,
	})
	if err != nil {
		a.markCheckFailed(checkID, err)
		return
	}

	items := mapAnalysisItems(run.DocumentIDs, result.Items, "Clause analysis returned no items; manual review is required.")
	a.markCheckCompleted(checkID, items)
}

func (a *API) runCompanyNameCheck(checkID string, req companyNameCheckRequest, requestID string) {
	if !a.markCheckRunning(checkID) {
		return
	}

	a.mu.RLock()
	run := a.checks[checkID]
	a.mu.RUnlock()

	result, err := a.ai.AnalyzeCompanyName(context.Background(), ai.AnalyzeCompanyNameRequest{
		JobID:          newUUID(),
		RequestID:      requestID,
		CheckID:        checkID,
		DocumentIDs:    run.DocumentIDs,
		OldCompanyName: req.OldCompanyName,
		NewCompanyName: req.NewCompanyName,
	})
	if err != nil {
		a.markCheckFailed(checkID, err)
		return
	}

	items := mapAnalysisItems(run.DocumentIDs, result.Items, "Company-name analysis returned no items; manual review is required.")
	a.markCheckCompleted(checkID, items)
}

func mapAnalysisItems(documentIDs []string, analysisItems []ai.AnalysisResultItem, fallbackSummary string) []checkResultItem {
	byDocument := make(map[string]ai.AnalysisResultItem, len(analysisItems))
	for _, item := range analysisItems {
		if item.DocumentID == "" {
			continue
		}
		byDocument[item.DocumentID] = item
	}

	items := make([]checkResultItem, 0, len(documentIDs))
	for _, documentID := range documentIDs {
		analysisItem, ok := byDocument[documentID]
		if !ok {
			items = append(items, checkResultItem{
				DocumentID: documentID,
				Outcome:    "review",
				Confidence: 0.35,
				Summary:    fallbackSummary,
			})
			continue
		}

		evidence := make([]evidenceSnippet, 0, len(analysisItem.Evidence))
		for _, snippet := range analysisItem.Evidence {
			evidence = append(evidence, evidenceSnippet{
				SnippetText: snippet.SnippetText,
				PageNumber:  snippet.PageNumber,
				ChunkID:     snippet.ChunkID,
				Score:       snippet.Score,
			})
		}

		items = append(items, checkResultItem{
			DocumentID: documentID,
			Outcome:    analysisItem.Outcome,
			Confidence: analysisItem.Confidence,
			Summary:    analysisItem.Summary,
			Evidence:   evidence,
		})
	}

	return items
}

func (a *API) markCheckRunning(checkID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	run, ok := a.checks[checkID]
	if !ok {
		return false
	}
	run.Status = checkStatusRunning
	a.checks[checkID] = run
	return true
}

func (a *API) markCheckCompleted(checkID string, items []checkResultItem) {
	now := time.Now().UTC()

	a.mu.Lock()
	defer a.mu.Unlock()

	run := a.checks[checkID]
	run.Status = checkStatusCompleted
	run.FinishedAt = &now
	run.Items = items
	a.checks[checkID] = run
	a.emitAuditEvent("check.completed", "check", checkID, map[string]any{"item_count": len(items)})
}

func (a *API) markCheckFailed(checkID string, err error) {
	now := time.Now().UTC()

	a.mu.Lock()
	defer a.mu.Unlock()

	run := a.checks[checkID]
	run.Status = checkStatusFailed
	run.FinishedAt = &now
	run.FailureReason = err.Error()
	a.checks[checkID] = run
	a.logger.Error("check execution failed", "check_id", checkID, "error", err)
	a.emitAuditEvent("check.failed", "check", checkID, map[string]any{"error": err.Error()})
}

func (a *API) markDocumentFailed(documentID string, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	doc := a.documents[documentID]
	doc.Status = documentStatusFailed
	doc.UpdatedAt = time.Now().UTC()
	a.documents[documentID] = doc
	a.logger.Error("document processing failed", "document_id", documentID, "error", err)
	a.emitAuditEvent("document.failed", "document", documentID, map[string]any{"error": err.Error()})
}

func (a *API) markDocumentIndexed(documentID string) document {
	a.mu.Lock()
	defer a.mu.Unlock()

	doc := a.documents[documentID]
	doc.Status = documentStatusIndexed
	doc.UpdatedAt = time.Now().UTC()
	a.documents[documentID] = doc
	return doc
}

func (a *API) enqueueExternalCopy(doc document, requestID string) {
	if !a.copier.Enabled() {
		return
	}

	now := time.Now().UTC()
	eventID := newUUID()
	payload := map[string]any{
		"request_id":  requestID,
		"document_id": doc.ID,
		"filename":    doc.Filename,
		"mime_type":   doc.MIMEType,
		"checksum":    doc.Checksum,
		"storage_uri": doc.StorageURI,
	}

	a.mu.Lock()
	a.copyEvents[eventID] = externalCopyEvent{
		ID:             eventID,
		DocumentID:     doc.ID,
		TargetSystem:   "external_copy_api",
		Status:         "queued",
		RequestPayload: payload,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	a.mu.Unlock()

	a.emitAuditEvent("external_copy.queued", "document", doc.ID, map[string]any{"event_id": eventID})
	go a.runExternalCopy(eventID, doc, requestID)
}

func (a *API) runExternalCopy(eventID string, doc document, requestID string) {
	result, err := a.copier.CopyDocument(context.Background(), externalcopy.CopyRequest{
		RequestID:  requestID,
		DocumentID: doc.ID,
		Filename:   doc.Filename,
		MIMEType:   doc.MIMEType,
		Checksum:   doc.Checksum,
		StorageURI: doc.StorageURI,
		CreatedAt:  doc.CreatedAt.Format(time.RFC3339),
	})

	a.mu.Lock()
	event := a.copyEvents[eventID]
	event.UpdatedAt = time.Now().UTC()
	if err != nil {
		event.Status = "failed"
		event.ErrorMessage = err.Error()
		var callErr *externalcopy.CallError
		if errors.As(err, &callErr) {
			event.Attempts = callErr.Attempts
		}
		a.copyEvents[eventID] = event
		a.mu.Unlock()
		a.emitAuditEvent("external_copy.failed", "document", doc.ID, map[string]any{
			"event_id": eventID,
			"error":    event.ErrorMessage,
			"attempts": event.Attempts,
		})
		a.logger.Error("external copy failed", "document_id", doc.ID, "event_id", eventID, "error", err)
		return
	}

	event.Status = "succeeded"
	event.Attempts = result.Attempts
	event.ResponseBody = result.Body
	a.copyEvents[eventID] = event
	a.mu.Unlock()

	a.emitAuditEvent("external_copy.succeeded", "document", doc.ID, map[string]any{
		"event_id": eventID,
		"attempts": result.Attempts,
	})
}

func (a *API) emitAuditEvent(eventType, entityType, entityID string, payload map[string]any) {
	a.logger.Info("audit event", "event_type", eventType, "entity_type", entityType, "entity_id", entityID, "payload", payload)
}

func handleCreateCheckError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key is already used with a different payload", false, nil)
	case strings.Contains(err.Error(), "document not found"):
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
	case strings.Contains(err.Error(), "document_id must be"):
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
	case strings.Contains(err.Error(), "at least one document"):
		writeError(w, http.StatusUnprocessableEntity, "invalid_scope", err.Error(), false, nil)
	case strings.Contains(err.Error(), "idempotency"):
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create check", true, nil)
	}
}

func mapDocument(doc document) documentResponse {
	return documentResponse{
		ID:         doc.ID,
		SourceType: doc.SourceType,
		SourceRef:  doc.SourceRef,
		Filename:   doc.Filename,
		MIMEType:   doc.MIMEType,
		Status:     doc.Status,
		Checksum:   doc.Checksum,
		CreatedAt:  doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  doc.UpdatedAt.Format(time.RFC3339),
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "invalid JSON payload", false, map[string]any{"error": err.Error()})
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string, retriable bool, details map[string]any) {
	writeJSON(w, status, errorEnvelope{Error: errorPayload{Code: code, Message: message, Retriable: retriable, Details: details}})
}

func hashPayload(payload any, documentIDs []string) (string, error) {
	blob := struct {
		Payload     any      `json:"payload"`
		DocumentIDs []string `json:"document_ids"`
	}{Payload: payload, DocumentIDs: documentIDs}

	data, err := json.Marshal(blob)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func isUUID(v string) bool {
	return uuidRx.MatchString(strings.ToLower(v))
}

func newUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

func extensionForFilename(filename, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if ext == ".pdf" || ext == ".jpg" || ext == ".jpeg" {
		return ext
	}

	if mimeType == "application/pdf" {
		return ".pdf"
	}

	return ".jpg"
}
