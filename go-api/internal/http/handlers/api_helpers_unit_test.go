//go:build !integration

package handlers

import "testing"

func TestHashPayload_IsDeterministic(t *testing.T) {
	payload := map[string]any{
		"required_clause_text": "Payment terms",
		"context_hint":         "MSA",
	}
	documentIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"550e8400-e29b-41d4-a716-446655440001",
	}

	left, err := hashPayload(payload, documentIDs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	right, err := hashPayload(payload, documentIDs)
	if err != nil {
		t.Fatalf("expected no error on second hash, got %v", err)
	}

	if left != right {
		t.Fatalf("expected stable hash, got %q and %q", left, right)
	}
}

func TestHashPayload_ReturnsErrorForUnmarshalablePayload(t *testing.T) {
	_, err := hashPayload(map[string]any{"bad": func() {}}, nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
}
