package zhexrt

import (
	"os"
	"strings"
	"testing"
)

// TestFilesCoversEveryRuntimeSource pins the embed manifest: every
// non-test, non-platform .go file beside this test must appear in Files(),
// because zhex compile --target=native copies exactly that set into
// generated programs — an unlisted file would silently ship a smaller
// runtime than the interpreter's.
func TestFilesCoversEveryRuntimeSource(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := Files()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "embed.go" {
			continue
		}
		if files[name] == "" {
			t.Errorf("%s is not in the embed manifest (embed.go's //go:embed list)", name)
		}
	}
	for name := range files {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("Files() reports %s, which does not exist", name)
		}
	}
}
