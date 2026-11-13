package lang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errorString string

func (e errorString) Error() string { return string(e) }

func catalogSections(t *testing.T) map[string]string {
	t.Helper()
	sections := map[string]string{}
	for i, section := range strings.Split(strings.TrimSpace(catalogMarkdown), "\n## ") {
		if i == 0 {
			continue
		}
		code, text, _ := strings.Cut(section, "\n")
		sections[strings.TrimSpace(code)] = strings.TrimSpace(text)
	}
	if len(sections) == 0 {
		t.Fatal("catalog has no sections")
	}
	return sections
}

func TestEveryCodeHasACatalogSectionAndViceVersa(t *testing.T) {
	sections := catalogSections(t)
	for _, code := range diagnosticCodes {
		if text, ok := sections[string(code)]; !ok || text == "" {
			t.Errorf("code %q lacks a catalog section with prose", code)
		}
	}
	known := map[string]bool{}
	for _, code := range diagnosticCodes {
		known[string(code)] = true
	}
	for code := range sections {
		if !known[code] {
			t.Errorf("catalog section %q has no code", code)
		}
	}
}

func TestDiagnosticCodesAreSortedAndUnique(t *testing.T) {
	codes := DiagnosticCodes()
	seen := map[string]bool{}
	for i, code := range codes {
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
		if i > 0 && codes[i-1] > code {
			t.Fatalf("codes not sorted: %q before %q", codes[i-1], code)
		}
	}
}

// TestRaiseSitesUseTheirCodes pins which code each raise site produces, so
// a future edit cannot swap codes at a site while catalog and tests stay
// green. unsupported-code and console-output are internal/failure-injection
// paths with no source-level trigger; empty-block and flow-step are
// defensive invariants — blank and comment-only lines never emit an indent
// token, so a parsed block always has a statement, and an empty flow would
// otherwise check as identity. All four are covered by construction.

func TestRaiseSitesUseTheirCodes(t *testing.T) {
	// The file codes need real fixtures: a bad-UTF-8 file and a drained one.
	utf8Path := filepath.Join(t.TempDir(), "bad.txt")
	if err := os.WriteFile(utf8Path, []byte{'a', 0xff}, 0o644); err != nil {
		t.Fatal(err)
	}
	eofPath := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(eofPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code, source, input string
		args                []string
		resolve             func(string) (ModuleSource, bool) // set: compile main.zhex against these modules
	}{
		{"error-no-cause", "word fail() -> Result<Int, Error>\n    return err(error.new(\"domain\", \"message\"))\nlet n = fail() catch problem\n    let cause = error.cause(problem) try\n    0\n", "", nil, nil},
		{"source-utf8", "\xff print(1)\n", "", nil, nil},
		{"tab-character", "\tprint(1)\n", "", nil, nil},
		{"indentation", "     print(1)\n", "", nil, nil},
		{"unterminated-string", "print(\"abc\n", "", nil, nil},
		{"string-escape", "print(\"a\\q\")\n", "", nil, nil},
		{"unexpected-character", "print(1) $\n", "", nil, nil},
		{"expected-token", "let = 1\n", "", nil, nil},
		{"expected-name", "word if(a: Int) -> Int\n    a\n", "", nil, nil},
		{"unknown-type", "word f(a: Bogus) -> Int\n    1\n", "", nil, nil},
		{"expected-indent", "if true\nprint(1)\n", "", nil, nil},
		{"nesting-limit", strings.Repeat("(", 260) + "1" + strings.Repeat(")", 260) + "\n", "", nil, nil},
		{"integer-literal-range", "print(99999999999999999999)\n", "", nil, nil},
		{"expected-expression", "let x =\n", "", nil, nil},
		{"unknown-name", "print(x)\n", "", nil, nil},
		{"duplicate-name", "let a = 1\nlet a = 2\n", "", nil, nil},
		{"duplicate-word", "word f() -> Unit\n    print(1)\nword f() -> Unit\n    print(2)\n", "", nil, nil},
		{"word-result", "word f() -> Int\n    true\n", "", nil, nil},
		{"immutable-name", "let a = 1\na = 2\n", "", nil, nil},
		{"assignment-type", "var a = 1\na = \"x\"\n", "", nil, nil},
		{"condition-type", "if 1\n    print(1)\n", "", nil, nil},
		{"branch-type", "if true\n    1\nelse\n    \"x\"\n", "", nil, nil},
		{"range-type", "for n in range(1, \"x\")\n    print(n)\n", "", nil, nil},
		{"try-input", "let x = 1 try\nprint(x)\n", "", nil, nil},
		{"unary-type", "print(not 1)\n", "", nil, nil},
		{"operand-types", "print(1 + \"a\")\n", "", nil, nil},
		{"bool-operand", "print(1 and 2)\n", "", nil, nil},
		{"compare-type", "if print(1) == print(2)\n    print(3)\n", "", nil, nil},
		{"ordering-type", "print(\"a\" < \"b\")\n", "", nil, nil},
		{"addition-type", "print(print(1) + print(2))\n", "", nil, nil},
		{"arithmetic-type", "print(true * true)\n", "", nil, nil},
		{"argument-count", "print(1, 2)\n", "", nil, nil},
		{"argument-type", "print(int.to-string(\"x\"))\n", "", nil, nil},
		{"unknown-word", "prnt(\"x\")\n", "", nil, nil},
		{"print-type", "print(print(1))\n", "", nil, nil},
		{"range-context", "let r = range(1, 2)\n", "", nil, nil},
		{"range-context", "let x = file.lines(\"a\")\n", "", nil, nil},
		{"range-type", "for line in file.lines(1)\n    print(line)\n", "", nil, nil},
		{"integer-overflow", "print(9223372036854775807 + 1)\n", "", nil, nil},
		{"division-zero", "print(1 / 0)\n", "", nil, nil},
		{"depth-limit", "word spin(n: Int) -> Int\n    spin(n)\nprint(spin(1))\n", "", nil, nil},
		{"console-input", "let n = read-line(\"N: \") try\nprint(n)\n", "", nil, nil},
		{"console-utf8", "let n = read-line(\"N: \") try\nprint(n)\n", "\xff\n", nil, nil},
		{"text-to-int", "let n = text.to-int(\"abc\") try\nprint(n)\n", "", nil, nil},
		{"replace-empty", "print(text.replace(\"a\", \"\", \"b\"))\n", "", nil, nil},
		{"argument-missing", "print(console.arg(0) try)\n", "", nil, nil},
		{"buffer-range", "let s = buffer.line(buffer.new(), 0) try\n", "", nil, nil},
		{"file-open", "let t = file.read-all(\"/nonexistent-zhex\") try\n", "", nil, nil},
		{"file-utf8", "let t = file.read-all(\"" + utf8Path + "\") try\n", "", nil, nil},
		{"file-read", "let f = file.open(\"" + eofPath + "\") try\nlet l = file.read-line(f) try\n", "", nil, nil},
		{code: "use-location", source: "word f() -> Unit\n    use math\n    print(1)\n"},
		{code: "use-missing", source: "use math\nprint(1)\n"},
		{code: "use-reserved", source: "use text\n"},
		{code: "use-cycle", source: "use math\n", resolve: mapResolver(map[string]string{"math.zhex": "use math\n" + mathModule})},
		{code: "module-body", source: "use math\n", resolve: mapResolver(map[string]string{"math.zhex": "print(1)\n"})},
		{code: "private-use", source: "use math\nprint(math._wrap(1))\n", resolve: mapResolver(map[string]string{"math.zhex": privateMathModule})},
	} {
		t.Run(tc.code, func(t *testing.T) {
			err := RunWithArgs("bad.zhex", tc.source, strings.NewReader(tc.input), &strings.Builder{}, nil, tc.args)
			if tc.resolve != nil {
				_, err = CompileModules("main.zhex", tc.source, tc.resolve)
			}
			if err == nil {
				t.Fatalf("expected a %s diagnostic", tc.code)
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"`+tc.code+`"`) {
				t.Fatalf("wanted code %s, got %s", tc.code, report)
			}
		})
	}
}

// TestReportAddsSpanFieldsAdditively pins the additive span fields: static
// diagnostics carry span_start_column/span_end_column; runtime diagnostics
// must stay byte-identical (no span_ fields).
func TestReportAddsSpanFieldsAdditively(t *testing.T) {
	err := Run("bad.zhex", "print(1 + \"a\")\n", strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	report, ok := Report(err)
	if !ok {
		t.Fatal("error is not a diagnostic")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(report), &fields); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	if fields["span_start_column"] != float64(7) || fields["span_end_column"] != float64(14) {
		t.Fatalf("wrong span fields: %s", report)
	}

	runtimeErr := Run("bad.zhex", "print(1 / 0)\n", strings.NewReader(""), &strings.Builder{})
	if runtimeErr == nil {
		t.Fatal("expected an error")
	}
	runtimeReport, ok := Report(runtimeErr)
	if !ok {
		t.Fatal("error is not a diagnostic")
	}
	if strings.Contains(runtimeReport, "span_") {
		t.Fatalf("runtime report must not carry span fields: %s", runtimeReport)
	}
}

func TestReportEmitsThePinnedJSONShape(t *testing.T) {
	err := Run("bad.zhex", "print(a-b)\n", strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	got, ok := Report(err)
	if !ok {
		t.Fatal("error is not a diagnostic")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(got), &fields); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	for _, key := range []string{"code", "severity", "file", "line", "column", "message", "hint"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("report missing %q: %s", key, got)
		}
	}
	if fields["code"] != "unknown-name" || fields["severity"] != "error" || fields["file"] != "bad.zhex" {
		t.Fatalf("wrong identity fields: %s", got)
	}
	if !strings.Contains(fields["hint"].(string), "a - b") {
		t.Fatalf("hint missing the fix: %s", got)
	}
	if _, ok := Report(errorString(err.Error())); ok {
		t.Fatal("plain errors must not report as diagnostics")
	}
}
