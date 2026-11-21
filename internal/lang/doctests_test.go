package lang

import (
	"strings"
	"testing"
)

// runDoctests runs one entry against optional sibling modules and returns the
// failures, one per failing doctest.
func runDoctests(t *testing.T, entry string, files map[string]string) []error {
	t.Helper()
	return RunDoctests("main.zhex", entry, strings.NewReader(""), &strings.Builder{}, mapResolver(files))
}

func doctestTexts(errs []error) []string {
	texts := make([]string, len(errs))
	for i, err := range errs {
		texts[i] = err.Error()
	}
	return texts
}

const doctestDouble = `## > double(2)
## 4
## > double(0 - 3)
## -6
word double(n: Int) -> Int
    n + n
`

func TestDoctestsPass(t *testing.T) {
	if got := runDoctests(t, doctestDouble, nil); len(got) != 0 {
		t.Fatalf("passing doctests reported failures: %q", doctestTexts(got))
	}
}

func TestDoctestsRunInModuleScope(t *testing.T) {
	files := map[string]string{"math.zhex": `## > gcd(12, 18)
## 6
## > _square(3)
## 9
word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)
word _square(n: Int) -> Int
    n * n
`}
	// A module's doctests call their words bare, private ones included, and
	// they run when the importing file is doctested.
	if got := runDoctests(t, "use math\n", files); len(got) != 0 {
		t.Fatalf("module doctests reported failures: %q", doctestTexts(got))
	}
	// The entry file's own doctests see the module qualified.
	entry := "use math\n\n## > math.gcd(9, 6)\n## 3\nword helper() -> Int\n    math.gcd(9, 6)\n"
	if got := runDoctests(t, entry, files); len(got) != 0 {
		t.Fatalf("entry doctests reported failures: %q", doctestTexts(got))
	}
}

func TestDoctestMismatchDiagnostic(t *testing.T) {
	got := doctestTexts(runDoctests(t, `## > double(2)
## 5
word double(n: Int) -> Int
    n + n
`, nil))
	if len(got) != 1 {
		t.Fatalf("want one mismatch, got %q", got)
	}
	want := strings.Join([]string{
		`main.zhex:1:1: doctest mismatch in double: expected 5, found 4`,
		`    ## > double(2)`,
		`    ^^^^^^^^^^^^^^`,
		`hint: update the expected output after "## > ", or fix the word`,
		"learn: https://desireco.github.io/zhex/errors.html#doctest-mismatch",
	}, "\n")
	if got[0] != want {
		t.Fatalf("diagnostic:\n%s\nwant:\n%s", got[0], want)
	}
}

// The pinned prose rule: a blank doc line ends an example's expected
// output, so doc lines after it are prose again until the next ## > line;
// prose directly after an example, with no blank line, still reads as
// output and mismatches.
func TestDoctestBlankLineEndsExpectedOutput(t *testing.T) {
	// Prose after a blank line is prose: the example passes.
	source := `## > double(2)
## 4
##
## Doubles its argument.
word double(n: Int) -> Int
    n + n
`
	if got := runDoctests(t, source, nil); len(got) != 0 {
		t.Fatalf("prose after a blank line must not join the expectation: %q", doctestTexts(got))
	}
	// Without the blank line the same prose is expected output and mismatches.
	got := doctestTexts(runDoctests(t, strings.Replace(source, "##\n", "", 1), nil))
	if len(got) != 1 || !strings.Contains(got[0], `expected "4\nDoubles its argument.", found 4`) {
		t.Fatalf("prose directly after an example must mismatch: %q", got)
	}
}

// The mismatch caret and span count columns in runes, matching the lexer.
func TestDoctestMismatchCaretCountsRunes(t *testing.T) {
	errs := runDoctests(t, `## > text.length("héllo")
## 4
word len-doc(s: String) -> Int
    text.length s
`, nil)
	if len(errs) != 1 {
		t.Fatalf("want one mismatch, got %d failures", len(errs))
	}
	got := errs[0].Error()
	want := strings.Join([]string{
		`main.zhex:1:1: doctest mismatch in len-doc: expected 4, found 5`,
		`    ## > text.length("héllo")`,
		`    ` + strings.Repeat("^", 25),
		`hint: update the expected output after "## > ", or fix the word`,
		"learn: https://desireco.github.io/zhex/errors.html#doctest-mismatch",
	}, "\n")
	if got != want {
		t.Fatalf("diagnostic:\n%s\nwant:\n%s", got, want)
	}
	report, ok := Report(errs[0])
	if !ok || !strings.Contains(report, `"span_start_column":1`) || !strings.Contains(report, `"span_end_column":26`) {
		t.Fatalf("span must count runes: %s", report)
	}
}

func TestDoctestFailToCheck(t *testing.T) {
	for _, tc := range []struct {
		name, doc, want string
	}{
		{"type error", `## > double("x")`, `main.zhex:1:6: doctest in double failed to check: argument 1 of double requires type Int, found String`},
		{"unknown word", `## > missing(1)`, `main.zhex:1:6: doctest in double failed to check: unknown word`},
		{"naked result", `## > text.to-int("4")`, `main.zhex:1:6: doctest in double failed to check: Result must be handled or propagated`},
		{"runtime failure", `## > 1 / 0`, `main.zhex:1:8: doctest in double failed to evaluate: division or remainder by zero`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := runDoctests(t, tc.doc+"\nword double(n: Int) -> Int\n    n + n\n", nil)
			if len(errs) != 1 {
				t.Fatalf("want one failure, got %q", doctestTexts(errs))
			}
			if !strings.HasPrefix(errs[0].Error(), tc.want) {
				t.Fatalf("first line %q misses %q", errs[0].Error(), tc.want)
			}
			if !strings.Contains(errs[0].Error(), "\nhint:") {
				t.Fatalf("diagnostic needs a hint: %q", errs[0].Error())
			}
			report, ok := Report(errs[0])
			if !ok || !strings.Contains(report, `"code":"doctest-check"`) {
				t.Fatalf("report must carry doctest-check: %s", report)
			}
		})
	}
}

// A failing check keeps the original hint when one exists, naming the fix.
func TestDoctestFailToCheckKeepsHint(t *testing.T) {
	got := doctestTexts(runDoctests(t, `## > double("x")
word double(n: Int) -> Int
    n + n
`, nil))
	if len(got) != 1 || !strings.Contains(got[0], "type Int, found String") || !strings.Contains(got[0], "\nhint: ") {
		t.Fatalf("diagnostic lost its message or hint: %q", got)
	}
}

func TestDoctestUnitExpectsNoOutput(t *testing.T) {
	if got := runDoctests(t, `## > show()
word show() -> Unit
    print "hi"
`, nil); len(got) != 0 {
		t.Fatalf("Unit echo is empty and must pass: %q", doctestTexts(got))
	}
	got := doctestTexts(runDoctests(t, `## > show()
## hi
word show() -> Unit
    print "hi"
`, nil))
	if len(got) != 1 || !strings.Contains(got[0], "doctest mismatch in show: expected hi, found ") {
		t.Fatalf("Unit echo must not match a nonempty expectation: %q", got)
	}
}

func TestDoctestsReportEveryFailureInOrder(t *testing.T) {
	got := doctestTexts(runDoctests(t, `## > double(1)
## 3
## > double(2)
## 5
## > double("x")
word double(n: Int) -> Int
    n + n
`, nil))
	if len(got) != 3 {
		t.Fatalf("want three failures, got %q", got)
	}
	for i, want := range []string{"expected 3, found 2", "expected 5, found 4", "failed to check"} {
		if !strings.Contains(got[i], want) {
			t.Fatalf("failure %d %q misses %q", i, got[i], want)
		}
	}
}

func TestDoctestsCompileFailurePassesThrough(t *testing.T) {
	got := doctestTexts(runDoctests(t, `## > double(2)
## 4
word double(n: Int) -> Int
    n + n
print(missing(1))
`, nil))
	if len(got) != 1 || !strings.HasPrefix(got[0], "main.zhex:5:7: unknown word") {
		t.Fatalf("compile failure must pass through unchanged: %q", got)
	}
}

func TestDoctestsWithoutExamplesPassSilently(t *testing.T) {
	if got := runDoctests(t, "print 1\n", nil); len(got) != 0 {
		t.Fatalf("a file without doctests must pass: %q", doctestTexts(got))
	}
}

// Plain Run never executes docs: an example that would fail checking or
// evaluating leaves the run itself untouched.
func TestRunNeverExecutesDoctests(t *testing.T) {
	source := `## > double("x")
## > 1 / 0
## > missing(1)
word double(n: Int) -> Int
    n + n
print(double(2))
`
	var out strings.Builder
	if err := Run("main.zhex", source, strings.NewReader(""), &out); err != nil || out.String() != "4\n" {
		t.Fatalf("run must ignore docs: err %v, output %q", err, out.String())
	}
}

func TestDoctestReportsCarryTheDocLineSpan(t *testing.T) {
	errs := runDoctests(t, `## > double(2)
## 5
word double(n: Int) -> Int
    n + n
`, nil)
	if len(errs) != 1 {
		t.Fatalf("want one mismatch, got %d failures", len(errs))
	}
	report, ok := Report(errs[0])
	if !ok {
		t.Fatal("mismatch must be a diagnostic")
	}
	for _, want := range []string{
		`"code":"doctest-mismatch"`, `"severity":"error"`, `"file":"main.zhex"`, `"line":1`, `"column":1`,
		`"span_start_column":1`, `"span_end_column":15`,
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report %s misses %s", report, want)
		}
	}
}

func TestDoctestsTryExampleEvaluatesUnwrapped(t *testing.T) {
	if got := runDoctests(t, `## > text.to-int("42") try
## 42
word unused() -> Unit
    print "never"
`, nil); len(got) != 0 {
		t.Fatalf("a try example evaluates to its success value: %q", doctestTexts(got))
	}
}

// The one-line catch fallback lets a doctest show a word's failing path.
func TestDoctestsCatchExampleEvaluatesFallback(t *testing.T) {
	if got := runDoctests(t, `## > text.to-int("42") catch 0
## 42
## > text.to-int("x") catch 0
## 0
word parse-doc(s: String) -> Int
    text.to-int(s) catch 0
`, nil); len(got) != 0 {
		t.Fatalf("a catch example evaluates to its fallback: %q", doctestTexts(got))
	}
}
