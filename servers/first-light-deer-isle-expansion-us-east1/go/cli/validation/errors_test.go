package validation

import "testing"

func TestErrFailedMessage(t *testing.T) {
	if ErrFailed.Error() != "validation failed" {
		t.Fatalf("ErrFailed = %q, want validation failed", ErrFailed.Error())
	}
}
