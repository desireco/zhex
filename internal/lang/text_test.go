package lang

import (
	"strings"
	"testing"
)

func TestTextWords(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"length counts code points", "print(text.length(\"héllo\"))\n", "5\n"},
		{"line-count", "print(text.line-count(\"\"))\nprint(text.line-count(\"a\"))\nprint(text.line-count(\"a\\n\"))\nprint(text.line-count(\"a\\nb\"))\n", "0\n1\n1\n2\n"},
		{"word-count splits on Unicode whitespace", "print(text.word-count(\"  a\\u00a0b\\tc \"))\n", "3\n"},
		{"upper maps Unicode", "print(text.upper(\"éà\"))\n", "ÉÀ\n"},
		{"lower maps Unicode", "print(text.lower(\"ÉÀ\"))\n", "éà\n"},
		{"trim cuts Unicode whitespace on both ends", "print(\"[\" + text.trim(\"\\u00a0 \\t hi \\n\\u00a0\") + \"]\")\n", "[hi]\n"},
		{"contains", "print(text.contains(\"hello\", \"ell\"))\nprint(text.contains(\"hello\", \"xyz\"))\n", "true\nfalse\n"},
		{"index-of reports the first occurrence in code points", "print(text.index-of(\"héllo\", \"l\"))\n", "2\n"},
		{"index-of absent is -1", "print(text.index-of(\"héllo\", \"z\"))\n", "-1\n"},
		{"slice is empty when start is at or past end", "print(text.slice(\"hello\", 3, 2))\nprint(text.slice(\"hello\", 2, 2))\n", "\n\n"},
		{"slice clamps below zero and past the length", "print(text.slice(\"hello\", -2, 99))\n", "hello\n"},
		{"slice is code-point indexed", "print(text.slice(\"héllo\", 1, 3))\n", "él\n"},
		{"replace rewrites every occurrence", "print(text.replace(\"a-b-c\", \"-\", \"+\"))\n", "a+b+c\n"},
		{"replace is literal, not a pattern", "print(text.replace(\"a.b.c\", \".\", \"!\"))\n", "a!b!c\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runText(t, tc.source)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReplaceEmptyIsABugSignal pins the hard error: an empty old text has no
// sane result, so the run exits with replace-empty — stable first line, no
// hint, because there is nothing to suggest but a nonempty old text.
func TestReplaceEmptyIsABugSignal(t *testing.T) {
	err := Run("bad.zhex", "print(text.replace(\"abc\", \"\", \"x\"))\n", strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.HasPrefix(err.Error(), "bad.zhex:1:7: text.replace old text is empty") {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if strings.Contains(err.Error(), "hint:") {
		t.Fatalf("replace-empty must carry no hint: %q", err.Error())
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"replace-empty"`) {
		t.Fatalf("wanted code replace-empty, got %s", report)
	}
}

func runText(t *testing.T, source string) string {
	t.Helper()
	var out strings.Builder
	if err := Run("text.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
