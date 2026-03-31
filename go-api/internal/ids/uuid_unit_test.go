//go:build !integration

package ids

import "testing"

func TestIsUUID_AcceptsCanonicalUUIDsCaseInsensitively(t *testing.T) {
	if !IsUUID("550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("expected lowercase UUID to be accepted")
	}
	if !IsUUID("550E8400-E29B-41D4-A716-446655440000") {
		t.Fatal("expected uppercase UUID to be accepted")
	}
}

func TestIsUUID_RejectsMalformedValues(t *testing.T) {
	cases := []string{
		"",
		"not-a-uuid",
		"550e8400e29b41d4a716446655440000",
		"550e8400-e29b-61d4-a716-446655440000",
		"550e8400-e29b-41d4-6716-446655440000",
	}

	for _, tc := range cases {
		if IsUUID(tc) {
			t.Fatalf("expected %q to be rejected", tc)
		}
	}
}

func TestNewUUID_ReturnsValidVersion4UUID(t *testing.T) {
	value := NewUUID()
	if !IsUUID(value) {
		t.Fatalf("expected generated UUID to be valid, got %q", value)
	}
	if value[14] != '4' {
		t.Fatalf("expected version 4 UUID, got %q", value)
	}
	switch value[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Fatalf("expected RFC 4122 variant, got %q", value)
	}
}
