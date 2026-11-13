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
	for _, dir := range []string{"site", "site/design", "examples"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"ROADMAP.md":                     "# Zhex roadmap\n\nA readable language.\n\n## Done\n\nWorking today.\n\n- Checked words.\n\n## Next\n\nTry it interactively.\n\n- REPL.\n\n## Future\n\nOpen ideas.\n\n- Compilation.\n",
		"site/roadmap.tmpl":              `<main><p>{{.Roadmap.Intro}}</p>{{range .Roadmap.Sections}}<section><h2>{{.Title}}</h2><p>{{.Summary}}</p><ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul></section>{{end}}<a href="ROADMAP.md">Roadmap source</a></main>`,
		"site/docs.tmpl":                 "<main><h1>Editors</h1><p>Syntax highlighting for Zhex.</p><a href=\"ROADMAP.md\">Roadmap</a></main>",
		"site/index.tmpl":                "<main><pre>{{.HeroSource | highlight}}</pre>{{range .Examples}}<article id=\"{{.ID}}\"><h2>{{.Title}}</h2><pre>{{.Source}}</pre><pre>{{.Output}}</pre><a href=\"{{.SourceURL}}\">Source</a></article>{{end}}</main>",
		"site/examples.tmpl":             "<main>{{range .Examples}}<article id=\"{{.ID}}\"><pre>{{.Source}}</pre><a href=\"{{.SourceURL}}\">Download</a></article>{{end}}</main>",
		"site/styles.css":                "body { color: black; }",
		"site/favicon.svg":               "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"site/llms.txt":                  "llms guide",
		"site/logo.svg":                  "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>",
		"site/design/design-system.html": "<a href=\"../index.html\">Website</a>",
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

func TestHighlightMarksUpZhexSourceWithTokenClasses(t *testing.T) {
	source := "# show <n>\nword greet(name: String) -> String\nlet n = 15\nif n > 0\n    print(\"hi <b>\")\nflow n\n    greet\n"
	got := string(highlight(source))
	for _, want := range []string{
		`<span class="tok-comment"># show &lt;n&gt;</span>`,
		`<span class="tok-keyword">word</span> <span class="tok-function">greet</span>(`,
		`<span class="tok-variable">name</span>: <span class="tok-type">String</span>`,
		`-&gt; <span class="tok-type">String</span>`,
		`<span class="tok-keyword">let</span> <span class="tok-variable">n</span> = <span class="tok-number">15</span>`,
		`<span class="tok-keyword">if</span>`,
		`<span class="tok-function">print</span>(<span class="tok-string">&#34;hi &lt;b&gt;&#34;</span>)`,
		`<span class="tok-flow">flow</span> <span class="tok-variable">n</span>`,
		`<span class="tok-function">greet</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("highlight output misses %q; got %s", want, got)
		}
	}
	if strings.Contains(got, "<n>") || strings.Contains(got, `"hi`) {
		t.Fatalf("highlight output is not fully escaped: %s", got)
	}
}

func TestBuildPublishesDesignSystemTree(t *testing.T) {
	root := siteFixture(t)
	for name, content := range map[string]string{
		"site/design/design-system.html": `<a href="../index.html">Website</a><a href="archive/index.html">Archive</a><link rel="stylesheet" href="design-tokens.css">`,
		"site/design/design-tokens.css":  ":root { --paper: #f6f3ec; }",
		"site/design/archive/index.html": `<a href="../design-system.html">System</a>`,
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"design/design-system.html", "design/design-tokens.css", "design/archive/index.html"} {
		if _, err := os.Stat(filepath.Join(root, "output", file)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRoadmapPageUsesMarkdownSource(t *testing.T) {
	root := siteFixture(t)
	path := filepath.Join(root, "ROADMAP.md")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source = []byte(strings.Replace(string(source), "- REPL.", "- REPL with <clear> errors.", 1))
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(root, "output", "roadmap.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Done", "Next", "Future", "REPL with &lt;clear&gt; errors."} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("missing %q in roadmap", want)
		}
	}
	copy, err := os.ReadFile(filepath.Join(root, "output", "ROADMAP.md"))
	if err != nil || string(copy) != string(source) {
		t.Fatalf("roadmap download differs: %v", err)
	}
}

func TestRoadmapRejectsMissingOrRenamedMilestones(t *testing.T) {
	for _, source := range []string{
		"", "# Zhex roadmap\n\n## Done\n- Interpreter.\n",
		"# Zhex roadmap\n\n## Done\n- Interpreter.\n## Later\n- REPL.\n## Future\n- Compilation.\n",
	} {
		if _, err := loadRoadmap([]byte(source)); err == nil {
			t.Fatalf("accepted incomplete roadmap %q", source)
		}
	}
}

func TestRoadmapToleratesCRLFAndRejectsIntroBullets(t *testing.T) {
	crlf := "# Zhex roadmap\r\n\r\nIntro.\r\n\r\n## Done\r\n\r\nBase.\r\n\r\n- Interpreter.\r\n## Next\r\n- REPL.\r\n## Future\r\n- Compilation.\r\n"
	if _, err := loadRoadmap([]byte(crlf)); err != nil {
		t.Fatalf("rejected CRLF roadmap: %v", err)
	}
	stray := "# Zhex roadmap\n\n- Stray bullet.\n## Done\n- Interpreter.\n## Next\n- REPL.\n## Future\n- Compilation.\n"
	if _, err := loadRoadmap([]byte(stray)); err == nil || !strings.Contains(err.Error(), "outside a section") {
		t.Fatalf("wanted outside-a-section error, got %v", err)
	}
}

func TestBuildClearsStaleOutputButNotTheRoot(t *testing.T) {
	root := siteFixture(t)
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "output", "stale.html")
	if err := os.WriteFile(stray, []byte("orphan"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatal("stale file from a previous build survived")
	}
	if err := build(root, "."); err == nil || !strings.Contains(err.Error(), "repository root") {
		t.Fatalf("wanted repository-root guard error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "ROADMAP.md")); err != nil {
		t.Fatal("root guard deleted repository files")
	}
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
	for _, file := range []string{".nojekyll", "styles.css", "favicon.svg", "logo.svg", "llms.txt", "examples.html", "docs.html", "examples/hello.zhex", "examples/fizzbuzz.zhex"} {
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
	want := map[string]string{"composition": "32\n", "hello": "Your name: Hello, Ada!\n", "fibonacci": "Fibonacci index (0..92): 55\n", "factorial": "Factorial of (0..20): 120\n", "palindrome": "Text to check: Palindrome\n", "gcd": "First number: Second number: 6\n", "times-table": "Table for: 7 x 1 = 7\n7 x 2 = 14\n7 x 3 = 21\n7 x 4 = 28\n7 x 5 = 35\n7 x 6 = 42\n7 x 7 = 49\n7 x 8 = 56\n7 x 9 = 63\n7 x 10 = 70\n"}
	if len(data.Examples) != 8 {
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
		if example.Command != "zhex run examples/"+example.ID+".zhex" {
			t.Fatalf("user-facing run command for %s is %q", example.ID, example.Command)
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
