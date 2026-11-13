package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestoPageUsesMarkdownSource(t *testing.T) {
	root := siteFixture(t)
	source := "# Our manifesto\r\n\r\nAn edited introduction.\r\n\r\n## Words & values\r\n\r\nKeep <script> as text.\r\nContinue this paragraph.\r\n\r\nAnother paragraph.\r\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "manifesto.md"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(root, "output", "manifesto.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Our manifesto", "An edited introduction.", "Words &amp; values", "<p>Keep &lt;script&gt; as text. Continue this paragraph.</p>", "<p>Another paragraph.</p>"} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("missing %q in %s", want, page)
		}
	}
	download, err := os.ReadFile(filepath.Join(root, "output", "manifesto.md"))
	if err != nil || string(download) != source {
		t.Fatalf("source download differs: %v", err)
	}
}

func TestManifestoRejectsIncompleteOrUnsupportedContent(t *testing.T) {
	for _, source := range []string{"", "# Title", "# Title\n\nIntro", "# Title\n\nIntro\n\n## Empty", "# Title\n\nIntro\n\n## Words\n\n**unsupported**"} {
		if _, err := loadManifesto([]byte(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
