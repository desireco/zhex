package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"zhex/internal/lang"
	"zhex/internal/wasm"
)

type Example struct {
	ID, Title, Description string
	Group                  string
	Source, Input, Output  string
	Highlighted            string
	Command, SourceURL     string
	// Stdin is the raw manifest input, trailing newline included; Input is
	// the trimmed display value.
	Stdin string
	// InputKind names what the run panel's box expects — "none" (the
	// program reads nothing), "int" (InputLines integer lines), or "text"
	// (InputLines lines of text) — and InputHint is the short human
	// description rendered under the Program input label.
	InputKind  string
	InputLines int
	InputHint  string
	// Wasm is the compiled artifact's relative URL when the example is in
	// the compiled subset, and WasmNote the fence diagnostic message when it
	// is not; the -wasm pass sets them.
	Wasm, WasmNote string
	// File labels the entry source's code block; Modules stacks labeled
	// library files under it when the example spans several files.
	File    string
	Modules []ExampleModule
}

// ExampleModule is one labeled secondary source beside the entry file.
type ExampleModule struct {
	File, Source, URL string
	Highlighted       string
}

type pageData struct {
	Manifesto  Manifesto
	HeroSource string
	HeroInput  string
	HeroOutput string
	Examples   []Example
	Roadmap    Roadmap
	Catalog    []CatalogEntry
	Nav        template.HTML
}

// CatalogEntry is one stable diagnostic code with its catalog prose; the
// source of truth is the embedded catalog in internal/lang.
type CatalogEntry struct {
	Code, Text string
}

// loadCatalog splits the language's catalog markdown into one entry per
// "## code" section, skipping the intro that precedes the first section.
func loadCatalog() ([]CatalogEntry, error) {
	sections := strings.Split(strings.TrimSpace(lang.CatalogMarkdown()), "\n## ")
	var entries []CatalogEntry
	for i, section := range sections {
		if i == 0 {
			continue
		}
		code, text, _ := strings.Cut(section, "\n")
		entries = append(entries, CatalogEntry{strings.TrimSpace(code), strings.TrimSpace(text)})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("diagnostic catalog has no sections")
	}
	return entries, nil
}

type Roadmap struct {
	Intro    string
	Sections []RoadmapSection
	// DoneGroups renders the Done section for the changelog page: the
	// grouping is mechanical, derived from each bullet's leading prefix.
	DoneGroups []RoadmapGroup
}

type RoadmapSection struct {
	Title, Summary string
	Items          []string
}

// RoadmapGroup is one changelog group: every Done bullet whose leading
// "Name:" prefix shares the name, in first-appearance order, with the
// section's own items untouched underneath.
type RoadmapGroup struct {
	Name, Anchor string
	Count        int
	Items        []string
}

// The roadmap uses plain paragraphs, level-two headings, and single-line bullets.
// Keep this small format explicit rather than silently dropping Markdown syntax.
func loadRoadmap(source []byte) (Roadmap, error) {
	var roadmap Roadmap
	lines := strings.Split(strings.TrimSpace(string(source)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# Zhex roadmap" {
		return roadmap, fmt.Errorf("ROADMAP.md must start with # Zhex roadmap")
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			roadmap.Sections = append(roadmap.Sections, RoadmapSection{Title: strings.TrimPrefix(line, "## ")})
			continue
		}
		if strings.HasPrefix(line, "#") || (strings.ContainsAny(line, "`*_[]<>") && !strings.HasPrefix(line, "- ")) {
			return roadmap, fmt.Errorf("unsupported roadmap markup: %s", line)
		}
		if len(roadmap.Sections) == 0 {
			if strings.HasPrefix(line, "- ") {
				return roadmap, fmt.Errorf("roadmap bullet outside a section: %s", line)
			}
			roadmap.Intro = strings.TrimSpace(roadmap.Intro + " " + line)
			continue
		}
		section := &roadmap.Sections[len(roadmap.Sections)-1]
		if strings.HasPrefix(line, "- ") {
			section.Items = append(section.Items, strings.TrimPrefix(line, "- "))
		} else if len(section.Items) == 0 {
			section.Summary = strings.TrimSpace(section.Summary + " " + line)
		} else {
			return roadmap, fmt.Errorf("roadmap bullets must each occupy one line: %s", line)
		}
	}
	for i, title := range []string{"Done", "Next", "Future"} {
		if len(roadmap.Sections) != 3 || roadmap.Sections[i].Title != title || len(roadmap.Sections[i].Items) == 0 {
			return roadmap, fmt.Errorf("ROADMAP.md needs Done, Next, and Future sections with bullets")
		}
	}
	roadmap.DoneGroups = groupDone(roadmap.Sections[0].Items)
	return roadmap, nil
}

// groupDone derives the changelog grouping from data, not curation: a
// bullet's leading "Name:" prefix (colon plus space) names its group, groups
// keep first-appearance order, and prefix-less bullets or prefixes over 40
// bytes fall under "More" so a long sentence never becomes a heading.
func groupDone(items []string) []RoadmapGroup {
	index := map[string]int{}
	var groups []RoadmapGroup
	for _, item := range items {
		name := "More"
		if prefix, _, found := strings.Cut(item, ": "); found && prefix != "" && len(prefix) <= 40 {
			name = prefix
		}
		i, ok := index[name]
		if !ok {
			i = len(groups)
			index[name] = i
			groups = append(groups, RoadmapGroup{Name: name, Anchor: anchorSlug(name)})
		}
		groups[i].Items = append(groups[i].Items, item)
		groups[i].Count = len(groups[i].Items)
	}
	return groups
}

// anchorSlug turns a group name into the changelog page's stable anchor id.
func anchorSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteRune('-')
		}
	}
	return "done-" + strings.Trim(b.String(), "-")
}

// highlight marks up Zhex source with the design system's token classes.
// Every token's text and everything between tokens is HTML-escaped; only
// the spans themselves are added, so the result is safe trusted HTML.
// codeChips renders backticked spans as <code> chips. Every segment is
// HTML-escaped and only the code tags are added, so the result is safe
// trusted HTML.
func codeChips(text string) template.HTML {
	parts := strings.Split(text, "`")
	var out strings.Builder
	for i, part := range parts {
		if i%2 == 1 {
			out.WriteString("<code>")
		}
		out.WriteString(html.EscapeString(part))
		if i%2 == 1 {
			out.WriteString("</code>")
		}
	}
	return template.HTML(out.String())
}

func highlight(source string) template.HTML {
	runes := []rune(source)
	words := map[string]bool{}
	var out strings.Builder
	writeToken := func(class, text string) {
		out.WriteString(`<span class="` + class + `">` + html.EscapeString(text) + `</span>`)
	}
	identAt := func(i int) (string, int, bool) {
		if i >= len(runes) || !unicode.IsLetter(runes[i]) {
			return "", i, false
		}
		j := i + 1
		for j < len(runes) && (runes[j] == '-' || runes[j] == '.' || unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
			j++
		}
		return string(runes[i:j]), j, true
	}
	skipBlank := func(i int) int {
		for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		return i
	}
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case r == '#':
			j := i
			for j < len(runes) && runes[j] != '\n' {
				j++
			}
			writeToken("tok-comment", string(runes[i:j]))
			i = j
		case r == '"':
			j := i + 1
			for j < len(runes) && runes[j] != '"' && runes[j] != '\n' {
				j++
				if runes[j-1] == '\\' {
					j++
				}
			}
			if j < len(runes) && runes[j] == '"' {
				j++
			}
			writeToken("tok-string", string(runes[i:j]))
			i = j
		case unicode.IsDigit(r):
			j := i + 1
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			writeToken("tok-number", string(runes[i:j]))
			i = j
		case unicode.IsLetter(r):
			text, j, _ := identAt(i)
			switch {
			case text == "flow":
				writeToken("tok-flow", text)
			case lang.Reserved(text):
				writeToken("tok-keyword", text)
				if text == "word" {
					if name, _, ok := identAt(skipBlank(j)); ok {
						words[name] = true
					}
				}
			case strings.ContainsRune(text, '.') || lang.HostWord(text):
				writeToken("tok-function", text)
			default:
				if k := skipBlank(j); k < len(runes) && runes[k] == '(' {
					writeToken("tok-function", text)
				} else if words[text] {
					writeToken("tok-function", text)
				} else if head, _ := utf8.DecodeRuneInString(text); unicode.IsUpper(head) {
					writeToken("tok-type", text)
				} else {
					writeToken("tok-variable", text)
				}
			}
			i = j
		default:
			out.WriteString(html.EscapeString(string(r)))
			i++
		}
	}
	return template.HTML(out.String())
}

// exampleSpecs is the human-facing manifest; its enforced twin is
// corpusTable in internal/wasm/corpus_test.go — the manifest is the matrix,
// that table is the gate; keep both in sync when an example's inputs change.
type exampleSpec struct {
	id, title, description, input, group string
	args                                 []string
	files                                []string // labeled library files beside the entry (a module)
	// kind is the run panel's input kind ("none", "int", or "text"),
	// lines the expected line count for int and text, and hint the short
	// human description of the entry; none rows leave it empty and the
	// generator renders the standard no-input sentence.
	kind  string
	lines int
	hint  string
}

var exampleSpecs = []exampleSpec{
	{"hello", "Console input", "Read a name, transform the value, and print a greeting.", "Ada\n", "Language basics", nil, nil, "text", 1, "One line of text"},
	{"composition", "Composition", "Build larger operations from small typed words.", "", "Language basics", nil, nil, "none", 0, ""},
	{"calls", "Calls and defaults", "Call words with or without parentheses and compose them in flows.", "", "Language basics", nil, nil, "none", 0, ""},
	{"word-defaults", "Word defaults", "Use inferred parameter defaults and fresh values for each call.", "", "Language basics", nil, nil, "none", 0, ""},
	{"partials", "Explicit partial application", "Capture arguments with holes, then call the resulting word value.", "", "Language basics", nil, nil, "none", 0, ""},
	{"records", "Named records", "Group related values in a checked named type.", "", "Language basics", nil, nil, "none", 0, ""},
	{"choices", "Choices and matching", "Select and handle alternatives with a checked match.", "", "Language basics", nil, nil, "none", 0, ""},
	{"collections", "Lists and arrays", "Build and safely iterate lists and rectangular arrays.", "", "Language basics", nil, nil, "none", 0, ""},
	{"temporal", "Dates and time zones", "Compare calendar days with elapsed time across a daylight-saving transition.", "", "Language basics", nil, nil, "none", 0, ""},
	{"json-config", "Typed JSON configuration", "Decode configuration with typed defaults and encode it again.", "", "Language basics", nil, nil, "none", 0, ""},
	{"json-tree", "JSON trees", "Parse a heterogeneous payload into a Json tree, walk it recursively, and encode it back.", "", "Language basics", nil, nil, "none", 0, ""},
	{"xml-config", "Typed XML configuration", "Decode an embedded XML document with attribute fields, a list of endpoint records, and an optional owner.", "", "Language basics", nil, nil, "none", 0, ""},
	{"maps", "Maps and lookup", "Store keyed values, match optional lookups, and merge maps.", "", "Language basics", nil, nil, "none", 0, ""},
	{"recovery", "Error recovery", "Catch invalid numeric input and continue with a useful fallback.", "", "Language basics", nil, nil, "none", 0, ""},
	{"exit-status", "Exit status", "Choose a valid status; invalid input exits with status 2.", "0\n", "Language basics", nil, nil, "int", 1, "One number"},
	{"text-stats", "Text statistics", "Read a file two ways: whole-buffer counts and a streamed uppercased echo.", "", "Language basics", []string{"examples/text-stats-sample.txt"}, nil, "none", 0, ""},
	{"line-filter", "Line filter", "Split embedded records, select them by literal prefix and suffix, and join a report.", "", "Language basics", nil, nil, "none", 0, ""},
	{"score-report", "Score report", "Trim and score embedded records with list map, filter, fold, find, any, and all.", "", "Language basics", nil, nil, "none", 0, ""},
	{"sorting", "Sorting", "Rank records with list.sort-by and a named key word, and sort plain text with list.sort.", "", "Language basics", nil, nil, "none", 0, ""},
	{"file-write", "File write", "Attempt a checked report write and handle the failed Result with catch.", "", "Language basics", nil, nil, "none", 0, ""},
	{"paths", "Lexical paths", "Compose and decompose paths with file.join, base, extension, and directory-of.", "", "Language basics", nil, nil, "none", 0, ""},
	{"file-work", "File work folder", "Create a work folder, write reports into it, inspect them, and remove the whole tree.", "", "Language basics", []string{"."}, nil, "none", 0, ""},
	{"env-config", "Env configuration", "Write a .env fixture, load it, read settings back as options, and clean up.", "", "Language basics", []string{"."}, nil, "none", 0, ""},
	{"modules/main", "Modules", "Split a program across files: main imports a sibling math library.", "48\n18\n", "Language basics", nil, []string{"math.zhex"}, "int", 2, "Two numbers, one per line"},
	{"modules/math.test", "Module tests", "Check a module's words from a .test.zhex file the zhex test runner discovers.", "", "Language basics", nil, []string{"math.zhex", "test.zhex"}, "none", 0, ""},
	{"modules/md-report/main", "Markdown report", "Compose records and lists into a Markdown report through a pure-Zhex md module.", "", "Language basics", nil, []string{"md.zhex"}, "none", 0, ""},
	{"modules/html-page/main", "HTML page", "Compose a page from the html module: text escapes by default, markup nests through element with one deliberate raw, and a javascript: link is rejected.", "", "Language basics", nil, []string{"html.zhex"}, "none", 0, ""},
	{"fizzbuzz", "FizzBuzz", "Combine conditions, integer remainders, and a bounded loop.", "15\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"fibonacci", "Fibonacci", "Use explicit mutable locals for an iterative algorithm.", "10\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"factorial", "Factorial", "Accumulate a result using a half-open integer range.", "5\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"times-table", "Times table", "Iterate a bounded range and build output strings from integers.", "7\n", "Algorithms", nil, nil, "int", 1, "One number"},
	{"palindrome", "Palindrome", "Compare text with its Unicode code-point reversal.", "racecar\n", "Algorithms", nil, nil, "text", 1, "One line of text"},
	{"gcd", "Greatest common divisor", "Recurse with Euclid's method and pipe a computed result through a flow.", "48\n18\n", "Algorithms", nil, nil, "int", 2, "Two numbers, one per line"},
	{"life", "Game of Life", "Carry a finite grid through each generation in a flow; cells beyond the board are dead.", "", "Algorithms", nil, nil, "none", 0, ""},
	{"edit-distance", "Edit distance", "Fill a dynamic-programming table to compare Unicode text.", "", "Algorithms", nil, nil, "none", 0, ""},
	{"collatz", "Collatz steps", "Count the steps each number takes to reach 1 with a while loop.", "", "Algorithms", nil, nil, "none", 0, ""},
}

func main() {
	wasmDir := flag.String("wasm", "", "compile each example's browser WebAssembly artifact into this directory")
	flag.Parse()
	data, err := loadExamples(".")
	if err == nil {
		data.Roadmap, err = readRoadmap("ROADMAP.md")
	}
	if err == nil {
		var source []byte
		source, err = os.ReadFile("docs/manifesto.md")
		if err == nil {
			data.Manifesto, err = loadManifesto(source)
		}
	}
	if err == nil {
		data.Catalog, err = loadCatalog()
	}
	if err == nil && *wasmDir != "" {
		err = compileExamples(&data, *wasmDir)
	}
	if err == nil {
		err = json.NewEncoder(os.Stdout).Encode(data)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readRoadmap(path string) (Roadmap, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Roadmap{}, err
	}
	return loadRoadmap(source)
}

// validateSpec enforces the run-panel declaration: the kind must classify
// the row and agree with the stdin it pairs with; the page validates the
// box against it, so a wrong declaration would teach visitors the wrong
// entry.
func validateSpec(spec exampleSpec) error {
	if spec.kind != "none" && spec.kind != "int" && spec.kind != "text" {
		return fmt.Errorf("example %s: kind must be none, int, or text", spec.id)
	}
	if spec.kind != "none" && spec.hint == "" {
		return fmt.Errorf("example %s: kind %s needs a hint", spec.id, spec.kind)
	}
	if spec.kind != "none" && spec.lines < 1 {
		return fmt.Errorf("example %s: kind %s needs at least one line", spec.id, spec.kind)
	}
	if stdinLines := strings.Count(spec.input, "\n"); (spec.kind == "none" && stdinLines != 0) || (spec.kind != "none" && stdinLines != spec.lines) {
		return fmt.Errorf("example %s: stdin %q disagrees with kind %s of %d lines", spec.id, spec.input, spec.kind, spec.lines)
	}
	return nil
}

func loadExamples(root string) (pageData, error) {
	var data pageData
	for _, spec := range exampleSpecs {
		if err := validateSpec(spec); err != nil {
			return pageData{}, err
		}
		kind, hint := spec.kind, spec.hint
		if kind == "none" {
			hint = "No input — this example is self-contained"
		}
		filename := spec.id + ".zhex"
		path := filepath.Join(root, "examples", filename)
		source, err := os.ReadFile(path)
		if err != nil {
			return pageData{}, fmt.Errorf("read example %s: %w", filename, err)
		}
		// Modules resolve as sibling files next to the entry, the rule the
		// interpreter applies; single-file examples never call the resolver.
		resolve := resolveSibling(path)
		var output bytes.Buffer
		runArgs := make([]string, len(spec.args))
		for i, arg := range spec.args {
			runArgs[i] = filepath.Join(root, arg)
		}
		if err := lang.RunWithModules(filename, string(source), strings.NewReader(spec.input), &output, nil, runArgs, resolve); err != nil {
			var exit *lang.ExitError
			if !errors.As(err, &exit) || exit.Status != 0 {
				return pageData{}, fmt.Errorf("run website example %s: %w", filename, err)
			}
		}
		command := "zhex run examples/" + filename
		for _, arg := range spec.args {
			command += " " + arg
		}
		example := Example{
			ID: spec.id, Title: spec.title, Description: spec.description,
			Group:  spec.group,
			Source: string(source), Highlighted: string(highlight(string(source))), Input: strings.TrimSuffix(spec.input, "\n"), Stdin: spec.input, Output: output.String(),
			InputKind: kind, InputLines: spec.lines, InputHint: hint,
			Command:   command,
			SourceURL: "examples/" + filename,
			File:      filepath.Base(filename),
		}
		for _, file := range spec.files {
			moduleSource, err := os.ReadFile(filepath.Join(filepath.Dir(path), file))
			if err != nil {
				return pageData{}, fmt.Errorf("read example %s: %w", file, err)
			}
			url := filepath.ToSlash(filepath.Join("examples", filepath.Dir(filename), file))
			example.Modules = append(example.Modules, ExampleModule{File: file, Source: string(moduleSource), URL: url, Highlighted: string(highlight(string(moduleSource)))})
		}
		data.Examples = append(data.Examples, example)
		if spec.id == "hello" {
			data.HeroSource = string(source)
			data.HeroInput = strings.TrimSuffix(spec.input, "\n")
			data.HeroOutput = output.String()
		}
	}
	return data, nil
}

// resolveSibling builds the module resolver the interpreter applies: `use
// name` reads the sibling name.zhex next to the entry file.
func resolveSibling(entryPath string) func(string) (lang.ModuleSource, bool) {
	dir := filepath.Dir(entryPath)
	return func(name string) (lang.ModuleSource, bool) {
		path := filepath.Join(dir, name+".zhex")
		module, err := os.ReadFile(path)
		if err != nil {
			return lang.ModuleSource{}, false
		}
		return lang.ModuleSource{Path: path, Source: string(module)}, true
	}
}

// compileExamples writes each in-subset example's browser WebAssembly
// artifact into dir (flat names: modules/main becomes modules-main.wasm) and
// records on the matching Example the relative artifact URL (Wasm) or the
// fence diagnostic message (WasmNote). Any other failure — a missing
// assembler, an emitter bug — fails the build loudly: no silent button-less
// site.
func compileExamples(data *pageData, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, spec := range exampleSpecs {
		example := &data.Examples[i]
		path := filepath.Join("examples", spec.id+".zhex")
		name := strings.ReplaceAll(spec.id, "/", "-") + ".wasm"
		err := wasm.Compile(path, example.Source, filepath.Join(dir, name), resolveSibling(path), lang.WatSkinBrowser)
		if err == nil {
			example.Wasm = "wasm/" + name
			continue
		}
		reportJSON, ok := lang.Report(err)
		var report struct{ Code, Message string }
		if !ok || json.Unmarshal([]byte(reportJSON), &report) != nil || report.Code != string(lang.CodeNotAvailableInBuild) {
			return fmt.Errorf("compile example %s: %w", spec.id+".zhex", err)
		}
		example.WasmNote = report.Message
	}
	return nil
}
