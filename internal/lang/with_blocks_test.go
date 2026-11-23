package lang

import (
	"bytes"
	"strings"
	"testing"
)

// lineOf reports the 1-based line of the first occurrence of substr.
func lineOf(source, substr string) int {
	before := source[:strings.Index(source, substr)]
	return strings.Count(before, "\n") + 1
}

// ---------------------------------------------------------------- explicit form

func TestWithBlockFeedsEveryStepTheOriginalSeed(t *testing.T) {
	source := "let name = \"zhex\"\nlet shout = flow name\nwith text\n    upper\n    length\n    slice 1, 3\nprint(shout)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "he\n" {
		t.Fatal(out.String())
	}
}

func TestWithBlockValueIsTheLastStepValue(t *testing.T) {
	source := "let n = flow [1, 2]\nwith list\n    append(3)\n    length\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// append mutates the shared seed list and returns it, but its value is
	// discarded — the block's value is the last step's value.
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestWithStepsMutateTheSeedThroughSharedStorage(t *testing.T) {
	source := "let values = [1, 2]\nflow values\nwith list\n    append(3)\n    length\nprint(list.length(values))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

// --------------------------------------------------------------------- the sugar

func TestWithSugarDropsTheFlowKeywordOnTheLineAbove(t *testing.T) {
	source := "let name = \"zhex\"\nname\nwith text\n    upper\n    length\n    slice 1, 3\n    print\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// print falls back to an ordinary word step and receives the seed.
	if out.String() != "zhex\n" {
		t.Fatal(out.String())
	}
}

func TestWithSugarFusesLetAssignmentAndPlainStatements(t *testing.T) {
	source := "let values = [1, 2]\nlet kept = values\nwith list\n    append(3)\n    length\nvar total = 0\ntotal = values\nwith list\n    length\nprint(kept)\nprint(total)\nvalues\nwith list\n    length\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n3\n" {
		t.Fatal(out.String())
	}
}

func TestNakedExpressionElsewhereStaysADiscardedNoOp(t *testing.T) {
	source := "let values = [1, 2]\nvalues\nprint(list.length(values))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestWithFuseRequiresTheImmediatelyNextLine(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"blank line breaks the fuse", "let values = [1, 2]\nvalues\n\nwith list\n    length\n"},
		{"comment line breaks the fuse", "let values = [1, 2]\nvalues\n# a comment\nwith list\n    length\n"},
		{"explicit flow needs adjacency too", "let values = [1, 2]\nflow values\n\nwith list\n    length\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile("test.zhex", tc.source)
			if err == nil {
				t.Fatal("expected a with-location error")
			}
			d, ok := err.(*diagnostic)
			if !ok || d.code != CodeWithLocation {
				t.Fatalf("got %v", err)
			}
			if d.pos.line != lineOf(tc.source, "with") {
				t.Fatalf("error at line %d, want the with line:\n%v", d.pos.line, err)
			}
		})
	}
}

func TestOrphanWithErrorsAtTheWithLine(t *testing.T) {
	// A let line fuses, so an orphan with is one with no statement line
	// above it at all: the first line of a file or of a word body.
	for _, source := range []string{
		"with list\n    length\n",
		"word broken() -> Int\n    with list\n        length\n",
	} {
		_, err := Compile("test.zhex", source)
		if err == nil {
			t.Fatalf("expected a with-location error for %q", source)
		}
		d, ok := err.(*diagnostic)
		if !ok || d.code != CodeWithLocation || d.pos.line != lineOf(source, "with") {
			t.Fatalf("got %v", err)
		}
		if !strings.Contains(err.Error(), "hint:") {
			t.Fatalf("want a hint naming the fix:\n%v", err)
		}
		// A REPL user cannot put with directly below a bare seed line — the
		// seed line submits as its own entry — so the hint must name the
		// explicit flow form.
		if !strings.Contains(err.Error(), "write the explicit `flow` form") {
			t.Fatalf("want the REPL clause naming the flow form:\n%v", err)
		}
	}
}

func TestWithIsAReservedKeyword(t *testing.T) {
	if _, err := Compile("test.zhex", "let with = 3\nprint(with)\n"); err == nil {
		t.Fatal("with must not be available as a binding name")
	}
}

// ------------------------------------------------------------------ step typing

func TestWithStepsResolveNamespaceFirstThenOrdinaryWords(t *testing.T) {
	source := "word length(raw: String) -> String\n    raw\nlet n = flow [1, 2, 3]\nwith list\n    length\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// list.length wins over the same-named user word: the seed is a List, so
	// the host word's Int result is what the block produces.
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestWithStepFallsBackToOrdinaryWordsWithTheSeed(t *testing.T) {
	source := "word shout(raw: String) -> String\n    text.upper(raw)\nlet loud = flow \"zhex\"\nwith text\n    shout\nprint(loud)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// text.shout does not resolve, so the step is the ordinary word shout(seed).
	if out.String() != "ZHEX\n" {
		t.Fatal(out.String())
	}
}

func TestWithDottedStepNamesKeepTheirQualifiedName(t *testing.T) {
	source := "let n = flow [1, 2]\nwith text\n    list.length\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// text.list.length resolves nowhere, so the written name stands as-is.
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestWithStepCallShapesKeepExtraArgumentsAfterTheSeed(t *testing.T) {
	source := "word mark(n: Int) -> String\n    print(n)\n    int.to-string(n)\nlet replaced = flow \"x2x\"\nwith text\n    replace(mark(2), mark(3))\nprint(replaced)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	// Extra arguments evaluate once, left to right, after the seed.
	if out.String() != "2\n3\nx3x\n" {
		t.Fatal(out.String())
	}
}

func TestWithStepResolvingNowhereNamesBothTries(t *testing.T) {
	source := "flow [1, 2]\nwith list\n    frobnicate\n"
	_, err := Compile("test.zhex", source)
	if err == nil {
		t.Fatal("expected a with-step error")
	}
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeWithStep {
		t.Fatalf("got %v", err)
	}
	for _, want := range []string{"list.frobnicate", `"frobnicate"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must narrate %q:\n%v", want, err)
		}
	}
	if d.pos.line != 3 {
		t.Fatalf("error at line %d, want the step line", d.pos.line)
	}
}

func TestWithModuleNamespaceResolvesThroughTheUsedModule(t *testing.T) {
	entry := "use math\nlet n = flow 48\nwith math\n    gcd 18\nprint(n)\n"
	resolve := func(name string) (ModuleSource, bool) {
		if name != "math" {
			return ModuleSource{}, false
		}
		return ModuleSource{Path: "math.zhex", Source: "word gcd(a: Int, b: Int) -> Int\n    if b == 0\n        a\n    else\n        gcd(b, a % b)\n"}, true
	}
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", entry, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "6\n" {
		t.Fatal(out.String())
	}
}

func TestWithPinsTheSeedTypeForEveryStep(t *testing.T) {
	// A threaded flow would hand text.length's Int to text.upper and fail;
	// a with block hands every step the String seed, so this checks.
	source := "let n = flow \"zhex\"\nwith text\n    length\n    upper\n    length\nprint(n)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestWithSeedResultMustBeHandled(t *testing.T) {
	source := "flow read-line(\"N: \")\nwith text\n    upper\n    print\n"
	_, err := Compile("test.zhex", source)
	if err == nil {
		t.Fatal("expected an unhandled-result error")
	}
	if !strings.Contains(err.Error(), "must be handled") {
		t.Fatalf("got %v", err)
	}
}

func TestWithFusedUnitSeedReportsAtTheStepWithSeedLineHint(t *testing.T) {
	source := "print(1)\nwith text\n    upper\n    print\n"
	_, err := Compile("test.zhex", source)
	if err == nil {
		t.Fatal("expected a with-seed error")
	}
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeWithSeed {
		t.Fatalf("got %v", err)
	}
	if d.pos.line != 3 {
		t.Fatalf("error at line %d, want the first step inside the block", d.pos.line)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("hint must name the seed line:\n%v", err)
	}
}

func TestWithFallibleStepsNeedTry(t *testing.T) {
	source := "flow \"4\"\nwith text\n    to-int\n"
	_, err := Compile("test.zhex", source)
	if err == nil {
		t.Fatal("expected a try-input error")
	}
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeTryInput || d.pos.line != 3 {
		t.Fatalf("got %v", err)
	}
}

func TestWithTryStepsPropagateToTheWordBoundary(t *testing.T) {
	source := "word parse(seed: String) -> Int\n    flow seed\n    with text\n        to-int try\nprint(parse(\"4\") try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestWithBlockInsideAWordBodyAndBranches(t *testing.T) {
	source := "word describe(values: List<Int>) -> Int\n    let n = flow values\n    with list\n        append(9)\n        length\n    n\nlet base = [1, 2]\nif describe(base) == 3\n    print(\"three\")\nelse\n    print(\"other\")\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "three\n" {
		t.Fatal(out.String())
	}
}

// ------------------------------------------------------------------------ trace

func TestWithStepsEmitWithStepTraceEvents(t *testing.T) {
	source := "flow \"zhex\"\nwith text\n    upper\n    length\n    print\n"
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		`{"event":"with-step","word":"text.upper","value":"zhex","file":"test.zhex","line":3}`,
		`{"event":"with-step","word":"text.length","value":"zhex","file":"test.zhex","line":4}`,
		`{"event":"with-step","word":"print","value":"zhex","file":"test.zhex","line":5}`,
	}, "\n")
	if !strings.Contains(trace.String(), want) {
		t.Fatalf("trace events:\n%s", trace.String())
	}
	if out.String() != "zhex\n" {
		t.Fatal(out.String())
	}
}

// ------------------------------------------------------------------------- REPL

func TestReplWithBlockRunsThroughPendingState(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("let values = [1, 2]\n"); err != nil {
		t.Fatal(err)
	}
	// The explicit form arrives as one continuation entry: `flow values`
	// alone and the bare with clause both stay pending until the steps do.
	if _, err := session.Enter("flow values\nwith list\n    append(3)\n    length\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := session.Enter("list.length(values)"); err != nil || echo != "3" {
		t.Fatalf("echo %q err %v", echo, err)
	}
	if _, err := parse("repl", "flow values\n"); err == nil || !IsIncomplete(err) {
		t.Fatal("flow values alone must signal an incomplete block")
	}
	if _, err := parse("repl", "flow values\nwith list\n"); err == nil || !IsIncomplete(err) {
		t.Fatal("the with clause without steps must stay pending")
	}
}
