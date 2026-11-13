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

func TestCatchRunsFallbackOnlyOnFailure(t *testing.T) {
	source := "let good = text.to-int(\"12\") catch\n    print(\"unexpected\")\n    0\nlet bad = text.to-int(\"no\") catch problem\n    print(\"using fallback\")\n    3\nprint(good + bad)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "using fallback\n15\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchRejectsMismatchedFallback(t *testing.T) {
	source := "let n = text.to-int(\"bad\") catch\n    \"wrong\"\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted mismatched catch fallback")
	}
}

func TestCatchDoesNotInterceptRuntimeFaultInOperand(t *testing.T) {
	source := "let n = text.to-int(int.to-string(1 / 0)) catch\n    3\nprint(n)\n"
	var output bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &output)
	if err == nil || !strings.Contains(err.Error(), "division or remainder by zero") {
		t.Fatalf("runtime fault lost: %v", err)
	}
}

func TestCatchBindingIsLocal(t *testing.T) {
	source := "let n = text.to-int(\"bad\") catch problem\n    0\nprint(problem)\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "unknown name") {
		t.Fatalf("handler binding escaped: %v", err)
	}
}

func TestCatchRecoversFailureReturnedByWord(t *testing.T) {
	source := "word parse(s: String) -> Result<Int, ParseError>\n    ok(text.to-int(s) try)\nlet n = parse(\"bad\") catch problem\n    7\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestCatchInspectsErrorOriginAndCode(t *testing.T) {
	source := "let n = text.to-int(\"bad\") catch problem\n    print(error.code(problem))\n    print(error.operation(problem))\n    print(error.file(problem))\n    print(error.line(problem))\n    print(error.column(problem))\n    0\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "text-to-int\ntext.to-int\ntest.zhex\n1\n9\n0\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestFileErrorExposesNormalizedDetails(t *testing.T) {
	source := "let n = file.read-all(\"/nonexistent-zhex-file\") catch problem\n    print(error.detail(problem, \"reason\"))\n    print(error.detail(problem, \"path\"))\n    \"\"\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "not-found\n/nonexistent-zhex-file\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestErrorDetailsForBufferAndArguments(t *testing.T) {
	source := "let b = buffer.new()\nlet line = buffer.line(b, 4) catch problem\n    print(error.detail(problem, \"index\"))\n    print(error.detail(problem, \"line-count\"))\n    \"\"\nlet arg = console.arg(2) catch problem\n    print(error.detail(problem, \"index\"))\n    print(error.detail(problem, \"argument-count\"))\n    \"\"\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n0\n2\n0\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestErrorCanBeInspectedByOrdinaryWord(t *testing.T) {
	source := "word describe(problem: Error) -> String\n    error.code(problem)\nlet n = text.to-int(\"bad\") catch problem\n    print(describe(problem))\n    0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n" {
		t.Fatalf("output = %q", out.String())
	}
}
