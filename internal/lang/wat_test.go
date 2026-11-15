package lang

import "testing"

// The skeleton lands the CompileWat seam Builder C codes against; the
// emitter replaces the placeholder error.
func TestWatSkeletonSignature(t *testing.T) {
	_, err := CompileWat("repl", `print(1)`, nil)
	if err == nil || err.Error() != "wat emitter: not implemented" {
		t.Fatalf("skeleton error: got %v", err)
	}
}
