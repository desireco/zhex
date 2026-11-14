package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestReplMultilineDelimitersAndBlankLines(t *testing.T) {
	script := "print(\n\n    1 + 1\n)\nflow text.replace(\n    \"Ada\",\n    \"A\",\n    \"E\"\n)\n    print\n\n"
	var out, diagnostics bytes.Buffer
	if code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "2\n") || !strings.Contains(out.String(), "Eda\n") {
		t.Fatalf("exit %d out %q stderr %q", code, out.String(), diagnostics.String())
	}
}

func TestReplUnclosedDelimiterAtPipedEOFDiagnoses(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := run([]string{"repl"}, strings.NewReader("print(\n  1"), &out, &diagnostics); code != 0 || !strings.Contains(diagnostics.String(), "unclosed delimiter") {
		t.Fatalf("exit %d stderr %q", code, diagnostics.String())
	}
}
