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
	validDocumentMimes = map[string]struct{}{"application/pdf": {}, "image/jpeg": {}, "image/png": {}}
	validSourceTypes   = map[string]struct{}{"repository": {}, "upload": {}, "api": {}}
	validDocStatuses   = map[string]struct{}{documentStatusIngested: {}, documentStatusProcessing: {}, documentStatusIndexed: {}, documentStatusFailed: {}}
)

type aiClient interface {
	AnalyzeClause(ctx context.Context, req ai.AnalyzeClauseRequest) (ai.AnalysisResult, error)
	AnalyzeCompanyName(ctx context.Context, req ai.AnalyzeCompanyNameRequest) (ai.AnalysisResult, error)
	Extract(ctx context.Context, req ai.ExtractRequest) (ai.ExtractResult, error)
	Index(ctx context.Context, req ai.IndexRequest) (ai.IndexResult, error)
	SearchSections(ctx context.Context, req ai.SearchSectionsRequest) (ai.SearchSectionsResult, error)
}

type documentStore interface {
	Put(ctx context.Context, key string, body io.Reader) (string, error)
	Delete(ctx context.Context, key string) error
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
func (noopAIClient) SearchSections(_ context.Context, _ ai.SearchSectionsRequest) (ai.SearchSectionsResult, error) {
	return ai.SearchSectionsResult{Items: []ai.SearchSectionsResultItem{}}, nil
}

type noopDocumentStore struct{}

func (noopDocumentStore) Put(_ context.Context, key string, _ io.Reader) (string, error) {
	return "file:///" + key, nil
}

func (noopDocumentStore) Delete(_ context.Context, _ string) error {
	return nil
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
	contracts   map[string]contract
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
	ID            string
	ContractID    string
	SourceType    string
	SourceRef     string
	Tags          []string
	Filename      string
	MIMEType      string
	Status        string
	Checksum      string
	ExtractedText string
	StorageKey    string
	StorageURI    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type contract struct {
	ID         string
	Name       string
	SourceType string
	SourceRef  string
	Tags       []string
	FileIDs    []string
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
	ContractID    string   `json:"contract_id,omitempty"`
	SourceType    string   `json:"source_type,omitempty"`
	SourceRef     string   `json:"source_ref,omitempty"`
	Filename      string   `json:"filename"`
	MIMEType      string   `json:"mime_type"`
	ContentBase64 string   `json:"content_base64"`
	Tags          []string `json:"tags,omitempty"`
}

type documentResponse struct {
	ID         string   `json:"id"`
	ContractID string   `json:"contract_id,omitempty"`
	SourceType string   `json:"source_type,omitempty"`
	SourceRef  string   `json:"source_ref,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Filename   string   `json:"filename"`
	MIMEType   string   `json:"mime_type"`
	Status     string   `json:"status"`
	Checksum   string   `json:"checksum,omitempty"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

type documentListResponse struct {
	Items  []documentResponse `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
	Total  int                `json:"total"`
}

type documentTextResponse struct {
	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	Text       string `json:"text"`
	HasText    bool   `json:"has_text"`
}

type createContractRequest struct {
	Name       string   `json:"name"`
	SourceType string   `json:"source_type,omitempty"`
	SourceRef  string   `json:"source_ref,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

type updateContractRequest struct {
	Name *string   `json:"name,omitempty"`
	Tags *[]string `json:"tags,omitempty"`
}

type reorderContractFilesRequest struct {
	FileIDs []string `json:"file_ids"`
}

type contractResponse struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	SourceType string             `json:"source_type,omitempty"`
	SourceRef  string             `json:"source_ref,omitempty"`
	Tags       []string           `json:"tags,omitempty"`
	FileCount  int                `json:"file_count"`
	Files      []documentResponse `json:"files,omitempty"`
	CreatedAt  string             `json:"created_at"`
	UpdatedAt  string             `json:"updated_at"`
}

type contractListResponse struct {
	Items  []contractResponse `json:"items"`
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

type contractSearchRequest struct {
	QueryText   string   `json:"query_text"`
	DocumentIDs []string `json:"document_ids,omitempty"`
	Limit       int      `json:"limit,omitempty"`
}

type contractSearchResultItem struct {
	DocumentID  string  `json:"document_id"`
	ContractID  string  `json:"contract_id,omitempty"`
	Filename    string  `json:"filename"`
	PageNumber  int     `json:"page_number"`
	ChunkID     string  `json:"chunk_id,omitempty"`
	Score       float64 `json:"score"`
	SnippetText string  `json:"snippet_text"`
}

type contractSearchResponse struct {
	Items []contractSearchResultItem `json:"items"`
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
		contracts:   map[string]contract{},
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

	doc, err := a.createDocumentFromRequest(r.Context(), req, middleware.GetRequestID(r.Context()))
	if err != nil {
		switch err.Error() {
		case "filename is required", "unsupported mime_type", "content_base64 must be valid base64", "unsupported source_type":
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
			return
		case "contract_id must be a valid UUID":
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
			return
		case "contract not found":
			writeError(w, http.StatusNotFound, "not_found", err.Error(), false, nil)
			return
		case "failed to persist document":
			writeError(w, http.StatusBadGateway, "storage_unavailable", err.Error(), true, nil)
			return
		case "failed to extract document text", "failed to index document text":
			writeError(w, http.StatusBadGateway, "upstream_unavailable", err.Error(), true, nil)
			return
		default:
			if strings.HasPrefix(err.Error(), "tag ") || strings.HasPrefix(err.Error(), "at most ") {
				writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create document", true, nil)
			return
		}
	}

	writeJSON(w, http.StatusCreated, mapDocument(doc))
}

func (a *API) CreateContract(w http.ResponseWriter, r *http.Request) {
	var req createContractRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_argument", "name is required", false, nil)
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

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
		return
	}

	now := time.Now().UTC()
	item := contract{
		ID:         newUUID(),
		Name:       name,
		SourceType: sourceType,
		SourceRef:  req.SourceRef,
		Tags:       tags,
		FileIDs:    nil,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	a.mu.Lock()
	a.contracts[item.ID] = item
	a.mu.Unlock()

	a.emitAuditEvent("contract.created", "contract", item.ID, map[string]any{
		"name":        item.Name,
		"source_type": item.SourceType,
	})
	writeJSON(w, http.StatusCreated, mapContract(item, nil))
}

func (a *API) ListContracts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
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
	items := make([]contract, 0, len(a.contracts))
	for _, item := range a.contracts {
		items = append(items, item)
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

	respItems := make([]contractResponse, 0, end-offset)
	for _, item := range items[offset:end] {
		respItems = append(respItems, mapContract(item, nil))
	}

	writeJSON(w, http.StatusOK, contractListResponse{Items: respItems, Limit: limit, Offset: offset, Total: total})
}

func (a *API) GetContract(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimSpace(r.PathValue("contract_id"))
	if !isUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	a.mu.RLock()
	item, ok := a.contracts[contractID]
	if !ok {
		a.mu.RUnlock()
		writeError(w, http.StatusNotFound, "not_found", "contract not found", false, nil)
		return
	}
	files := make([]documentResponse, 0, len(item.FileIDs))
	for _, fileID := range item.FileIDs {
		if doc, exists := a.documents[fileID]; exists {
			files = append(files, mapDocument(doc))
		}
	}
	a.mu.RUnlock()

	writeJSON(w, http.StatusOK, mapContract(item, files))
}

func (a *API) UpdateContract(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimSpace(r.PathValue("contract_id"))
	if !isUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	var req updateContractRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == nil && req.Tags == nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", "at least one of name or tags is required", false, nil)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	item, ok := a.contracts[contractID]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "contract not found", false, nil)
		return
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "invalid_argument", "name is required", false, nil)
			return
		}
		item.Name = name
	}

	if req.Tags != nil {
		tags, err := normalizeTags(*req.Tags)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
			return
		}
		item.Tags = tags
	}

	item.UpdatedAt = time.Now().UTC()
	a.contracts[contractID] = item
	a.emitAuditEvent("contract.updated", "contract", contractID, map[string]any{
		"name": item.Name,
		"tags": item.Tags,
	})

	files := make([]documentResponse, 0, len(item.FileIDs))
	for _, fileID := range item.FileIDs {
		if doc, exists := a.documents[fileID]; exists {
			files = append(files, mapDocument(doc))
		}
	}
	writeJSON(w, http.StatusOK, mapContract(item, files))
}

func (a *API) DeleteContract(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimSpace(r.PathValue("contract_id"))
	if !isUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	a.mu.RLock()
	item, ok := a.contracts[contractID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "contract not found", false, nil)
		return
	}

	for _, fileID := range item.FileIDs {
		a.mu.RLock()
		doc, exists := a.documents[fileID]
		a.mu.RUnlock()
		if !exists {
			continue
		}
		if err := a.store.Delete(r.Context(), doc.StorageKey); err != nil {
			a.logger.Error("document storage delete failed", "document_id", fileID, "error", err)
			writeError(w, http.StatusBadGateway, "storage_unavailable", "failed to delete document asset", true, nil)
			return
		}
	}

	a.mu.Lock()
	delete(a.contracts, contractID)
	for _, fileID := range item.FileIDs {
		delete(a.documents, fileID)
	}
	a.mu.Unlock()
	a.emitAuditEvent("contract.deleted", "contract", contractID, map[string]any{"file_count": len(item.FileIDs)})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) AddContractFile(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimSpace(r.PathValue("contract_id"))
	if !isUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	var req createDocumentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ContractID = contractID
	doc, err := a.createDocumentFromRequest(r.Context(), req, middleware.GetRequestID(r.Context()))
	if err != nil {
		switch err.Error() {
		case "filename is required", "unsupported mime_type", "content_base64 must be valid base64", "unsupported source_type":
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
			return
		case "contract_id must be a valid UUID":
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
			return
		case "contract not found":
			writeError(w, http.StatusNotFound, "not_found", err.Error(), false, nil)
			return
		case "failed to persist document":
			writeError(w, http.StatusBadGateway, "storage_unavailable", err.Error(), true, nil)
			return
		case "failed to extract document text", "failed to index document text":
			writeError(w, http.StatusBadGateway, "upstream_unavailable", err.Error(), true, nil)
			return
		default:
			if strings.HasPrefix(err.Error(), "tag ") || strings.HasPrefix(err.Error(), "at most ") {
				writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to create document", true, nil)
			return
		}
	}

	writeJSON(w, http.StatusCreated, mapDocument(doc))
}

func (a *API) ReorderContractFiles(w http.ResponseWriter, r *http.Request) {
	contractID := strings.TrimSpace(r.PathValue("contract_id"))
	if !isUUID(contractID) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "contract_id must be a valid UUID", false, nil)
		return
	}

	var req reorderContractFilesRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	item, ok := a.contracts[contractID]
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "contract not found", false, nil)
		return
	}

	if len(req.FileIDs) != len(item.FileIDs) {
		writeError(w, http.StatusBadRequest, "invalid_argument", "file_ids must contain all contract file ids exactly once", false, nil)
		return
	}

	expected := make(map[string]struct{}, len(item.FileIDs))
	for _, id := range item.FileIDs {
		expected[id] = struct{}{}
	}
	seen := make(map[string]struct{}, len(req.FileIDs))
	for _, id := range req.FileIDs {
		if _, ok := expected[id]; !ok {
			writeError(w, http.StatusBadRequest, "invalid_argument", "file_ids contains an unknown file id", false, nil)
			return
		}
		if _, ok := seen[id]; ok {
			writeError(w, http.StatusBadRequest, "invalid_argument", "file_ids must not contain duplicates", false, nil)
			return
		}
		seen[id] = struct{}{}
	}

	item.FileIDs = append([]string{}, req.FileIDs...)
	item.UpdatedAt = time.Now().UTC()
	a.contracts[contractID] = item
	a.emitAuditEvent("contract.files_reordered", "contract", contractID, map[string]any{"file_count": len(item.FileIDs)})

	files := make([]documentResponse, 0, len(item.FileIDs))
	for _, fileID := range item.FileIDs {
		if doc, exists := a.documents[fileID]; exists {
			files = append(files, mapDocument(doc))
		}
	}
	writeJSON(w, http.StatusOK, mapContract(item, files))
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
	tagFilters, err := parseTagFilters(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_argument", err.Error(), false, nil)
		return
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
		if len(tagFilters) > 0 && !documentHasAnyTag(doc, tagFilters) {
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

func (a *API) DeleteDocument(w http.ResponseWriter, r *http.Request) {
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

	if err := a.store.Delete(r.Context(), doc.StorageKey); err != nil {
		a.logger.Error("document storage delete failed", "document_id", documentID, "error", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "failed to delete document asset", true, nil)
		return
	}

	a.mu.Lock()
	if _, exists := a.documents[documentID]; !exists {
		a.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	delete(a.documents, documentID)
	if doc.ContractID != "" {
		if item, exists := a.contracts[doc.ContractID]; exists {
			filtered := make([]string, 0, len(item.FileIDs))
			for _, id := range item.FileIDs {
				if id != documentID {
					filtered = append(filtered, id)
				}
			}
			item.FileIDs = filtered
			item.UpdatedAt = time.Now().UTC()
			a.contracts[doc.ContractID] = item
		}
	}

	deletedChecks := make(map[string]struct{})
	for checkID, run := range a.checks {
		if containsString(run.DocumentIDs, documentID) {
			delete(a.checks, checkID)
			deletedChecks[checkID] = struct{}{}
		}
	}

	for key, rec := range a.idempotency {
		if _, ok := deletedChecks[rec.CheckID]; ok {
			delete(a.idempotency, key)
		}
	}

	deletedCopyEvents := 0
	for eventID, event := range a.copyEvents {
		if event.DocumentID == documentID {
			delete(a.copyEvents, eventID)
			deletedCopyEvents++
		}
	}
	a.mu.Unlock()

	a.emitAuditEvent("document.deleted", "document", documentID, map[string]any{
		"checks_deleted":      len(deletedChecks),
		"copy_events_deleted": deletedCopyEvents,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) GetDocumentText(w http.ResponseWriter, r *http.Request) {
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

	text := strings.TrimSpace(doc.ExtractedText)
	writeJSON(w, http.StatusOK, documentTextResponse{
		DocumentID: doc.ID,
		Filename:   doc.Filename,
		Text:       text,
		HasText:    text != "",
	})
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

func (a *API) SearchContracts(w http.ResponseWriter, r *http.Request) {
	var req contractSearchRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	queryText := strings.TrimSpace(req.QueryText)
	if len(queryText) < 2 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "query_text must be at least 2 characters", false, nil)
		return
	}

	limit := req.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 0 || limit > 50 {
		writeError(w, http.StatusBadRequest, "invalid_argument", "limit must be between 1 and 50", false, nil)
		return
	}

	var resolvedDocIDs []string
	var err error
	if len(req.DocumentIDs) > 0 {
		resolvedDocIDs, err = a.resolveDocumentIDs(req.DocumentIDs)
		if err != nil {
			handleCreateCheckError(w, err)
			return
		}
	} else {
		a.mu.RLock()
		resolvedDocIDs = make([]string, 0, len(a.documents))
		for id := range a.documents {
			resolvedDocIDs = append(resolvedDocIDs, id)
		}
		a.mu.RUnlock()
		sort.Strings(resolvedDocIDs)
	}

	if len(resolvedDocIDs) == 0 {
		writeJSON(w, http.StatusOK, contractSearchResponse{Items: []contractSearchResultItem{}})
		return
	}

	result, err := a.ai.SearchSections(context.Background(), ai.SearchSectionsRequest{
		JobID:       newUUID(),
		RequestID:   middleware.GetRequestID(r.Context()),
		QueryText:   queryText,
		DocumentIDs: resolvedDocIDs,
		Limit:       limit,
	})
	if err != nil {
		a.logger.Error("contract search failed", "error", err)
		writeError(w, http.StatusBadGateway, "search_unavailable", "semantic search is temporarily unavailable", true, nil)
		return
	}

	a.mu.RLock()
	items := make([]contractSearchResultItem, 0, len(result.Items))
	for _, item := range result.Items {
		doc, ok := a.documents[item.DocumentID]
		if !ok {
			continue
		}
		items = append(items, contractSearchResultItem{
			DocumentID:  item.DocumentID,
			ContractID:  doc.ContractID,
			Filename:    doc.Filename,
			PageNumber:  item.PageNumber,
			ChunkID:     item.ChunkID,
			Score:       item.Score,
			SnippetText: item.SnippetText,
		})
	}
	a.mu.RUnlock()

	writeJSON(w, http.StatusOK, contractSearchResponse{Items: items})
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

func (a *API) createDocumentFromRequest(ctx context.Context, req createDocumentRequest, requestID string) (document, error) {
	if strings.TrimSpace(req.Filename) == "" {
		return document{}, errors.New("filename is required")
	}
	if _, ok := validDocumentMimes[req.MIMEType]; !ok {
		return document{}, errors.New("unsupported mime_type")
	}
	payload, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		return document{}, errors.New("content_base64 must be valid base64")
	}

	sourceType := req.SourceType
	if sourceType == "" {
		sourceType = "upload"
	}
	if _, ok := validSourceTypes[sourceType]; !ok {
		return document{}, errors.New("unsupported source_type")
	}

	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return document{}, err
	}

	contractID := strings.TrimSpace(req.ContractID)
	if contractID != "" {
		if !isUUID(contractID) {
			return document{}, errors.New("contract_id must be a valid UUID")
		}
		a.mu.RLock()
		_, exists := a.contracts[contractID]
		a.mu.RUnlock()
		if !exists {
			return document{}, errors.New("contract not found")
		}
	}

	now := time.Now().UTC()
	docID := newUUID()
	checksum := sha256Hex(payload)
	objectKey := fmt.Sprintf("documents/%s%s", docID, extensionForFilename(req.Filename, req.MIMEType))
	storageURI, err := a.store.Put(ctx, objectKey, bytes.NewReader(payload))
	if err != nil {
		a.logger.Error("document storage failed", "document_id", docID, "error", err)
		return document{}, errors.New("failed to persist document")
	}

	doc := document{
		ID:         docID,
		ContractID: contractID,
		SourceType: sourceType,
		SourceRef:  req.SourceRef,
		Tags:       tags,
		Filename:   req.Filename,
		MIMEType:   req.MIMEType,
		Status:     documentStatusProcessing,
		Checksum:   checksum,
		StorageKey: objectKey,
		StorageURI: storageURI,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	a.mu.Lock()
	a.documents[docID] = doc
	if contractID != "" {
		item := a.contracts[contractID]
		item.FileIDs = append(item.FileIDs, docID)
		item.UpdatedAt = now
		a.contracts[contractID] = item
	}
	a.mu.Unlock()

	a.emitAuditEvent("document.created", "document", docID, map[string]any{
		"source_type": sourceType,
		"mime_type":   req.MIMEType,
		"checksum":    checksum,
		"tags":        tags,
		"contract_id": contractID,
	})

	extractResult, err := a.ai.Extract(ctx, ai.ExtractRequest{
		JobID:      newUUID(),
		RequestID:  requestID,
		DocumentID: docID,
		StorageURI: storageURI,
		MIMEType:   req.MIMEType,
	})
	if err != nil {
		a.markDocumentFailed(docID, err)
		return document{}, errors.New("failed to extract document text")
	}

	pages := make([]ai.IndexPageInput, 0, len(extractResult.Pages))
	for _, page := range extractResult.Pages {
		pages = append(pages, ai.IndexPageInput{
			PageNumber: page.PageNumber,
			Text:       page.Text,
		})
	}

	if _, err := a.ai.Index(ctx, ai.IndexRequest{
		JobID:           newUUID(),
		RequestID:       requestID,
		DocumentID:      docID,
		VersionChecksum: checksum,
		ExtractedText:   extractResult.Text,
		Pages:           pages,
		SourceURI:       storageURI,
		Reindex:         false,
	}); err != nil {
		a.markDocumentFailed(docID, err)
		return document{}, errors.New("failed to index document text")
	}

	doc.ExtractedText = combineExtractedText(extractResult)
	doc.UpdatedAt = time.Now().UTC()
	a.mu.Lock()
	a.documents[docID] = doc
	a.mu.Unlock()

	doc = a.markDocumentIndexed(docID)
	a.emitAuditEvent("document.indexed", "document", docID, map[string]any{"status": doc.Status})
	a.enqueueExternalCopy(doc, requestID)
	return doc, nil
}

func mapDocument(doc document) documentResponse {
	return documentResponse{
		ID:         doc.ID,
		ContractID: doc.ContractID,
		SourceType: doc.SourceType,
		SourceRef:  doc.SourceRef,
		Tags:       doc.Tags,
		Filename:   doc.Filename,
		MIMEType:   doc.MIMEType,
		Status:     doc.Status,
		Checksum:   doc.Checksum,
		CreatedAt:  doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  doc.UpdatedAt.Format(time.RFC3339),
	}
}

func mapContract(item contract, files []documentResponse) contractResponse {
	return contractResponse{
		ID:         item.ID,
		Name:       item.Name,
		SourceType: item.SourceType,
		SourceRef:  item.SourceRef,
		Tags:       item.Tags,
		FileCount:  len(item.FileIDs),
		Files:      files,
		CreatedAt:  item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.Format(time.RFC3339),
	}
}

func combineExtractedText(result ai.ExtractResult) string {
	if strings.TrimSpace(result.Text) != "" {
		return strings.TrimSpace(result.Text)
	}
	if len(result.Pages) == 0 {
		return ""
	}
	var builder strings.Builder
	for i, page := range result.Pages {
		content := strings.TrimSpace(page.Text)
		if content == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(content)
		if i < len(result.Pages)-1 {
			builder.WriteString("\n")
		}
	}
	return strings.TrimSpace(builder.String())
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
	if ext == ".pdf" || ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
		return ext
	}

	if mimeType == "application/pdf" {
		return ".pdf"
	}
	if mimeType == "image/png" {
		return ".png"
	}

	return ".jpg"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func normalizeTags(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}

	const maxTags = 20
	const maxTagLength = 50

	tags := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if len(tag) > maxTagLength {
			return nil, fmt.Errorf("tag must be at most %d characters", maxTagLength)
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, tag)
		if len(tags) > maxTags {
			return nil, fmt.Errorf("at most %d tags are allowed", maxTags)
		}
	}
	if len(tags) == 0 {
		return nil, nil
	}
	return tags, nil
}

func parseTagFilters(q map[string][]string) ([]string, error) {
	raw := append([]string{}, q["tag"]...)
	if extra := strings.TrimSpace(strings.Join(q["tags"], ",")); extra != "" {
		raw = append(raw, strings.Split(extra, ",")...)
	}
	return normalizeTags(raw)
}

func documentHasAnyTag(doc document, filters []string) bool {
	if len(doc.Tags) == 0 || len(filters) == 0 {
		return false
	}

	docTags := make(map[string]struct{}, len(doc.Tags))
	for _, tag := range doc.Tags {
		docTags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, filter := range filters {
		if _, ok := docTags[strings.ToLower(filter)]; ok {
			return true
		}
	}
	return false
}
