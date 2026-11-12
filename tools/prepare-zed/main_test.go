package main

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareLocalExtension(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	parser := filepath.Join(root, "editors/tree-sitter-zhex/src/parser.c")
	write(t, parser, "/* generated parser */\n")
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "--quiet")
	git("add", ".")
	git("commit", "--quiet", "-m", "fixture")
	revision := git("rev-parse", "HEAD")
	manifest := "repository = \"https://github.com/desireco/zhex\"\n\n[grammars.zhex]\nrepository = \"https://github.com/desireco/zhex\"\nrev = \"" + revision + "\"\npath = \"editors/tree-sitter-zhex\"\n"
	write(t, filepath.Join(root, "editors/zed/extension.toml"), manifest)
	write(t, filepath.Join(root, "editors/zed/languages/zhex/config.toml"), "name = \"Zhex\"\n")
	write(t, filepath.Join(root, "editors/zed/languages/zhex/highlights.scm"), "(identifier) @variable\n")
	output := filepath.Join(root, "dist/editors/zed")
	for i := 0; i < 2; i++ {
		if err := prepare(root, output); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(output, "extension.toml"))
	if err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	if !strings.Contains(string(data), "repository = \""+uri+"\"") {
		t.Fatalf("missing local URI: %s", data)
	}
	if strings.Count(string(data), "https://github.com/desireco/zhex") != 1 || !strings.Contains(string(data), revision) {
		t.Fatalf("metadata or pin changed: %s", data)
	}
	for _, file := range []string{"config.toml", "highlights.scm"} {
		source, err := os.ReadFile(filepath.Join(root, "editors/zed/languages/zhex", file))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(filepath.Join(output, "languages/zhex", file))
		if err != nil || string(copied) != string(source) {
			t.Fatalf("copy %s: %v", file, err)
		}
	}
	write(t, filepath.Join(root, "editors/zed/extension.toml"), strings.Replace(manifest, revision, strings.Repeat("0", 40), 1))
	badOutput := filepath.Join(root, "invalid-output")
	if err := prepare(root, badOutput); err == nil || !strings.Contains(err.Error(), "pinned grammar") {
		t.Fatalf("wanted revision error, got %v", err)
	}
	if _, err := os.Stat(badOutput); !os.IsNotExist(err) {
		t.Fatal("invalid revision created output")
	}
}

func TestLocalManifestRejectsIncompleteGrammar(t *testing.T) {
	for _, source := range []string{"", "[grammars.zhex]\nrepository = \"remote\"\n", "[grammars.zhex]\nrepository = \"remote\"\nrev = \"main\"\n"} {
		if _, _, err := localManifest(source, "file:///checkout"); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
