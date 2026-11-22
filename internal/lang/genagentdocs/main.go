// Command genagentdocs regenerates the generated agent surfaces (#194)
// from the embedded catalog: skills/diagnostics/SKILL.md whole, and the
// error-codes list fenced inside site/llms.txt. Run it with `go generate
// ./internal/lang`; agentdocs_test.go fails the suite when either artifact
// drifts, so regeneration is the only way to change them.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zhex/internal/lang"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	skill := filepath.Join(root, "skills", "diagnostics", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(skill, []byte(lang.SkillMarkdown()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	llmsPath := filepath.Join(root, "site", "llms.txt")
	source, err := os.ReadFile(llmsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	spliced, err := lang.SpliceLLMSCodes(string(source))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(llmsPath, []byte(spliced), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", skill, "and the error-codes section of", llmsPath)
}

// repoRoot walks up from the working directory to the module root — go
// generate runs commands in the directive's package directory, so the
// repo-root-relative artifact paths need the root found, not assumed.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		source, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil && strings.Contains(string(source), "module zhex") {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no zhex module root above %s — run from the repository", dir)
		}
		dir = parent
	}
}
