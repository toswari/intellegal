//go:build !integration

package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"legal-doc-intel/go-api/internal/ai"
)

func TestNormalizeTags_RemovesDuplicatesAndWhitespace(t *testing.T) {
	// Arrange

	// Act
	tags, err := normalizeTags([]string{"  Finance  ", "finance", "", "MSA"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0] != "Finance" || tags[1] != "MSA" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestNormalizeTags_ReturnsErrorForLongAndExcessiveTags(t *testing.T) {
	// Arrange
	longTag := "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"

	// Act
	if _, err := normalizeTags([]string{longTag}); err == nil {
		t.Fatal("expected tag length validation error")
	}

	input := make([]string, 21)
	for i := range input {
		input[i] = newUUID()
	}

	// Assert
	if _, err := normalizeTags(input); err == nil {
		t.Fatal("expected max tag validation error")
	}
}

func TestParseTagFilters_MergesSingularAndPluralTagParams(t *testing.T) {
	// Arrange

	// Act
	filters, err := parseTagFilters(url.Values{
		"tag":  []string{"Finance"},
		"tags": []string{"MSA, procurement "},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Assert
	if len(filters) != 3 {
		t.Fatalf("expected 3 filters, got %d", len(filters))
	}
}

func TestDocumentHasAnyTag_MatchesTagsCaseInsensitively(t *testing.T) {
	// Arrange
	doc := document{Tags: []string{"Finance", "MSA"}}

	// Act and Assert
	if !documentHasAnyTag(doc, []string{"finance"}) {
		t.Fatal("expected document to match lower-cased filter")
	}
	if documentHasAnyTag(doc, []string{"privacy"}) {
		t.Fatal("expected non-matching filter to return false")
	}
}

func TestCombineExtractedText_PrefersExplicitTextAndOtherwiseCombinesPages(t *testing.T) {
	// Arrange and Act
	got := combineExtractedText(ai.ExtractResult{
		Pages: []ai.ExtractPage{
			{PageNumber: 1, Text: " First page "},
			{PageNumber: 2, Text: ""},
			{PageNumber: 3, Text: "Third page"},
		},
	})
	if got != "First page\n\nThird page" {
		t.Fatalf("unexpected combined text: %q", got)
	}

	// Act
	verbatim := combineExtractedText(ai.ExtractResult{
		Text: "  indexed text  ",
		Pages: []ai.ExtractPage{
			{PageNumber: 1, Text: "ignored"},
		},
	})
	if verbatim != "indexed text" {
		t.Fatalf("expected explicit extracted text to win, got %q", verbatim)
	}
}

func TestWriteCreateDocumentError_ReturnsBadRequestForTagValidationError(t *testing.T) {
	// Arrange
	api := NewAPI(noopLogger{}, nil, nil, nil)
	recorder := httptest.NewRecorder()
	err := errors.New("tag must be at most 50 characters")

	// Act
	api.writeCreateDocumentError(recorder, err)

	// Assert
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	body := decodeJSONBody(t, recorder)
	if body.Error.Code != "invalid_argument" {
		t.Fatalf("expected error code %q, got %q", "invalid_argument", body.Error.Code)
	}
}

func TestWriteCreateDocumentError_ReturnsBadGatewayForPersistenceFailure(t *testing.T) {
	// Arrange
	api := NewAPI(noopLogger{}, nil, nil, nil)
	recorder := httptest.NewRecorder()
	err := errors.New("failed to persist document")

	// Act
	api.writeCreateDocumentError(recorder, err)

	// Assert
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, recorder.Code)
	}

	body := decodeJSONBody(t, recorder)
	if body.Error.Code != "storage_unavailable" {
		t.Fatalf("expected error code %q, got %q", "storage_unavailable", body.Error.Code)
	}
}

func TestWriteCreateDocumentError_ReturnsInternalServerErrorForUnexpectedFailure(t *testing.T) {
	// Arrange
	api := NewAPI(noopLogger{}, nil, nil, nil)
	recorder := httptest.NewRecorder()
	err := errors.New("boom")

	// Act
	api.writeCreateDocumentError(recorder, err)

	// Assert
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
	}

	body := decodeJSONBody(t, recorder)
	if body.Error.Code != "internal_error" {
		t.Fatalf("expected error code %q, got %q", "internal_error", body.Error.Code)
	}
}
