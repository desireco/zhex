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

func TestTryReturnsFailureAtWordBoundary(t *testing.T) {
	source := "word parse(s: String) -> Result<Int, ParseError>\n    let n = text.to-int(s) try\n    ok(n)\n\nparse(\"bad\") try\n"
	var output, trace bytes.Buffer
	err := RunWithTrace("test.zhex", source, strings.NewReader(""), &output, &trace)
	if err == nil || !strings.Contains(err.Error(), "not a signed 64-bit decimal integer") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(trace.String(), `"event":"word-return","word":"parse","value":"err(`) {
		t.Fatalf("failure did not return through word boundary: %s", trace.String())
	}
}

func TestTryRequiresCompatibleFallibleWord(t *testing.T) {
	cases := []string{
		"word parse(s: String) -> Int\n    text.to-int(s) try\n",
		"word parse(s: String) -> Result<Int, InputError>\n    let n = text.to-int(s) try\n    ok(n)\n",
	}
	for _, source := range cases {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Fatalf("accepted incompatible try in %q", source)
		}
	}
}

func TestTryPropagatesThroughMultipleWordsWithoutChangingOrigin(t *testing.T) {
	source := "word inner(s: String) -> Result<Int, ParseError>\n    ok(text.to-int(s) try)\nword outer(s: String) -> Result<Int, ParseError>\n    ok(inner(s) try)\nprint(outer(\"bad\") try)\n"
	var output bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &output)
	if err == nil || !strings.HasPrefix(err.Error(), "test.zhex:2:") {
		t.Fatalf("origin lost: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected output: %q", output.String())
	}
}
