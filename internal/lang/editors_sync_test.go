package lang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// editorFiles are the syntax surfaces that enumerate built-in words; the repo
// root sits two levels above this package.
var editorFiles = struct {
	vim, vscode, treeSitterHighlights, zedHighlights string
}{
	vim:                  filepath.Join("..", "..", "editors", "vim", "syntax", "zhex.vim"),
	vscode:               filepath.Join("..", "..", "editors", "vscode", "syntaxes", "zhex.tmLanguage.json"),
	treeSitterHighlights: filepath.Join("..", "..", "editors", "tree-sitter-zhex", "queries", "highlights.scm"),
	zedHighlights:        filepath.Join("..", "..", "editors", "zed", "languages", "zhex", "highlights.scm"),
}

func hostWordSet(t *testing.T) map[string]bool {
	t.Helper()
	words := make(map[string]bool, len(hostWords))
	for _, word := range hostWords {
		words[word] = true
	}
	return words
}

func assertSameWords(t *testing.T, surface string, listed []string) {
	t.Helper()
	host := hostWordSet(t)
	seen := make(map[string]bool, len(listed))
	var missing, extra []string
	for _, name := range listed {
		if seen[name] {
			t.Errorf("%s: duplicate builtin %q", surface, name)
		}
		seen[name] = true
		if !host[name] {
			extra = append(extra, name)
		}
	}
	for _, name := range hostWords {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		t.Errorf("%s: builtin words drifted from internal/lang hostWords (add them to the editor's word list): missing %v, extra %v", surface, missing, extra)
	}
}

// TestVimBuiltinsMatchHostWords pins the vim syntax keyword list to hostWords.
func TestVimBuiltinsMatchHostWords(t *testing.T) {
	source, err := os.ReadFile(editorFiles.vim)
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`(?m)^syntax keyword zhexBuiltin (.*)$`).FindSubmatch(source)
	if line == nil {
		t.Fatal("no `syntax keyword zhexBuiltin` line in editors/vim/syntax/zhex.vim")
	}
	assertSameWords(t, "editors/vim/syntax/zhex.vim", strings.Fields(string(line[1])))
}

// TestVSCodeBuiltinsMatchHostWords pins the tmLanguage builtin pattern to
// hostWords. The pattern is a flat alternation plus `ns.(?:a|b)` namespace
// groups; both spellings decode to the same dotted names.
func TestVSCodeBuiltinsMatchHostWords(t *testing.T) {
	raw, err := os.ReadFile(editorFiles.vscode)
	if err != nil {
		t.Fatal(err)
	}
	var grammar struct {
		Patterns []struct {
			Match string `json:"match"`
			Name  string `json:"name"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(raw, &grammar); err != nil {
		t.Fatal(err)
	}
	pattern := ""
	for _, p := range grammar.Patterns {
		if p.Name == "support.function.builtin.zhex" {
			pattern = p.Match
		}
	}
	if pattern == "" {
		t.Fatal("no support.function.builtin.zhex pattern in editors/vscode/syntaxes/zhex.tmLanguage.json")
	}
	const prefix, suffix = "(?<![A-Za-z0-9_.-])(?:", ")(?![A-Za-z0-9_.-])"
	if !strings.HasPrefix(pattern, prefix) || !strings.HasSuffix(pattern, suffix) {
		t.Fatalf("unexpected builtin pattern shape: %s", pattern)
	}
	var listed []string
	for _, part := range splitAlternation(pattern[len(prefix) : len(pattern)-len(suffix)]) {
		decoded := strings.ReplaceAll(part, `\.`, ".")
		if ns, inner, ok := strings.Cut(decoded, ".(?:"); ok && strings.HasSuffix(inner, ")") {
			for _, word := range strings.Split(inner[:len(inner)-1], "|") {
				listed = append(listed, ns+"."+word)
			}
			continue
		}
		listed = append(listed, decoded)
	}
	assertSameWords(t, "editors/vscode/syntaxes/zhex.tmLanguage.json", listed)
}

// splitAlternation splits a regex alternation on top-level `|`, honoring
// parentheses and backslash escapes.
func splitAlternation(pattern string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				parts = append(parts, pattern[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, pattern[start:])
}

// TestHighlightQueriesBuiltinLiteralsAreHostWords covers the tree-sitter and
// Zed queries: they highlight calls structurally, so their only builtin-word
// literals are the words the grammar itself names. Each such literal must
// still be a registered host word, and the two query files must stay
// identical (CI compares them byte for byte).
func TestHighlightQueriesBuiltinLiteralsAreHostWords(t *testing.T) {
	treeSitter, err := os.ReadFile(editorFiles.treeSitterHighlights)
	if err != nil {
		t.Fatal(err)
	}
	zed, err := os.ReadFile(editorFiles.zedHighlights)
	if err != nil {
		t.Fatal(err)
	}
	if string(treeSitter) != string(zed) {
		t.Error("editors/tree-sitter-zhex/queries/highlights.scm and editors/zed/languages/zhex/highlights.scm differ")
	}
	host := hostWordSet(t)
	literal := regexp.MustCompile(`"([A-Za-z0-9_.-]+)"`)
	for _, line := range strings.Split(string(treeSitter), "\n") {
		if !strings.HasSuffix(line, "@function") {
			continue
		}
		for _, word := range literal.FindAllStringSubmatch(line, -1) {
			if !host[word[1]] {
				t.Errorf("highlights.scm marks %q @function but it is not a host word", word[1])
			}
		}
	}
}
