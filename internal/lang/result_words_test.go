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
	if err == nil || !strings.Contains(err.Error(), "not a signed 128-bit decimal integer") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(trace.String(), `"event":"word-return","word":"parse","value":"err(`) {
		t.Fatalf("failure did not return through word boundary: %s", trace.String())
	}
}

func TestTryRequiresCompatibleFallibleWord(t *testing.T) {
	cases := []string{
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

func TestCatchOneLineFallbackEvaluates(t *testing.T) {
	source := "let good = text.to-int(\"42\") catch 0\nvar bad = text.to-int(\"x\") catch 7\nprint(good)\nprint(bad)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "42\n7\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchOneLineHandlesStoredResult(t *testing.T) {
	source := "let p = text.to-int(\"x\")\np catch 0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output = %q", out.String())
	}
}

func TestCatchOneLineRejectsMismatchedFallback(t *testing.T) {
	if _, err := Compile("test.zhex", "let n = text.to-int(\"x\") catch \"wrong\"\n"); err == nil {
		t.Fatal("accepted mismatched one-line catch fallback")
	}
}

// #181: catch also binds in assignment right-hand sides and inside call
// arguments, evaluating identically to the let form. The one-line fallback
// extends to the argument's boundary — the next comma, the closing paren,
// or the statement newline.
func TestCatchAssignmentFallbackEvaluates(t *testing.T) {
	source := "var n = 0\nn = text.to-int(\"42\") catch 7\nprint(n)\nn = text.to-int(\"x\") catch 7\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "42\n7\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchAssignmentBlockFormBindsError(t *testing.T) {
	source := "var n = 0\nn = text.to-int(\"x\") catch problem\n    print(error.code(problem))\n    7\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "text-to-int\n7\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchInCallArgumentEvaluates(t *testing.T) {
	source := "let xs: List<Int> = [1]\nlet ys = list.append(xs, text.to-int(\"x\") catch 0)\nlet zs = list.append(ys, text.to-int(\"9\") catch 0)\nprint(list.length(zs))\nprint(zs[2])\nprint(zs[1])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "3\n9\n0\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchInArgumentFallbackExtendsToBoundary(t *testing.T) {
	source := "print(text.to-int(\"x\") catch 0 + 1)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "1\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchInGroupedExpressionEvaluates(t *testing.T) {
	source := "let n = (text.to-int(\"x\") catch 3)\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "3\n" {
		t.Fatalf("output = %q", got)
	}
}

// The compat rule: a trailing ungrouped catch on the last argument of a
// short-form call lifts to guard the whole call, exactly as trailing try does.
func TestShortFormTrailingCatchGuardsWholeCall(t *testing.T) {
	source := "word parse(s: String) -> Result<Int, ParseError>\n    ok(text.to-int(s) try)\nvar n = 0\nn = parse \"x\" catch 8\nprint(n)\nn = parse \"10\" catch 8\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "8\n10\n" {
		t.Fatalf("output = %q", got)
	}
}

// Parenthesizing keeps a trailing catch inside the last argument of a
// short-form call instead of lifting it to the whole call.
func TestShortFormGroupedArgumentCatchStaysInside(t *testing.T) {
	source := "word scale(n: Int) -> Int\n    n * 100\nprint(scale (text.to-int(\"x\") catch 5))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "500\n" {
		t.Fatalf("output = %q", got)
	}
}

// A catch on an earlier argument binds that argument (today the trailing
// comma after a whole-call fallback is a syntax error).
func TestCatchOnEarlierShortFormArgumentBindsArgument(t *testing.T) {
	source := "word pair(a: Int, b: Int) -> Int\n    a * 10 + b\nprint(pair(text.to-int(\"x\") catch 1, 2))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "12\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchInFlowStepArgumentEvaluates(t *testing.T) {
	source := "let xs: List<Int> = []\nflow xs\n    list.append(text.to-int(\"x\") catch 0)\nprint(list.length(xs))\nprint(xs[0])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "1\n0\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchInArgumentRejectsMismatchedFallback(t *testing.T) {
	source := "let xs: List<Int> = [1]\nlet ys = list.append(xs, text.to-int(\"x\") catch \"wrong\")\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted mismatched argument catch fallback")
	}
}

func TestCatchAssignmentRequiresResult(t *testing.T) {
	if _, err := Compile("test.zhex", "var n = 0\nn = int.to-string(1) catch \"x\"\n"); err == nil || !strings.Contains(err.Error(), "catch requires a Result") {
		t.Fatalf("assignment catch accepted a non-Result: %v", err)
	}
}

// The disambiguation rule: any same-line content after catch is a one-line
// fallback expression, except a lone name before the newline, which stays the
// block form's error binding.
func TestCatchOneLineSameLineContentWins(t *testing.T) {
	source := "word fallback(n: Int) -> Int\n    n\nlet n = text.to-int(\"x\") catch fallback(2 * 3 + 1)\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "7\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestCatchLoneNameBeforeNewlineStaysBlock(t *testing.T) {
	// A name directly before the newline is the block form's error binding,
	// not a fallback reference to a same-named value, so the indented handler
	// stays required.
	source := "let fallback = 5\nlet n = text.to-int(\"x\") catch fallback\n"
	_, err := Compile("test.zhex", source)
	if err == nil {
		t.Fatal("a lone name after catch must keep the block form")
	}
	if !strings.Contains(err.Error(), "parenthesize a lone identifier fallback: catch (fallback)") {
		t.Fatalf("missing lone-identifier catch hint: %v", err)
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
	source := "let n = file.read(\"/nonexistent-zhex-file\") catch problem\n    print(error.detail(problem, \"reason\"))\n    print(error.detail(problem, \"path\"))\n    \"\"\n"
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
	source := "word describe(problem: Error) -> String\n    error.code(problem)\nlet n = text.to-int(\"bad\") catch problem\n    print(describe(error.wrap(\"text-to-int\", \"converted\", problem)))\n    0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "text-to-int\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestCatchCanConvertErrorAndReturnEarly(t *testing.T) {
	source := "word load(path: String) -> Result<String, Error>\n    let text = file.read(path) catch problem\n        return err(error.wrap(\"settings-read\", \"could not read settings\", problem))\n    ok(text)\nlet result = load(\"/nonexistent-zhex-file\") catch problem\n    print(error.code(problem))\n    print(error.operation(problem))\n    \"default\"\nprint(result)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "settings-read\nfile.read\ndefault\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestWordCanCreateDomainError(t *testing.T) {
	source := "word validate(n: Int) -> Result<Int, Error>\n    if n < 0\n        return err(error.new(\"negative-count\", \"count must be positive\"))\n    ok(n)\nlet n = validate(-1) catch problem\n    print(error.code(problem))\n    0\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "negative-count\n0\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestErrorCauseIsPreservedAndAbsentCauseFails(t *testing.T) {
	source := "word load() -> Result<String, Error>\n    let text = file.read(\"/nonexistent-zhex-file\") catch problem\n        return err(error.wrap(\"settings-read\", \"failed\", problem))\n    ok(text)\nlet text = load() catch problem\n    print(error.has-cause(problem))\n    let original = error.cause(problem) try\n    print(error.code(original))\n    print(error.has-cause(original))\n    let fallback = error.cause(original) catch missing\n        print(error.code(missing))\n        problem\n    \"default\"\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "true\nfile-open\nfalse\nerror-no-cause\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestErrRequiresMatchingFamilyAndFallibleWord(t *testing.T) {
	cases := []string{
		"word bad() -> Result<Int, ParseError>\n    let text = file.read(\"missing\") catch problem\n        return err(problem)\n    ok(1)\n",
		"return err(error.new(\"bad\", \"bad\"))\n",
	}
	for _, source := range cases {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Fatalf("accepted invalid return: %q", source)
		}
	}
}

func TestErrorAliasPreservesFamily(t *testing.T) {
	bad := "word parse() -> Result<Int, Error>\n    let n = text.to-int(\"bad\") catch problem\n        let alias = problem\n        return err(alias)\n    ok(n)\n"
	if _, err := Compile("test.zhex", bad); err == nil {
		t.Fatal("alias widened ParseError to Error")
	}
	good := "word parse() -> Result<Int, ParseError>\n    let n = text.to-int(\"bad\") catch problem\n        let alias = problem\n        return err(alias)\n    ok(n)\nparse() try\n"
	var out bytes.Buffer
	if err := Run("test.zhex", good, strings.NewReader(""), &out); err == nil || !strings.Contains(err.Error(), "not a signed 128-bit") {
		t.Fatalf("same-family alias: %v", err)
	}
}

func TestErrorIdentityWordCannotWidenConcreteFamily(t *testing.T) {
	source := "word identity(problem: Error) -> Error\n    problem\nword parse() -> Result<Int, Error>\n    let n = text.to-int(\"bad\") catch problem\n        return err(identity(problem))\n    ok(n)\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("identity word widened concrete error family")
	}
}

func TestFallibleFlowRequiresTryAndPropagates(t *testing.T) {
	source := "word parse(s: String) -> Result<Int, ParseError>\n    ok(text.to-int(s) try)\nflow \"42\"\n    parse try\n    print\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "42\n" {
		t.Fatalf("output = %q", out.String())
	}
	bad := strings.Replace(source, "    parse try\n", "    parse\n", 1)
	if _, err := Compile("test.zhex", bad); err == nil {
		t.Fatal("accepted bare fallible flow step")
	}
	source = strings.Replace(source, `flow "42"`, `flow "bad"`, 1)
	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil || !strings.Contains(err.Error(), "not a signed 128-bit") || out.Len() != 0 {
		t.Fatalf("flow failure: %v, output %q", err, out.String())
	}
}

func TestFallibleFlowInsideFallibleWord(t *testing.T) {
	source := "word parse(s: String) -> Result<Int, ParseError>\n    ok(text.to-int(s) try)\nword pipeline(s: String) -> Result<Int, ParseError>\n    flow s\n        parse try\n        ok\nprint(pipeline(\"12\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestOkCannotWidenConcreteErrorFamily(t *testing.T) {
	source := "word extract() -> Result<Error, Error>\n    let n = text.to-int(\"bad\") catch problem\n        return ok(problem)\n    ok(error.new(\"unused\", \"unused\"))\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("ok widened ParseError to Error")
	}
}

func TestCatchFallbackCannotWidenConcreteError(t *testing.T) {
	source := "word fail() -> Result<Error, Error>\n    let n = text.to-int(\"bad\") catch problem\n        let x = error.cause(error.new(\"e\", \"e\")) catch missing\n            problem\n        return ok(x)\n    ok(error.new(\"e\", \"e\"))\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("catch fallback widened ParseError to Error")
	}
}

func TestResultsMustBeHandledImmediately(t *testing.T) {
	invalid := []string{
		"text.to-int(\"1\")\n",
		"let p = text.to-int(\"1\")\n",
		"var p = text.to-int(\"1\")\nlet n = p try\n",
		"let p = text.to-int(\"1\")\nprint(1)\nlet n = p try\n",
		"let p = text.to-int(\"1\")\nprint(p try)\n",
		"let p = text.to-int(\"1\")\nlet q = p\n",
		"let p = text.to-int(\"1\")\nif true\n    p try\n",
		"let p = text.to-int(\"1\")\nlet n = p try\nlet m = p try\n",
		"let p = text.to-int(\"bad\")\np catch\n    p try\n",
		"word bad() -> Result<Int, ParseError>\n    text.to-int(\"1\")\n",
		"word bad() -> Result<Int, ParseError>\n    return text.to-int(\"1\")\n",
	}
	for _, source := range invalid {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted unchecked Result: %q", source)
		}
	}
	valid := "let p = text.to-int(\"1\")\nlet n = p try\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", valid, strings.NewReader(""), &out); err != nil || out.String() != "1\n" {
		t.Fatalf("valid stored Result: %v, %q", err, out.String())
	}
}

func TestResultWordCanConstructResultInBothBranches(t *testing.T) {
	source := "word validate(n: Int) -> Result<Int, Error>\n    if n > 0\n        ok(n)\n    else\n        err(error.new(\"invalid\", \"invalid\"))\nprint(validate(2) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLoopCannotDiscardConstructedResult(t *testing.T) {
	cases := []string{
		"word choose() -> Result<Int, Error>\n    for n in range(0, 1)\n        err(error.new(\"lost\", \"lost\"))\n    ok(7)\n",
		"word choose() -> Result<Int, Error>\n    for line in file.stream(\"x\")\n        ok(1)\n    ok(7)\n",
	}
	for _, source := range cases {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("loop discarded Result: %q", source)
		}
	}
}

func TestLoopMayReturnResultFromWord(t *testing.T) {
	source := "word choose() -> Result<Int, Error>\n    for n in range(0, 1)\n        return ok(n)\n    ok(7)\nprint(choose() try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil || out.String() != "0\n" {
		t.Fatalf("error %v, output %q", err, out.String())
	}
}

func TestFileFailureDetailsDescribeReadReason(t *testing.T) {
	path := writeFile(t, string([]byte{0xff}))
	source := "let text = file.read(\"" + path + "\") catch problem\n    print(error.detail(problem, \"reason\"))\n    print(error.detail(problem, \"path\"))\n    \"\"\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "invalid-utf8\n"+path+"\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestFileReadLineFailureReasons(t *testing.T) {
	path := writeFile(t, "")
	source := "let f = file.open(\"" + path + "\") try\nlet first = file.read-line(f) catch problem\n    print(error.detail(problem, \"reason\"))\n    \"\"\nfile.close(f)\nlet next = file.read-line(f) catch problem\n    print(error.detail(problem, \"reason\"))\n    \"\"\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "end-of-file\nclosed\n" {
		t.Fatalf("output = %q", out.String())
	}
}
