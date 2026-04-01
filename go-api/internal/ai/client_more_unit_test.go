//go:build !integration

package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient_TrimsBaseURLAndDefaultsTimeout(t *testing.T) {
	client := NewClient("https://example.test/", "", 0)

	if client.baseURL != "https://example.test" {
		t.Fatalf("expected trimmed base url, got %q", client.baseURL)
	}
	if client.httpClient.Timeout != 10*time.Second {
		t.Fatalf("expected default timeout, got %v", client.httpClient.Timeout)
	}
}

func TestPostJSONWithResponse_ReturnsErrorForUnexpectedStatus(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, " request failed ", http.StatusBadGateway)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	err := client.postJSONWithResponse(context.Background(), "/internal/v1/test", map[string]string{"ok": "yes"}, &struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "unexpected status 502: request failed" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestPostJSONWithResponse_ReturnsDecodeErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("{"))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	err := client.postJSONWithResponse(context.Background(), "/internal/v1/test", map[string]string{"ok": "yes"}, &struct{}{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "decode response:") {
		t.Fatalf("expected decode response error, got %q", err.Error())
	}
}

func TestPostJSON_ReturnsMarshalError(t *testing.T) {
	t.Parallel()

	client := NewClient("https://example.test", "", time.Second)

	err := client.postJSON(context.Background(), "/internal/v1/test", map[string]any{"bad": func() {}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "marshal request:") {
		t.Fatalf("expected marshal request error, got %q", err.Error())
	}
}

func TestContractChat_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	var seenPath string
	var seenBody ContractChatRequest

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&seenBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-chat-1",
			"status":   "completed",
			"job_type": "contract_chat",
			"result": map[string]any{
				"answer": "Answer",
				"citations": []map[string]any{
					{"document_id": "doc-1", "snippet_text": "citation"},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	result, err := client.ContractChat(context.Background(), ContractChatRequest{
		JobID:      "job-chat-1",
		ContractID: "contract-1",
		Messages:   []ContractChatMessage{{Role: "user", Content: "Question?"}},
		Documents:  []ContractChatDocument{{DocumentID: "doc-1", Text: "Contract text"}},
	})
	if err != nil {
		t.Fatalf("ContractChat returned error: %v", err)
	}

	if seenPath != "/internal/v1/chat/contract" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if seenBody.ContractID != "contract-1" || len(seenBody.Messages) != 1 || len(seenBody.Documents) != 1 {
		t.Fatalf("unexpected body payload: %#v", seenBody)
	}
	if result.Answer != "Answer" || len(result.Citations) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExtract_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	var seenPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-extract-1",
			"status":   "completed",
			"job_type": "extract",
			"result": map[string]any{
				"mime_type": "application/pdf",
				"text":      "Extracted text",
				"pages": []map[string]any{
					{"page_number": 1, "text": "Extracted text", "char_count": 14, "confidence": 0.9, "source": "ocr"},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	result, err := client.Extract(context.Background(), ExtractRequest{
		JobID:      "job-extract-1",
		DocumentID: "doc-1",
		StorageURI: "s3://bucket/doc-1.pdf",
		MIMEType:   "application/pdf",
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if seenPath != "/internal/v1/extract" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if result.MIMEType != "application/pdf" || len(result.Pages) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestIndex_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	var seenPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-index-1",
			"status":   "completed",
			"job_type": "index",
			"result": map[string]any{
				"document_id": "doc-1",
				"checksum":    "abc123",
				"chunk_count": 2,
				"indexed":     true,
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	result, err := client.Index(context.Background(), IndexRequest{
		JobID:           "job-index-1",
		DocumentID:      "doc-1",
		VersionChecksum: "abc123",
		ExtractedText:   "text",
	})
	if err != nil {
		t.Fatalf("Index returned error: %v", err)
	}

	if seenPath != "/internal/v1/index" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if result.DocumentID != "doc-1" || !result.Indexed {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSearchSections_PostsExpectedRequest(t *testing.T) {
	t.Parallel()

	var seenPath string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":   "job-search-1",
			"status":   "completed",
			"job_type": "search_sections",
			"result": map[string]any{
				"items": []map[string]any{
					{
						"document_id":  "doc-1",
						"page_number":  3,
						"chunk_id":     "chunk-3",
						"score":        0.93,
						"snippet_text": "payment terms",
					},
				},
			},
		})
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "", time.Second)

	result, err := client.SearchSections(context.Background(), SearchSectionsRequest{
		JobID:       "job-search-1",
		QueryText:   "payment terms",
		DocumentIDs: []string{"doc-1"},
		Limit:       3,
		Strategy:    "semantic",
	})
	if err != nil {
		t.Fatalf("SearchSections returned error: %v", err)
	}

	if seenPath != "/internal/v1/search/sections" {
		t.Fatalf("unexpected path: %q", seenPath)
	}
	if len(result.Items) != 1 || result.Items[0].DocumentID != "doc-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}
