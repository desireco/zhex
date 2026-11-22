package lang

import (
	"fmt"
	"strings"
)

// agentdocs.go renders the generated agent surfaces (#194): the per-code
// error-codes section of site/llms.txt and the whole of
// skills/diagnostics/SKILL.md. Generation is one direction only, from the
// embedded catalog these functions read — a hand-kept second copy drifts
// (the #191 research's Solid #5051 lesson) — and agentdocs_test.go fails
// the suite when either artifact stops matching.
//
//go:generate go run zhex/internal/lang/genagentdocs

// The fence around llms.txt's generated error-codes list: everything
// between the two markers is generated, everything outside them hand kept.
const (
	LLMSCodesFenceBegin = "<!-- zhex:error-codes begin -->"
	LLMSCodesFenceEnd   = "<!-- zhex:error-codes end -->"
)

// FirstSentence flattens the entry's prose to its first sentence — the
// one-line listing form llms.txt and `zhex errors` share. Section prose
// can wrap lines, so join before cutting at the sentence boundary.
func (e CatalogEntry) FirstSentence() string {
	meaning, _ := e.splitMeaningFix()
	return meaning
}

// splitMeaningFix cuts the entry's flattened prose at its first sentence
// boundary: the first sentence is the meaning, any remainder the fix — the
// durable analog of the diagnostic's dynamic hint.
func (e CatalogEntry) splitMeaningFix() (meaning, fix string) {
	flat := strings.Join(strings.Fields(e.Text), " ")
	if i := strings.Index(flat, ". "); i >= 0 {
		return flat[:i+1], flat[i+2:]
	}
	return flat, ""
}

// LLMSErrorCodes renders the llms.txt error-codes list — one line per
// catalog code with its one-line meaning and learn URL, in sorted
// CatalogEntries order. Each line ends in a newline.
func LLMSErrorCodes() string {
	var b strings.Builder
	for _, entry := range CatalogEntries() {
		fmt.Fprintf(&b, "- [%s](%s): %s\n", entry.Code, Code(entry.Code).DocsURL(), entry.FirstSentence())
	}
	return b.String()
}

// SpliceLLMSCodes rewrites the generated error-codes list inside an
// llms.txt document, leaving every byte outside the fence untouched; a
// document without both markers fails instead of passing through.
func SpliceLLMSCodes(llms string) (string, error) {
	begin := strings.Index(llms, LLMSCodesFenceBegin)
	end := strings.LastIndex(llms, LLMSCodesFenceEnd)
	if begin < 0 || end < begin {
		return "", fmt.Errorf("llms.txt lacks the %s / %s fence around the generated error-codes list", LLMSCodesFenceBegin, LLMSCodesFenceEnd)
	}
	return llms[:begin] + LLMSCodesFenceBegin + "\n" + LLMSErrorCodes() + LLMSCodesFenceEnd + llms[end+len(LLMSCodesFenceEnd):], nil
}

// SkillMarkdown renders skills/diagnostics/SKILL.md — the repository's
// agent repair skill: frontmatter, the workflow preamble, and one anchored
// section per catalog code with its meaning, fix, and learn URL.
func SkillMarkdown() string {
	var b strings.Builder
	b.WriteString(`---
name: zhex-diagnostics
description: Repair guide for Zhex diagnostic codes — use when a zhex report or diagnostic names a code; one anchored section per code with its meaning, fix, and learn URL, generated from the catalog inside the tool.
---

<!-- GENERATED from internal/lang/catalog.md — edit the catalog, then run ` + "`go generate ./internal/lang`" + `; never edit this file by hand. -->

# Zhex diagnostics repair guide

## Workflow

- Zhex checks the whole program before anything runs. Run ` + "`zhex run <file> --report=json`" + ` and read each entry ` + "`{code, severity, file, line, column, message, hint}`" + ` (plus additive ` + "`span_start_column`/`span_end_column`" + ` on checker diagnostics and ` + "`url`" + ` on catalog codes); a clean run means names, types, and word connections resolved.
- Apply the message and its ` + "`hint:`" + ` line — the hint names the action — and use the code's section below for the meaning and the fix. Rerun until the report is clean.
- Offline, ` + "`zhex errors <code>`" + ` prints the code's full catalog entry with its learn URL; ` + "`zhex errors`" + ` lists every code, ` + "`zhex errors --json`" + ` emits the catalog as JSON, and an unknown code suggests the closest.
- To watch what a program did instead of its failures, add ` + "`--trace`" + `: JSONL step events stream to stderr before any diagnostic, and the flags combine (events, then the report).

`)
	for _, entry := range CatalogEntries() {
		meaning, fix := entry.splitMeaningFix()
		fmt.Fprintf(&b, "## %s\n\n%s\n\n", entry.Code, meaning)
		if fix != "" {
			fmt.Fprintf(&b, "Fix: %s\n\n", fix)
		}
		fmt.Fprintf(&b, "Learn: %s\n\n", Code(entry.Code).DocsURL())
	}
	return b.String()
}
