package router

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"legal-doc-intel/go-api/internal/ai"
	"legal-doc-intel/go-api/internal/http/handlers"
)

type mockAIClient struct{}

func (mockAIClient) AnalyzeClause(_ context.Context, _ ai.AnalyzeClauseRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
}
func (mockAIClient) AnalyzeCompanyName(_ context.Context, _ ai.AnalyzeCompanyNameRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{}, nil
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

func TestHealthEndpoint(t *testing.T) {
	api := handlers.NewAPI(slog.New(slog.NewJSONHandler(io.Discard, nil)), mockAIClient{}, nil, nil)
	handler := New(slog.New(slog.NewJSONHandler(io.Discard, nil)), api, nil, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
}

func TestReadinessEndpoint(t *testing.T) {
	api := handlers.NewAPI(slog.New(slog.NewJSONHandler(io.Discard, nil)), mockAIClient{}, nil, nil)
	handler := New(slog.New(slog.NewJSONHandler(io.Discard, nil)), api, nil, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadinessEndpointUnavailable(t *testing.T) {
	api := handlers.NewAPI(slog.New(slog.NewJSONHandler(io.Discard, nil)), mockAIClient{}, nil, nil)
	handler := New(
		slog.New(slog.NewJSONHandler(io.Discard, nil)),
		api,
		func(_ context.Context) error { return context.DeadlineExceeded },
		[]string{"http://localhost:3000"},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/readiness", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
