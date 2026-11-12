package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

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
}

var exampleSpecs = []struct{ id, title, description, input string }{
	{"composition", "Composition", "Build larger operations from small typed words.", ""},
	{"hello", "Console input", "Read a name, transform the value, and print a greeting.", "Ada\n"},
	{"fizzbuzz", "FizzBuzz", "Combine conditions, integer remainders, and a bounded loop.", "15\n"},
	{"fibonacci", "Fibonacci", "Use explicit mutable locals for an iterative algorithm.", "10\n"},
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
			Command:   "go run ./cmd/zhex run examples/" + filename,
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

func build(root, output string) error {
	data, err := loadExamples(root)
	if err != nil {
		return err
	}
	files := map[string][]byte{".nojekyll": {}}
	for _, name := range []string{"index", "examples"} {
		page, err := template.ParseFiles(filepath.Join(root, "site", name+".tmpl"))
		if err != nil {
			return fmt.Errorf("parse website template %s: %w", name, err)
		}
		var html bytes.Buffer
		if err := page.Execute(&html, data); err != nil {
			return fmt.Errorf("render website %s: %w", name, err)
		}
		files[name+".html"] = html.Bytes()
	}
	for _, asset := range []string{"styles.css", "favicon.svg"} {
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
