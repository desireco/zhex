package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestResultWordReturnsExplicitSuccess(t *testing.T) {
	source := "word answer() -> Result<Int, ParseError>\n    ok(42)\n\nprint(answer() try)\n"
	var output bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "42\n" {
		t.Fatalf("output = %q, want 42", got)
	}
}

func TestResultWordRejectsIncorrectSuccessType(t *testing.T) {
	source := "word answer() -> Result<Int, ParseError>\n    ok(\"wrong\")\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted wrong ok value type")
	}
}

func TestResultWordTraceRendersSuccess(t *testing.T) {
	source := "word answer() -> Result<Int, ParseError>\n    ok(42)\n\nprint(answer() try)\n"
	var output, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &output, &trace); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trace.String(), `"event":"word-return","word":"answer","value":"ok(\"42\")"`) {
		t.Fatalf("unexpected trace: %s", trace.String())
	}
}

func TestResultWordRejectsUnknownErrorFamily(t *testing.T) {
	if _, err := Compile("test.zhex", "word answer() -> Result<Int, Mystery>\n    ok(42)\n"); err == nil {
		t.Fatal("accepted unknown error family")
	}
}
