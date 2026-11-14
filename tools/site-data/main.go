package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"zhex/internal/lang"
)

type Example struct {
	ID, Title, Description string
	Group                  string
	Source, Input, Output  string
	Highlighted            string
	Command, SourceURL     string
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
}

type RoadmapSection struct {
	Title, Summary string
	Items          []string
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
	return roadmap, nil
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

var exampleSpecs = []struct {
	id, title, description, input, group string
	args                                 []string
	files                                []string // labeled library files beside the entry (a module)
}{
	{"hello", "Console input", "Read a name, transform the value, and print a greeting.", "Ada\n", "Language basics", nil, nil},
	{"composition", "Composition", "Build larger operations from small typed words.", "", "Language basics", nil, nil},
	{"records", "Named records", "Group related values in a checked named type.", "", "Language basics", nil, nil},
	{"choices", "Choices and matching", "Select and handle alternatives with a checked match.", "", "Language basics", nil, nil},
	{"collections", "Lists and arrays", "Build and safely iterate lists and rectangular arrays.", "", "Language basics", nil, nil},
	{"temporal", "Dates and time zones", "Compare calendar days with elapsed time across a daylight-saving transition.", "", "Language basics", nil, nil},
	{"json-config", "Typed JSON configuration", "Decode configuration with typed defaults and encode it again.", "", "Language basics", nil, nil},
	{"maps", "Maps and lookup", "Store keyed values, match optional lookups, and merge maps.", "", "Language basics", nil, nil},
	{"recovery", "Error recovery", "Catch invalid numeric input and continue with a useful fallback.", "", "Language basics", nil, nil},
	{"exit-status", "Exit status", "Choose a valid status; invalid input exits with status 2.", "0\n", "Language basics", nil, nil},
	{"text-stats", "Text statistics", "Read a file two ways: whole-buffer counts and a streamed uppercased echo.", "", "Language basics", []string{"examples/text-stats-sample.txt"}, nil},
	{"modules/main", "Modules", "Split a program across files: main imports a sibling math library.", "48\n18\n", "Language basics", nil, []string{"math.zhex"}},
	{"fizzbuzz", "FizzBuzz", "Combine conditions, integer remainders, and a bounded loop.", "15\n", "Algorithms", nil, nil},
	{"fibonacci", "Fibonacci", "Use explicit mutable locals for an iterative algorithm.", "10\n", "Algorithms", nil, nil},
	{"factorial", "Factorial", "Accumulate a result using a half-open integer range.", "5\n", "Algorithms", nil, nil},
	{"times-table", "Times table", "Iterate a bounded range and build output strings from integers.", "7\n", "Algorithms", nil, nil},
	{"palindrome", "Palindrome", "Compare text with its Unicode code-point reversal.", "racecar\n", "Algorithms", nil, nil},
	{"gcd", "Greatest common divisor", "Recurse with Euclid's method and pipe a computed result through a flow.", "48\n18\n", "Algorithms", nil, nil},
	{"life", "Game of Life", "Evolve a finite grid with a dead exterior and explicit array state.", "", "Algorithms", nil, nil},
	{"edit-distance", "Edit distance", "Fill a dynamic-programming table to compare Unicode text.", "", "Algorithms", nil, nil},
}

func main() {
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

func loadExamples(root string) (pageData, error) {
	var data pageData
	for _, spec := range exampleSpecs {
		filename := spec.id + ".zhex"
		path := filepath.Join(root, "examples", filename)
		source, err := os.ReadFile(path)
		if err != nil {
			return pageData{}, fmt.Errorf("read example %s: %w", filename, err)
		}
		// Modules resolve as sibling files next to the entry, the rule the
		// interpreter applies; single-file examples never call the resolver.
		dir := filepath.Dir(path)
		resolve := func(name string) (lang.ModuleSource, bool) {
			module, err := os.ReadFile(filepath.Join(dir, name+".zhex"))
			if err != nil {
				return lang.ModuleSource{}, false
			}
			return lang.ModuleSource{Path: filepath.Join(dir, name+".zhex"), Source: string(module)}, true
		}
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
			Source: string(source), Highlighted: string(highlight(string(source))), Input: strings.TrimSuffix(spec.input, "\n"), Output: output.String(),
			Command:   command,
			SourceURL: "examples/" + filename,
			File:      filepath.Base(filename),
		}
		for _, file := range spec.files {
			moduleSource, err := os.ReadFile(filepath.Join(dir, file))
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
