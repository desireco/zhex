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
