package main

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/prepare-zed [output-directory]")
		os.Exit(2)
	}
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	output := filepath.Join(root, "dist/editors/zed")
	if len(os.Args) == 2 {
		output, err = filepath.Abs(os.Args[1])
		if err != nil {
			fail(err)
		}
	}
	if err := prepare(root, output); err != nil {
		fail(err)
	}
	fmt.Println(output)
	fmt.Println("In Zed: Extensions > Install Dev Extension > select this folder.")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// This edits our own manifest's grammar section, not arbitrary TOML documents.
func localManifest(source, uri string) (string, string, error) {
	lines := strings.Split(source, "\n")
	inGrammar := false
	sections, repositories, revisions := 0, 0, 0
	revision := ""
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inGrammar = line == "[grammars.zhex]"
			if inGrammar {
				sections++
			}
			continue
		}
		if !inGrammar {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "repository":
			repositories++
			lines[i] = "repository = " + strconv.Quote(uri)
		case "rev":
			revisions++
			var err error
			revision, err = strconv.Unquote(strings.TrimSpace(value))
			if err != nil {
				return "", "", fmt.Errorf("grammar rev must be a quoted commit hash: %w", err)
			}
		}
	}
	if sections != 1 || repositories != 1 || revisions != 1 {
		return "", "", fmt.Errorf("extension.toml needs one [grammars.zhex] section with repository and rev")
	}
	if _, err := hex.DecodeString(revision); err != nil || len(revision) != 40 {
		return "", "", fmt.Errorf("grammar rev must be a full 40-character commit hash")
	}
	return strings.Join(lines, "\n"), revision, nil
}

func prepare(root, output string) error {
	extension := filepath.Join(root, "editors/zed")
	source, err := os.ReadFile(filepath.Join(extension, "extension.toml"))
	if err != nil {
		return fmt.Errorf("read Zed manifest (run from the repository root): %w", err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	manifest, revision, err := localManifest(string(source), uri)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", root, "cat-file", "-e", revision+":editors/tree-sitter-zhex/src/parser.c")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pinned grammar is unavailable in the local checkout: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "extension.toml"), []byte(manifest), 0644); err != nil {
		return err
	}
	return fs.WalkDir(os.DirFS(filepath.Join(extension, "languages")), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		destination := filepath.Join(output, "languages", filepath.FromSlash(path))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(filepath.Join(extension, "languages", filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
}
