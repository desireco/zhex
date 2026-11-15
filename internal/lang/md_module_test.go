package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mdModuleSource loads the shipped md report module so the tests exercise
// the corpus file itself, not a copy.
func mdModuleSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "examples", "modules", "md-report", "md.zhex"))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

// runMD compiles and runs a program that uses md, returning its output.
func runMD(t *testing.T, source string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := RunWithModulesOptions("main.zhex", source, strings.NewReader(""), &out, RunOptions{}, func(name string) (ModuleSource, bool) {
		if name != "md" {
			return ModuleSource{}, false
		}
		return ModuleSource{Path: "md.zhex", Source: mdModuleSource(t)}, true
	})
	return out.String(), err
}

func TestMdModuleEscapingMatrix(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"heading escapes text", `print(md.heading(2, "Status & Checks"))`, "## Status \\& Checks\n"},
		{"heading clamps below one", `print(md.heading(0, "Top"))`, "# Top\n"},
		{"heading clamps above six", `print(md.heading(9, "Deep"))`, "###### Deep\n"},
		{"paragraph is verbatim apart from escaping", `print("[" + md.paragraph("  padded  ") + "]")`, "[  padded  ]\n"},
		{"paragraph keeps unicode", `print(md.paragraph("café — naïve → ok"))`, "café — naïve → ok\n"},
		{"paragraph escapes quotes and backslash", `print(md.paragraph("say \"hi\" \\ done"))`, "say \\\"hi\\\" \\\\ done\n"},
		{"paragraph escapes angle brackets and ampersand", `print(md.paragraph("<b> & </b>"))`, "\\<b\\> \\& \\<\\/b\\>\n"},
		{"empty paragraph is the empty fragment", `print("[" + md.paragraph("") + "]")`, "[]\n"},
		{"bullet list", "print(md.bullet-list([\"one\", \"two\"]))", "- one\n- two\n"},
		{"bullet list escapes metacharacters", `print(md.bullet-list(["under_score"]))`, "- under\\_score\n"},
		{"empty bullet list is the empty fragment", `print("[" + md.bullet-list([]) + "]")`, "[]\n"},
		{"ordered list", "print(md.ordered-list([\"first\", \"second\"]))", "1. first\n2. second\n"},
		{"ordered list escapes metacharacters", `print(md.ordered-list(["1. not a heading"]))`, "1. 1\\. not a heading\n"},
		{"link escapes text", `print(md.link("A (B) [C]", "https://example.org/x"))`, "[A \\(B\\) \\[C\\]](https://example.org/x)\n"},
		{"link encodes spaces and parentheses", `print(md.link("docs", "https://example.org/a b(c)"))`, "[docs](https://example.org/a%20b%28c%29)\n"},
		{"link neutralizes javascript scheme", `print(md.link("click", "javascript:alert(1)"))`, "[click](javascript%3Aalert%281%29)\n"},
		{"link neutralizes data scheme case-insensitively", `print(md.link("payload", "DATA:text/plain, hi"))`, "[payload](DATA%3Atext/plain,%20hi)\n"},
		{"link keeps ordinary scheme colons", `print(md.link("site", "https://example.org:8080/a b"))`, "[site](https://example.org:8080/a%20b)\n"},
		{"code block uses a three fence by default", "print(md.code-block(\"\", [\"let a = 1\", \"`tick` mark\"]))", "```\nlet a = 1\n`tick` mark\n```\n"},
		{"code block escapes nothing inside", `print(md.code-block("zhex", ["\\ * | <b>", "plain"]))`, "```zhex\n\\ * | <b>\nplain\n```\n"},
		{"code block fence beats a triple backtick run", "print(md.code-block(\"\", [\"```raw```\", \"x\"]))", "````\n```raw```\nx\n````\n"},
		{"code block fence beats a multi backtick run", "print(md.code-block(\"\", [\"```` a\", \"`\"]))", "`````\n```` a\n`\n`````\n"},
		{"empty code block", `print("[" + md.code-block("", []) + "]")`, "[```\n\n```]\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runMD(t, "use md\n"+tc.source+"\n")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestMdModuleTable(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"header separator and rows", "print(md.table([\"name\", \"score\"], [[\"Ada\", \"36\"], [\"Grace\", \"9\"]]))",
			"| name  | score |\n| ----- | ----- |\n| Ada   | 36    |\n| Grace | 9     |\n"},
		{"cells escape pipes and width counts the escape", `print(md.table(["a"], [["x|y"]]))`, "| a    |\n| ---- |\n| x\\|y |\n"},
		{"rows only keeps the separator short", `print(md.table(["only"], []))`, "| only |\n| ---- |\n"},
		{"unicode cells pad by code points", "print(md.table([\"k\"], [[\"café\"]]))", "| k    |\n| ---- |\n| café |\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runMD(t, "use md\n"+tc.source+"\n")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// TestMdModuleTableRowsMustMatchHeaders pins the strict uneven-row policy:
// a row shorter or longer than the header width fails list-range before
// any table renders.
func TestMdModuleTableRowsMustMatchHeaders(t *testing.T) {
	for name, source := range map[string]string{
		"short row": `print(md.table(["a", "b"], [["only"]]))`,
		"long row":  `print(md.table(["a", "b"], [["x", "y", "z"]]))`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runMD(t, "use md\n"+source+"\n")
			if err == nil {
				t.Fatalf("program succeeded with output %q", out)
			}
			if code, _ := codeOf(t, err); code != "list-range" {
				t.Fatalf("wanted list-range, got %s: %v", code, err)
			}
			if out != "" {
				t.Fatalf("partial output %q", out)
			}
		})
	}
}

// TestMdModuleLeavesBuffersToTheCaller pins the fragment contract: md
// words return Strings and never touch the caller's Buffer, while two
// names still observe one buffer.
func TestMdModuleLeavesBuffersToTheCaller(t *testing.T) {
	got, err := runMD(t, `use md
var report = buffer.new()
var alias = report
buffer.add-line(report, md.heading(2, "One"))
buffer.add-line(alias, md.heading(3, "Two"))
print(buffer.to-text(report))
`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "## One\n### Two\n"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// TestMdModuleFragmentsComposeWithCallerBlankLines pins the documented
// composition: fragments joined by the caller with blank lines, and no
// fragments for empty input.
func TestMdModuleFragmentsComposeWithCallerBlankLines(t *testing.T) {
	got, err := runMD(t, `use md
var parts: List<String> = list.new()
list.append(parts, md.heading(1, "Report"))
list.append(parts, md.paragraph("Body."))
print(text.join(parts, "\n\n"))
`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Report\n\nBody\\.\n"; got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}
