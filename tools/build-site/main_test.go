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
	for _, dir := range []string{"site", "site/design", "examples", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"docs/manifesto.md":              "# Why Zhex exists.\n\nReadable programs.\n\n## Small words\n\nUseful pieces.\n",
		"ROADMAP.md":                     "# Zhex roadmap\n\nA readable language.\n\n## Done\n\nWorking today.\n\n- Checked words.\n\n## Next\n\nTry it interactively.\n\n- REPL.\n\n## Future\n\nOpen ideas.\n\n- Compilation.\n",
		"site/roadmap.tmpl":              `<main><p>{{.Roadmap.Intro}}</p>{{range .Roadmap.Sections}}<section><h2>{{.Title}}</h2><p>{{.Summary}}</p><ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul></section>{{end}}<a href="ROADMAP.md">Roadmap source</a></main>`,
		"site/docs.tmpl":                 "<main><h1>Editors</h1><p>Syntax highlighting for Zhex.</p><a href=\"ROADMAP.md\">Roadmap</a></main>",
		"site/errors.tmpl":               `<main>{{range .Catalog}}<section><h2 id="{{.Code}}">{{.Code}}</h2><p>{{.Text | codeChips}}</p></section>{{end}}</main>`,
		"site/manifesto.tmpl":            `<main><h1>{{.Manifesto.Title}}</h1>{{range .Manifesto.Intro}}<p>{{.}}</p>{{end}}{{range .Manifesto.Sections}}<h2>{{.Title}}</h2>{{range .Paragraphs}}<p>{{.}}</p>{{end}}{{end}}<a href="manifesto.md">Source</a></main>`,
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
		for _, file := range spec.files {
			files[filepath.ToSlash(filepath.Join("examples", filepath.Dir(spec.id), file))] = "word helper() -> Unit\n    print(1)\n"
		}
		for _, arg := range spec.args {
			files[arg] = "fixture text\n"
		}
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
			t.Fatal(err)
		}
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
	for _, file := range []string{".nojekyll", "styles.css", "favicon.svg", "logo.svg", "llms.txt", "examples.html", "docs.html", "errors.html", "examples/hello.zhex", "examples/fizzbuzz.zhex"} {
		if _, err := os.Stat(filepath.Join(root, "output", file)); err != nil {
			t.Fatal(err)
		}
	}
	errorsPage, err := os.ReadFile(filepath.Join(root, "output", "errors.html"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := string(errorsPage)
	if !strings.Contains(catalog, "<code>") {
		t.Fatal("catalog prose did not render code chips")
	}
	if strings.Contains(catalog, "`") {
		t.Fatal("catalog prose still contains raw backticks")
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
	want := map[string]string{"composition": "32\n", "hello": "Your name: Hello, Ada!\n", "fibonacci": "Fibonacci index (0..92): 55\n", "factorial": "Factorial of (0..20): 120\n", "palindrome": "Text to check: Palindrome\n", "gcd": "First number: Second number: 6\n", "times-table": "Table for: 7 x 1 = 7\n7 x 2 = 14\n7 x 3 = 21\n7 x 4 = 28\n7 x 5 = 35\n7 x 6 = 42\n7 x 7 = 49\n7 x 8 = 56\n7 x 9 = 63\n7 x 10 = 70\n", "text-stats": "3 lines  11 words  64 chars\n", "modules/main": "First number: Second number: 30\n6\n144\n"}
	wantCommand := map[string]string{"text-stats": "zhex run examples/text-stats.zhex examples/text-stats-sample.txt", "modules/main": "zhex run examples/modules/main.zhex"}
	if len(data.Examples) != 10 {
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
		if expected := wantCommand[example.ID]; expected != "" {
			if example.Command != expected {
				t.Fatalf("user-facing run command for %s is %q", example.ID, example.Command)
			}
		} else if example.Command != "zhex run examples/"+example.ID+".zhex" {
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
		if example.ID == "modules/main" {
			if example.File != "main.zhex" || len(example.Modules) != 1 || example.Modules[0].File != "math.zhex" {
				t.Fatalf("modules example labels: file %q, modules %+v", example.File, example.Modules)
			}
			module, err := os.ReadFile(filepath.Join(root, "examples", "modules", "math.zhex"))
			if err != nil || example.Modules[0].Source != string(module) {
				t.Fatalf("module source drifted for %s: %v", example.ID, err)
			}
		}
	}
}

// TestModulesExampleCardRendersStackedSources pins the multi-file card: the
// entry file and its module render as labeled stacked sources, the command
// names the entry path, and both files publish for download.
func TestModulesExampleCardRendersStackedSources(t *testing.T) {
	root := siteFixture(t)
	tmpl := `<main>{{range .Examples}}{{if .Modules}}<article id="{{.ID}}">` +
		`<span class="file">{{.File}}</span><pre>{{.Source | highlight}}</pre>` +
		`{{range .Modules}}<span class="file">{{.File}}</span><pre>{{.Source | highlight}}</pre><a href="{{.URL}}">Download</a>{{end}}` +
		`<pre>{{.Command}}</pre><pre>{{.Output}}</pre></article>{{end}}{{end}}</main>`
	if err := os.WriteFile(filepath.Join(root, "site", "examples.tmpl"), []byte(tmpl), 0644); err != nil {
		t.Fatal(err)
	}
	main := "use math\n\nprint(math.gcd(54, 18))\n"
	math := "word gcd(a: Int, b: Int) -> Int\n    if b == 0\n        a\n    else\n        gcd(b, a % b)\n"
	for name, content := range map[string]string{"examples/modules/main.zhex": main, "examples/modules/math.zhex": math} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := build(root, "output"); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(root, "output", "examples.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<span class="file">main.zhex</span>`,
		`<span class="file">math.zhex</span>`,
		`<span class="tok-keyword">use</span>`,
		`<span class="tok-function">math.gcd</span>`,
		"zhex run examples/modules/main.zhex",
		"18\n",
		`href="examples/modules/math.zhex"`,
	} {
		if !strings.Contains(string(page), want) {
			t.Fatalf("examples card misses %q: %s", want, page)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "output", "examples", "modules", "math.zhex")); err != nil {
		t.Fatal("module source not published for download")
	}
}
