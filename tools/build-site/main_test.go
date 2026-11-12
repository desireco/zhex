package main

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func siteFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"site", "examples"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"site/index.tmpl":    "<main><pre>{{.HeroSource}}</pre>{{range .Examples}}<article id=\"{{.ID}}\"><h2>{{.Title}}</h2><pre>{{.Source}}</pre><pre>{{.Output}}</pre><a href=\"{{.SourceURL}}\">Source</a></article>{{end}}</main>",
		"site/examples.tmpl": "<main>{{range .Examples}}<article id=\"{{.ID}}\"><pre>{{.Source}}</pre><a href=\"{{.SourceURL}}\">Download</a></article>{{end}}</main>",
		"site/styles.css":    "body { color: black; }",
		"site/favicon.svg":   "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
	}
	for _, spec := range exampleSpecs {
		files["examples/"+spec.id+".zhex"] = "print(\"<script>alert(1)</script>\")\n"
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBuildReadsExamplesExecutesThemAndEscapesHTML(t *testing.T) {
	root := siteFixture(t)
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "output", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(content)
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Fatal("example source or output was rendered as HTML")
	}
	if !strings.Contains(page, html.EscapeString("<script>alert(1)</script>")) {
		t.Fatal("missing escaped example output")
	}
	for _, spec := range exampleSpecs {
		if !strings.Contains(page, "id=\""+spec.id+"\"") {
			t.Fatalf("missing example %s", spec.id)
		}
	}
	for _, file := range []string{".nojekyll", "styles.css", "favicon.svg", "examples.html", "examples/hello.zhex", "examples/fizzbuzz.zhex"} {
		if _, err := os.Stat(filepath.Join(root, "output", file)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBuildRejectsBrokenLocalLinks(t *testing.T) {
	root := siteFixture(t)
	if err := os.WriteFile(filepath.Join(root, "site", "index.tmpl"), []byte(`<a href="examples.html#missing">Example</a>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := build(root, "output"); err == nil || !strings.Contains(err.Error(), "missing anchor") {
		t.Fatalf("wanted missing-anchor error, got %v", err)
	}
}

func TestValidateLinksRespectsPagesProjectPaths(t *testing.T) {
	files := map[string][]byte{
		"index.html":    []byte(`<a href="./">Home</a><a href="examples.html#hello">Example</a>`),
		"examples.html": []byte(`<article id="hello"></article>`),
	}
	if err := validateLinks(files); err != nil {
		t.Fatal(err)
	}
	files["index.html"] = []byte(`<link href="/styles.css">`)
	if err := validateLinks(files); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("wanted absolute-path error, got %v", err)
	}
}

func TestBuildRejectsBrokenExamplesAndMissingAssets(t *testing.T) {
	t.Run("example", func(t *testing.T) {
		root := siteFixture(t)
		if err := os.WriteFile(filepath.Join(root, "examples", "fibonacci.zhex"), []byte("missing(1)\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := build(root, "output"); err == nil || !strings.Contains(err.Error(), "fibonacci.zhex") {
			t.Fatalf("wanted broken-example error, got %v", err)
		}
	})
	t.Run("asset", func(t *testing.T) {
		root := siteFixture(t)
		if err := os.Remove(filepath.Join(root, "site", "styles.css")); err != nil {
			t.Fatal(err)
		}
		if err := build(root, "output"); err == nil || !strings.Contains(err.Error(), "styles.css") {
			t.Fatalf("wanted missing-asset error, got %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "output")); !os.IsNotExist(err) {
			t.Fatal("created partial site despite missing asset")
		}
	})
}

func TestWebsiteExamplesUseRepositorySourcesAndActualOutputs(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := loadExamples(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"composition": "32\n", "hello": "Your name: Hello, Ada!\n", "fibonacci": "Fibonacci index (0..92): 55\n", "factorial": "Factorial of (0..20): 120\n", "palindrome": "Text to check: Palindrome\n"}
	if len(data.Examples) != 6 {
		t.Fatalf("got %d examples", len(data.Examples))
	}
	for _, example := range data.Examples {
		source, err := os.ReadFile(filepath.Join(root, "examples", example.ID+".zhex"))
		if err != nil {
			t.Fatal(err)
		}
		if example.Source != string(source) {
			t.Fatalf("source drifted for %s", example.ID)
		}
		if output, ok := want[example.ID]; ok && example.Output != output {
			t.Fatalf("%s output %q; want %q", example.ID, example.Output, output)
		}
		if example.ID == "fizzbuzz" && !strings.HasSuffix(example.Output, "FizzBuzz\n") {
			t.Fatalf("bad FizzBuzz output %q", example.Output)
		}
		if example.ID == "hello" && (data.HeroSource != example.Source || data.HeroOutput != example.Output || data.HeroInput != example.Input) {
			t.Fatal("hero source, input, or output does not match hello example")
		}
	}
}
