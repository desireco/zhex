package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestContinuationIndentTracksNestedBlocksAndDedent(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   int
	}{
		{"word f() -> Int\n", 4},
		{"word f() -> Int\n    if true\n", 8},
		{"word f() -> Int\n    if true\n        1\n", 8},
		{"word f() -> Int\n    if true\n        1\n    else\n", 8},
		{"word f() -> Int\n    if true\n        1\n    else\n        2\n    3\n", 4},
	} {
		if got := continuationIndent(tc.source); got != tc.want {
			t.Errorf("%q: got %d, want %d", tc.source, got, tc.want)
		}
	}
}

func TestPipedReplHasNoTerminalControlSequences(t *testing.T) {
	var out, diagnostics bytes.Buffer
	script := "word f() -> Int\n    42\n    \n\nf()\n"
	if status := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics); status != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "42\n") || strings.Contains(out.String(), "\x1b") {
		t.Fatalf("exit %d, stdout %q, stderr %q", status, out.String(), diagnostics.String())
	}
}
