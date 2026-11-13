package main

import "testing"

func TestManifestoRejectsIncompleteOrUnsupportedContent(t *testing.T) {
	for _, source := range []string{"", "# Title", "# Title\n\nIntro", "# Title\n\nIntro\n\n## Empty", "# Title\n\nIntro\n\n## Words\n\n**unsupported**"} {
		if _, err := loadManifesto([]byte(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
