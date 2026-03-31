//go:build !integration

package checksum

import "testing"

func TestSHA256Hex_ReturnsExpectedDigest(t *testing.T) {
	got := SHA256Hex([]byte("hello world"))
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
