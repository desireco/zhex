package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The generated agent surfaces (#194): the per-code llms.txt section and
// skills/diagnostics/SKILL.md both render from the embedded catalog, one
// direction only — a hand-kept second copy drifts (the #191 research's
// Solid #5051 lesson). These gates regenerate both artifacts and fail on
// any difference; `go generate ./internal/lang` rewrites them.
var (
	skillFilePath = filepath.Join("..", "..", "skills", "diagnostics", "SKILL.md")
	llmsFilePath  = filepath.Join("..", "..", "site", "llms.txt")
)

func TestCatalogEntryFirstSentenceFlattensAndSkipsCodeDots(t *testing.T) {
	for _, tc := range []struct {
		name, text, wantSentence, wantFix string
	}{
		{"wrapped prose joins one line", "The list index is negative\nor outside its length. Use a valid\nzero-based index.", "The list index is negative or outside its length.", "Use a valid zero-based index."},
		{"a dotted name before the boundary stays in the sentence", "`file.read` fails when the path is missing. Handle the Result with try or catch.", "`file.read` fails when the path is missing.", "Handle the Result with try or catch."},
		{"the remainder after the first sentence is the fix", "One operand of `+` is not a supported type. Check both operand types.", "One operand of `+` is not a supported type.", "Check both operand types."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := CatalogEntry{Code: "test-code", Text: tc.text}
			if got := entry.FirstSentence(); got != tc.wantSentence {
				t.Errorf("FirstSentence() = %q, want %q", got, tc.wantSentence)
			}
			_, fix := entry.splitMeaningFix()
			if fix != tc.wantFix {
				t.Errorf("splitMeaningFix fix = %q, want %q", fix, tc.wantFix)
			}
		})
	}
}

// TestSkillMarkdownStaysGeneratedFromTheCatalog pins the repair skill: the
// committed file must be byte-identical to what the catalog renders, and
// every code owns exactly one anchored section — no code without a section,
// no section without a code.
func TestSkillMarkdownStaysGeneratedFromTheCatalog(t *testing.T) {
	want := SkillMarkdown()
	got, err := os.ReadFile(skillFilePath)
	if err != nil {
		t.Fatalf("read skills/diagnostics/SKILL.md: %v (regenerate with `go generate ./internal/lang`)", err)
	}
	if string(got) != want {
		t.Error("skills/diagnostics/SKILL.md drifted from the catalog — regenerate with `go generate ./internal/lang`, never edit it by hand")
	}
	known := map[string]bool{"Workflow": true}
	for _, code := range DiagnosticCodes() {
		known[code] = true
		if n := strings.Count(string(got), "\n## "+code+"\n"); n != 1 {
			t.Errorf("code %q has %d anchored sections in the skill, want exactly 1", code, n)
		}
	}
	for _, line := range strings.Split(string(got), "\n") {
		if strings.HasPrefix(line, "## ") && !known[strings.TrimPrefix(line, "## ")] {
			t.Errorf("skill section %q matches no diagnostic code", line)
		}
	}
}

// TestLLMSErrorCodesSectionStaysGeneratedFromTheCatalog pins the llms.txt
// error-codes section: the lines fenced between the generated markers must
// be byte-identical to what the catalog renders, one bullet per code.
func TestLLMSErrorCodesSectionStaysGeneratedFromTheCatalog(t *testing.T) {
	source, err := os.ReadFile(llmsFilePath)
	if err != nil {
		t.Fatalf("read site/llms.txt: %v", err)
	}
	begin := strings.Index(string(source), LLMSCodesFenceBegin)
	end := strings.LastIndex(string(source), LLMSCodesFenceEnd)
	if begin < 0 || end < begin {
		t.Fatalf("site/llms.txt lacks the %s / %s fence around the generated error-codes list", LLMSCodesFenceBegin, LLMSCodesFenceEnd)
	}
	if got := string(source[begin+len(LLMSCodesFenceBegin) : end]); got != "\n"+LLMSErrorCodes() {
		t.Error("the fenced error-codes list in site/llms.txt drifted from the catalog — regenerate with `go generate ./internal/lang`, never edit it by hand")
	}
	codes := DiagnosticCodes()
	bullets := 0
	for _, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(line, "- [") && strings.Contains(line, "errors.html#") {
			bullets++
		}
	}
	if bullets != len(codes) {
		t.Errorf("llms.txt carries %d error-code bullets, want exactly one per code (%d)", bullets, len(codes))
	}
}

// TestSpliceLLMSCodesRewritesOnlyTheFence pins the splice the generator
// runs: everything outside the markers stays byte-for-byte, the fence
// itself is rewritten, and a file without markers is an error rather than a
// silent rewrite.
func TestSpliceLLMSCodesRewritesOnlyTheFence(t *testing.T) {
	docs := "# Zhex\n\n## Docs\n\n- [Docs](https://example.org): hand text.\n"
	fence := LLMSCodesFenceBegin + "\n- [stale](https://example.org): stale line.\n" + LLMSCodesFenceEnd
	got, err := SpliceLLMSCodes(docs + "\n## Error codes\n\n" + fence + "\n\n## Reference\n")
	if err != nil {
		t.Fatal(err)
	}
	want := docs + "\n## Error codes\n\n" + LLMSCodesFenceBegin + "\n" + LLMSErrorCodes() + LLMSCodesFenceEnd + "\n\n## Reference\n"
	if got != want {
		t.Errorf("splice rewrote more than the fence:\ngot  %q\nwant %q", got, want)
	}
	if _, err := SpliceLLMSCodes(docs); err == nil {
		t.Error("a file without the fence must fail the splice, not pass through")
	}
}
