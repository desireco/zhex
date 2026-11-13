package main

import (
	"fmt"
	"strings"
)

type Manifesto struct {
	Title    string
	Intro    []string
	Sections []ManifestoSection
}

type ManifestoSection struct {
	Title      string
	Paragraphs []string
}

// The manifesto uses a title, level-two headings, and plain paragraphs.
// Reject unsupported markup so edits cannot silently render incorrectly.
// The HTML template escapes all content.
func loadManifesto(source []byte) (Manifesto, error) {
	var result Manifesto
	text := strings.ReplaceAll(string(source), "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if !strings.HasPrefix(lines[0], "# ") || strings.TrimSpace(strings.TrimPrefix(lines[0], "# ")) == "" {
		return result, fmt.Errorf("manifesto needs a level-one title")
	}
	result.Title = strings.TrimSpace(strings.TrimPrefix(lines[0], "# "))
	var paragraph []string
	flush := func() {
		if len(paragraph) == 0 {
			return
		}
		p := strings.Join(paragraph, " ")
		if len(result.Sections) == 0 {
			result.Intro = append(result.Intro, p)
		} else {
			section := &result.Sections[len(result.Sections)-1]
			section.Paragraphs = append(section.Paragraphs, p)
		}
		paragraph = nil
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "## ") {
			flush()
			result.Sections = append(result.Sections, ManifestoSection{Title: strings.TrimSpace(strings.TrimPrefix(line, "## "))})
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- ") || strings.ContainsAny(line, "`*_[]") {
			return result, fmt.Errorf("unsupported manifesto markup: %s", line)
		}
		paragraph = append(paragraph, line)
	}
	flush()
	if len(result.Intro) == 0 || len(result.Sections) == 0 {
		return result, fmt.Errorf("manifesto needs an introduction and sections")
	}
	for _, section := range result.Sections {
		if section.Title == "" || len(section.Paragraphs) == 0 {
			return result, fmt.Errorf("manifesto has an empty section")
		}
	}
	return result, nil
}
