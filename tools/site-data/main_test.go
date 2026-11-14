package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestWebsiteExamplesUseRepositorySourcesAndActualOutputs(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := loadExamples(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"json-config": "3\ntrue\n{\"retries\":3,\"verbose\":true}\n", "maps": "1\n3\nAda: 11\nMorgan has no score\nAda: 11\nGrace: 9\nNancy: 23\n4\n4\n", "choices": "running\nCould not start: network unavailable\n7\n0\n", "records": "Ada, age 36\n", "collections": "3\nGrace\n0: Ada\ncollection-iterating\n1: Grace\ncollection-iterating\n2: Lin\ncollection-iterating\n0,0: 1\n0,1: 2\n0,2: 3\n1,0: 4\n1,1: 5\n1,2: 6\n6\n7\n12\n", "composition": "32\n", "hello": "Your name: Hello, Ada!\n", "fibonacci": "Fibonacci index (0..92): 55\n", "factorial": "Factorial of (0..20): 120\n", "palindrome": "Text to check: Palindrome\n", "gcd": "First number: Second number: 6\n", "times-table": "Table for: 7 x 1 = 7\n7 x 2 = 14\n7 x 3 = 21\n7 x 4 = 28\n7 x 5 = 35\n7 x 6 = 42\n7 x 7 = 49\n7 x 8 = 56\n7 x 9 = 63\n7 x 10 = 70\n", "text-stats": "3 lines  11 words  64 chars\n", "modules/main": "First number: Second number: 30\n6\n144\n", "recovery": "That is not a whole number. I'll use 3 retries.\nRetries: 3\n", "exit-status": "Exit status (0..255): Exiting with status 0\n", "edit-distance": "kitten → sitting: 3\nempty → abc: 3\ncafé → cafe: 1\n", "life": "Generation 0\n.......\n.......\n...#...\n....#..\n..###..\n.......\n.......\nGeneration 1\n.......\n.......\n.......\n..#.#..\n...##..\n...#...\n.......\nGeneration 2\n.......\n.......\n.......\n....#..\n..#.#..\n...##..\n.......\nGeneration 3\n.......\n.......\n.......\n...#...\n....##.\n...##..\n.......\nGeneration 4\n.......\n.......\n.......\n....#..\n.....#.\n...###.\n.......\n"}
	wantCommand := map[string]string{"text-stats": "zhex run examples/text-stats.zhex examples/text-stats-sample.txt", "modules/main": "zhex run examples/modules/main.zhex"}
	if len(data.Examples) != 19 {
		t.Fatalf("got %d examples", len(data.Examples))
	}
	wantGroup := map[string]string{"hello": "Language basics", "gcd": "Algorithms", "life": "Algorithms", "edit-distance": "Algorithms"}
	for _, example := range data.Examples {
		source, err := os.ReadFile(filepath.Join(root, "examples", example.ID+".zhex"))
		if err != nil {
			t.Fatal(err)
		}
		if example.Source != string(source) {
			t.Fatalf("source drifted for %s", example.ID)
		}
		if example.Group != "Language basics" && example.Group != "Algorithms" {
			t.Fatalf("example %s has unknown group %q", example.ID, example.Group)
		}
		if expected := wantGroup[example.ID]; expected != "" && example.Group != expected {
			t.Fatalf("example %s group %q; want %q", example.ID, example.Group, expected)
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

func TestLoadExamplesRejectsBrokenProgram(t *testing.T) {
	root := t.TempDir()
	for _, spec := range exampleSpecs {
		filename := filepath.Join(root, "examples", spec.id+".zhex")
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		source := "print(1)\n"
		if spec.id == "fibonacci" {
			source = "print(unknown)\n"
		}
		if err := os.WriteFile(filename, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		for _, file := range spec.files {
			if err := os.WriteFile(filepath.Join(filepath.Dir(filename), file), []byte("word helper() -> Int\n    1\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := loadExamples(root); err == nil || !strings.Contains(err.Error(), "fibonacci.zhex") {
		t.Fatalf("broken example error = %v", err)
	}
}

func TestLoadExamplesRejectsNonzeroDeliberateExit(t *testing.T) {
	root := t.TempDir()
	for _, spec := range exampleSpecs {
		filename := filepath.Join(root, "examples", spec.id+".zhex")
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		source := "print(1)\n"
		if spec.id == "exit-status" {
			source = "console.exit(2)\n"
		}
		if err := os.WriteFile(filename, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		for _, file := range spec.files {
			if err := os.WriteFile(filepath.Join(filepath.Dir(filename), file), []byte("word helper() -> Int\n    1\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := loadExamples(root); err == nil || !strings.Contains(err.Error(), "exit-status.zhex") {
		t.Fatalf("nonzero exit error = %v", err)
	}
}
