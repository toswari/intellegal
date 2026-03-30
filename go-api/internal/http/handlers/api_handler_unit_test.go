package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type noopLogger struct{}

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

func TestCreateDocumentRejectsUnsupportedMIMEType(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	resp := performJSONRequest(t, http.MethodPost, "/api/v1/documents", map[string]any{
		"filename":       "contract.txt",
		"mime_type":      "text/plain",
		"content_base64": "dGVzdA==",
	}, api.CreateDocument)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	body := decodeJSONBody(t, resp)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument error code, got %q", body.Error.Code)
	}
}

func TestCreateClauseCheckRejectsShortText(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	resp := performJSONRequest(t, http.MethodPost, "/api/v1/checks/clause-presence", map[string]any{
		"required_clause_text": "abc",
	}, api.CreateClauseCheck)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	body := decodeJSONBody(t, resp)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument error code, got %q", body.Error.Code)
	}
}

func TestCreateCompanyNameCheckRejectsShortOldCompanyName(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	resp := performJSONRequest(t, http.MethodPost, "/api/v1/checks/company-name", map[string]any{
		"old_company_name": " ",
	}, api.CreateCompanyNameCheck)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	body := decodeJSONBody(t, resp)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument error code, got %q", body.Error.Code)
	}
}

func TestCreateClauseCheckRejectsUnknownDocumentID(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)

	resp := performJSONRequest(t, http.MethodPost, "/api/v1/checks/clause-presence", map[string]any{
		"document_ids":         []string{"00000000-0000-4000-8000-000000000001"},
		"required_clause_text": "payment terms are required",
	}, api.CreateClauseCheck)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}

	body := decodeJSONBody(t, resp)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected invalid_argument error code, got %q", body.Error.Code)
	}
}

func TestGetCheckResultsReturnsConflictWhenNotCompleted(t *testing.T) {
	api := NewAPI(noopLogger{}, nil, nil, nil)
	checkID := "00000000-0000-4000-8000-000000000001"
	api.checks[checkID] = checkRun{
		CheckID:     checkID,
		Status:      checkStatusQueued,
		CheckType:   checkTypeClause,
		RequestedAt: time.Now().UTC(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/checks/"+checkID+"/results", nil)
	req.SetPathValue("check_id", checkID)
	w := httptest.NewRecorder()

	api.GetCheckResults(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}

	body := decodeJSONBody(t, w)
	if body.Error.Code != "results_not_ready" {
		t.Fatalf("expected results_not_ready, got %q", body.Error.Code)
	}
}

type errorResponse struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func performJSONRequest(t *testing.T, method, path string, payload any, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func decodeJSONBody(t *testing.T, resp *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	var out errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	return out
}
