package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReplBlockValuesAndMatchContinuation(t *testing.T) {
	source := "let clean = flow \"  ADA  \"\n    text.trim\n    text.lower\n\nclean\nlet value = match Option.Some(3)\n    case Option.Some(n)\n        n\n    case Option.None\n        0\n\nprint(value)\n"
	var out, diagnostics bytes.Buffer
	if status := run([]string{"repl"}, strings.NewReader(source), &out, &diagnostics); status != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "ada\n") || !strings.Contains(out.String(), "3\n") {
		t.Fatalf("exit %d stdout %q stderr %q", status, out.String(), diagnostics.String())
	}
}
