//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	logger "github.com/Gratheon/log-lib-go"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/http/handlers"
	"legal-doc-intel/go-api/internal/logging"
)

type mockAIClient struct{}

func (mockAIClient) AnalyzeClause(_ context.Context, _ ai.AnalyzeClauseRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (mockAIClient) AnalyzeLLMReview(_ context.Context, _ ai.AnalyzeLLMReviewRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (mockAIClient) ContractChat(_ context.Context, _ ai.ContractChatRequest) (ai.ContractChatResult, error) {
	return ai.ContractChatResult{}, nil
}
func (mockAIClient) Extract(_ context.Context, req ai.ExtractRequest) (ai.ExtractResult, error) {
	return ai.ExtractResult{MIMEType: req.MIMEType, Text: "ok"}, nil
}
func (mockAIClient) Index(_ context.Context, req ai.IndexRequest) (ai.IndexResult, error) {
	return ai.IndexResult{DocumentID: req.DocumentID, Checksum: req.VersionChecksum, Indexed: true}, nil
}
func (mockAIClient) SearchSections(_ context.Context, _ ai.SearchSectionsRequest) (ai.SearchSectionsResult, error) {
	return ai.SearchSectionsResult{}, nil
}

func TestHealthEndpoint_ReturnsOKAndRequestID(t *testing.T) {
	// Arrange
	log := logging.NewDiscard(logger.New(logger.LoggerConfig{}))
	api := handlers.NewAPI(log, mockAIClient{}, nil, nil)
	handler := New(log, api, nil, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %#v", body["status"])
	}
	if _, ok := body["timestamp"].(string); !ok {
		t.Fatalf("expected timestamp string, got %#v", body["timestamp"])
	}

	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
}

func TestReadinessEndpoint_ReturnsOKWhenDependencyCheckSucceeds(t *testing.T) {
	// Arrange
	log := logging.NewDiscard(logger.New(logger.LoggerConfig{}))
	api := handlers.NewAPI(log, mockAIClient{}, nil, nil)
	handler := New(log, api, []handlers.DependencyProbe{
		handlers.NewDependencyProbe("postgres", func(_ context.Context) error { return nil }),
	}, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Status       string                       `json:"status"`
		Dependencies map[string]map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal readiness response: %v", err)
	}
	if body.Status != "ready" {
		t.Fatalf("expected ready status, got %q", body.Status)
	}
	if body.Dependencies["postgres"]["status"] != "up" {
		t.Fatalf("expected postgres dependency up, got %#v", body.Dependencies["postgres"])
	}
}

func TestReadinessEndpoint_ReturnsServiceUnavailableWhenDependencyCheckFails(t *testing.T) {
	// Arrange
	log := logging.NewDiscard(logger.New(logger.LoggerConfig{}))
	api := handlers.NewAPI(log, mockAIClient{}, nil, nil)
	handler := New(
		log,
		api,
		[]handlers.DependencyProbe{
			handlers.NewDependencyProbe("postgres", func(_ context.Context) error { return context.DeadlineExceeded }),
		},
		[]string{"http://localhost:3000"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
	w := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(w, req)

	// Assert
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}

	var body struct {
		Status       string                       `json:"status"`
		Dependencies map[string]map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal readiness response: %v", err)
	}
	if body.Status != "not_ready" {
		t.Fatalf("expected not_ready status, got %q", body.Status)
	}
	if body.Dependencies["postgres"]["status"] != "down" {
		t.Fatalf("expected postgres dependency down, got %#v", body.Dependencies["postgres"])
	}
}
