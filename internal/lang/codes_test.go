package lang

import (
	"encoding/json"
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
	for _, tc := range []struct{ code, source, input string }{
		{"source-utf8", "\xff print(1)\n", ""},
		{"tab-character", "\tprint(1)\n", ""},
		{"indentation", "     print(1)\n", ""},
		{"unterminated-string", "print(\"abc\n", ""},
		{"string-escape", "print(\"a\\q\")\n", ""},
		{"unexpected-character", "print(1) $\n", ""},
		{"expected-token", "let = 1\n", ""},
		{"expected-name", "word if(a: Int) -> Int\n    a\n", ""},
		{"unknown-type", "word f(a: Bogus) -> Int\n    1\n", ""},
		{"expected-indent", "if true\nprint(1)\n", ""},
		{"nesting-limit", strings.Repeat("(", 260) + "1" + strings.Repeat(")", 260) + "\n", ""},
		{"integer-literal-range", "print(99999999999999999999)\n", ""},
		{"expected-expression", "let x =\n", ""},
		{"unknown-name", "print(x)\n", ""},
		{"duplicate-name", "let a = 1\nlet a = 2\n", ""},
		{"duplicate-word", "word f() -> Unit\n    print(1)\nword f() -> Unit\n    print(2)\n", ""},
		{"word-result", "word f() -> Int\n    true\n", ""},
		{"immutable-name", "let a = 1\na = 2\n", ""},
		{"assignment-type", "var a = 1\na = \"x\"\n", ""},
		{"condition-type", "if 1\n    print(1)\n", ""},
		{"branch-type", "if true\n    1\nelse\n    \"x\"\n", ""},
		{"range-type", "for n in range(1, \"x\")\n    print(n)\n", ""},
		{"try-input", "let x = 1 try\nprint(x)\n", ""},
		{"unary-type", "print(not 1)\n", ""},
		{"operand-types", "print(1 + \"a\")\n", ""},
		{"bool-operand", "print(1 and 2)\n", ""},
		{"compare-type", "if print(1) == print(2)\n    print(3)\n", ""},
		{"ordering-type", "print(\"a\" < \"b\")\n", ""},
		{"addition-type", "print(print(1) + print(2))\n", ""},
		{"arithmetic-type", "print(true * true)\n", ""},
		{"argument-count", "print(1, 2)\n", ""},
		{"argument-type", "print(int.to-string(\"x\"))\n", ""},
		{"unknown-word", "prnt(\"x\")\n", ""},
		{"print-type", "print(print(1))\n", ""},
		{"range-context", "let r = range(1, 2)\n", ""},
		{"integer-overflow", "print(9223372036854775807 + 1)\n", ""},
		{"division-zero", "print(1 / 0)\n", ""},
		{"depth-limit", "word spin(n: Int) -> Int\n    spin(n)\nprint(spin(1))\n", ""},
		{"console-input", "let n = read-line(\"N: \") try\nprint(n)\n", ""},
		{"console-utf8", "let n = read-line(\"N: \") try\nprint(n)\n", "\xff\n"},
		{"text-to-int", "let n = text.to-int(\"abc\") try\nprint(n)\n", ""},
	} {
		t.Run(tc.code, func(t *testing.T) {
			err := Run("bad.zhex", tc.source, strings.NewReader(tc.input), &strings.Builder{})
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
