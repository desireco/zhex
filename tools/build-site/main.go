package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"zhex/internal/lang"
)

type Example struct {
	ID, Title, Description string
	Source, Input, Output  string
	Command, SourceURL     string
}

type pageData struct {
	HeroSource string
	HeroInput  string
	HeroOutput string
	Examples   []Example
	Roadmap    Roadmap
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
	if len(lines) == 0 || lines[0] != "# Zhex roadmap" {
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
		if strings.HasPrefix(line, "#") || strings.ContainsAny(line, "`*_[]<>") && !strings.HasPrefix(line, "- ") {
			return roadmap, fmt.Errorf("unsupported roadmap markup: %s", line)
		}
		if len(roadmap.Sections) == 0 {
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

var zhexKeywords = map[string]bool{
	"word": true, "let": true, "var": true, "if": true, "elif": true, "else": true,
	"for": true, "in": true, "flow": true, "try": true, "and": true, "or": true,
	"not": true, "true": true, "false": true,
}

var zhexHostWords = map[string]bool{
	"print": true, "range": true, "int.to-string": true, "text.reverse": true,
	"console.read-line": true, "console.read-int": true,
}

// highlight marks up Zhex source with the design system's token classes.
// Every token's text and everything between tokens is HTML-escaped; only
// the spans themselves are added, so the result is safe trusted HTML.
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
			case zhexKeywords[text]:
				writeToken("tok-keyword", text)
				if text == "word" {
					if name, _, ok := identAt(skipBlank(j)); ok {
						words[name] = true
					}
				}
			case strings.ContainsRune(text, '.') || zhexHostWords[text]:
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

var exampleSpecs = []struct{ id, title, description, input string }{
	{"composition", "Composition", "Build larger operations from small typed words.", ""},
	{"hello", "Console input", "Read a name, transform the value, and print a greeting.", "Ada\n"},
	{"fizzbuzz", "FizzBuzz", "Combine conditions, integer remainders, and a bounded loop.", "15\n"},
	{"fibonacci", "Fibonacci", "Use explicit mutable locals for an iterative algorithm.", "10\n"},
	{"gcd", "Greatest common divisor", "Recurse with Euclid's method and pipe a computed result through a flow.", "48\n18\n"},
	{"times-table", "Times table", "Iterate a bounded range and build output strings from integers.", "7\n"},
	{"factorial", "Factorial", "Accumulate a result using a half-open integer range.", "5\n"},
	{"palindrome", "Palindrome", "Compare text with its Unicode code-point reversal.", "racecar\n"},
}

func main() {
	output := flag.String("out", "_site", "directory for the generated static website")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/build-site [-out directory]")
		os.Exit(2)
	}
	if err := build(".", *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Built Zhex website in %s\n", *output)
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
		var output bytes.Buffer
		if err := lang.Run(filename, string(source), strings.NewReader(spec.input), &output); err != nil {
			return pageData{}, fmt.Errorf("run website example %s: %w", filename, err)
		}
		data.Examples = append(data.Examples, Example{
			ID: spec.id, Title: spec.title, Description: spec.description,
			Source: string(source), Input: strings.TrimSuffix(spec.input, "\n"), Output: output.String(),
			Command:   "zhex run examples/" + filename,
			SourceURL: "examples/" + filename,
		})
		if spec.id == "hello" {
			data.HeroSource = string(source)
			data.HeroInput = strings.TrimSuffix(spec.input, "\n")
			data.HeroOutput = output.String()
		}
	}
	return data, nil
}

// Pages under site/design (the design system and the concept archive) are
// static documents: they are copied verbatim into design/ in the output.
func loadDesignFiles(root string, files map[string][]byte) error {
	base := filepath.Join(root, "site", "design")
	return filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read design asset %s: %w", rel, err)
		}
		files["design/"+filepath.ToSlash(rel)] = content
		return nil
	})
}

func build(root, output string) error {
	data, err := loadExamples(root)
	if err != nil {
		return err
	}
	files := map[string][]byte{".nojekyll": {}}
	if err := loadDesignFiles(root, files); err != nil {
		return err
	}
	roadmap, err := os.ReadFile(filepath.Join(root, "ROADMAP.md"))
	if err != nil {
		return fmt.Errorf("read roadmap: %w", err)
	}
	data.Roadmap, err = loadRoadmap(roadmap)
	if err != nil {
		return err
	}
	files["ROADMAP.md"] = roadmap
	for _, name := range []string{"index", "examples", "roadmap", "docs"} {
		page, err := template.New(name + ".tmpl").Funcs(template.FuncMap{"highlight": highlight}).ParseFiles(filepath.Join(root, "site", name+".tmpl"))
		if err != nil {
			return fmt.Errorf("parse website template %s: %w", name, err)
		}
		var html bytes.Buffer
		if err := page.Execute(&html, data); err != nil {
			return fmt.Errorf("render website %s: %w", name, err)
		}
		files[name+".html"] = html.Bytes()
	}
	for _, asset := range []string{"styles.css", "favicon.svg", "logo.svg", "llms.txt"} {
		content, err := os.ReadFile(filepath.Join(root, "site", asset))
		if err != nil {
			return fmt.Errorf("read website asset %s: %w", asset, err)
		}
		files[asset] = content
	}
	for _, example := range data.Examples {
		files[example.SourceURL] = []byte(example.Source)
	}
	if err := validateLinks(files); err != nil {
		return err
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(root, output)
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("create website output: %w", err)
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(output, name)), 0755); err != nil {
			return fmt.Errorf("create website directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(output, name), content, 0644); err != nil {
			return fmt.Errorf("write website file %s: %w", name, err)
		}
	}
	return nil
}

var linkPattern = regexp.MustCompile(`(?:href|src)=["']([^"']+)["']`)

func validateLinks(files map[string][]byte) error {
	for name, content := range files {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		for _, match := range linkPattern.FindAllSubmatch(content, -1) {
			target, err := url.Parse(string(match[1]))
			if err != nil {
				return fmt.Errorf("invalid link in %s: %w", name, err)
			}
			if target.IsAbs() || target.Host != "" {
				continue
			}
			if strings.HasPrefix(target.Path, "/") {
				return fmt.Errorf("local link must be relative to the Pages project in %s: %s", name, target)
			}
			path := name
			if target.Path != "" {
				path = filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(name), target.Path)))
				if strings.HasSuffix(target.Path, "/") {
					path = filepath.ToSlash(filepath.Join(path, "index.html"))
				}
			}
			linked, ok := files[path]
			if !ok {
				return fmt.Errorf("broken local link in %s: %s", name, target)
			}
			if target.Fragment != "" && !bytes.Contains(linked, []byte(`id="`+target.Fragment+`"`)) && !bytes.Contains(linked, []byte(`id='`+target.Fragment+`'`)) {
				return fmt.Errorf("missing anchor in %s: %s", name, target)
			}
		}
	}
	return nil
}
