//go:build !integration

package health

import "testing"

func TestOK_ReturnsHealthyStatus(t *testing.T) {
	// Arrange

	// Act
	got := OK()

	// Assert
	if got.Status != "ok" {
		t.Fatalf("expected status=ok, got %q", got.Status)
	}
}
