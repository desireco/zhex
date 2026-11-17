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

func TestValidateSpecRejectsWrongRunPanelDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec exampleSpec
		want string // error substring; empty means accepted
	}{
		{"unknown kind", exampleSpec{id: "x", kind: "number", lines: 1, hint: "h", input: "5\n"}, "kind must be none, int, or text"},
		{"missing hint", exampleSpec{id: "x", kind: "int", lines: 1, input: "5\n"}, "needs a hint"},
		{"lines under one", exampleSpec{id: "x", kind: "int", lines: 0, hint: "h", input: ""}, "at least one line"},
		{"int count disagrees with stdin", exampleSpec{id: "x", kind: "int", lines: 1, hint: "h", input: "5\n7\n"}, "disagrees with kind"},
		{"valid int row", exampleSpec{id: "x", kind: "int", lines: 1, hint: "h", input: "5\n"}, ""},
	} {
		err := validateSpec(tc.spec)
		if tc.want == "" {
			if err != nil {
				t.Fatalf("%s: rejected valid spec: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: wanted error containing %q, got %v", tc.name, tc.want, err)
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

func TestRoadmapGroupsDoneBulletsByLeadingPrefix(t *testing.T) {
	source := "# Zhex roadmap\n\nIntro.\n\n## Done\n\nBase.\n\n- Vocabulary: first ships.\n- Language: second ships.\n- Vocabulary: third ships.\n- No prefix here.\n- A leading prefix well past the forty-byte heading limit: demoted.\n## Next\n- REPL.\n## Future\n- Compilation.\n"
	roadmap, err := loadRoadmap([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{"Vocabulary", "Language", "More"}
	wantItems := []string{
		"Vocabulary: first ships.\nVocabulary: third ships.",
		"Language: second ships.",
		"No prefix here.\nA leading prefix well past the forty-byte heading limit: demoted.",
	}
	if len(roadmap.DoneGroups) != len(wantNames) {
		t.Fatalf("got %d groups, want %d: %+v", len(roadmap.DoneGroups), len(wantNames), roadmap.DoneGroups)
	}
	for i, group := range roadmap.DoneGroups {
		if group.Name != wantNames[i] {
			t.Fatalf("group %d is %q; want %q", i, group.Name, wantNames[i])
		}
		if strings.Join(group.Items, "\n") != wantItems[i] {
			t.Fatalf("group %q holds %q; want %q", group.Name, strings.Join(group.Items, "\n"), wantItems[i])
		}
		if group.Count != len(group.Items) {
			t.Fatalf("group %q count %d disagrees with %d items", group.Name, group.Count, len(group.Items))
		}
		if group.Anchor != "done-"+strings.ToLower(strings.ReplaceAll(group.Name, " ", "-")) {
			t.Fatalf("group %q anchor %q", group.Name, group.Anchor)
		}
	}
	if roadmap.Sections[0].Title != "Done" || len(roadmap.Sections[0].Items) != 5 {
		t.Fatalf("grouping disturbed the Done section: %+v", roadmap.Sections[0])
	}
}

// TestRoadmapDoneGroupsCoverEveryDoneBullet pins the real ROADMAP.md: the
// changelog grouping partitions the Done section — every bullet exactly once,
// each anchor unique — so neither page can silently drop shipped work.
func TestRoadmapDoneGroupsCoverEveryDoneBullet(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "ROADMAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	roadmap, err := loadRoadmap(source)
	if err != nil {
		t.Fatal(err)
	}
	anchors := map[string]bool{}
	total := 0
	for _, group := range roadmap.DoneGroups {
		if group.Name == "" || group.Anchor == "" {
			t.Fatalf("group with empty name or anchor: %+v", group)
		}
		if anchors[group.Anchor] {
			t.Fatalf("duplicate changelog anchor %q", group.Anchor)
		}
		anchors[group.Anchor] = true
		if group.Count != len(group.Items) {
			t.Fatalf("group %q count %d disagrees with %d items", group.Name, group.Count, len(group.Items))
		}
		total += len(group.Items)
	}
	if total != len(roadmap.Sections[0].Items) {
		t.Fatalf("changelog groups hold %d bullets; Done section has %d", total, len(roadmap.Sections[0].Items))
	}
}

func TestWebsiteExamplesUseRepositorySourcesAndActualOutputs(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := loadExamples(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"partials": "12\n12\n12\n1\n1\n12\naDa\n7\n1\n8\ntext-to-int\n3\n", "calls": "Hello, Ada\nHello, Grace\n3\n3\n7\n2\n1\n3\n", "word-defaults": "Hello Grace\ndefault ran\nHello Ada\n1\n1\n", "temporal": "2024-03-10T12:00:00-04:00[America/New_York]\n2024-03-10T13:00:00-04:00[America/New_York]\n", "json-config": "3\ntrue\n{\"retries\":3,\"verbose\":true}\n", "json-tree": "nodes: 12\nservice = ingest\nretries = 1.5e1\nendpoints = list of 4\nflags = object of 1\n{\"service\":\"ingest\",\"retries\":1.5e1,\"endpoints\":[\"primary\",443,null,{\"host\":\"db\",\"port\":5432}],\"flags\":{\"dry-run\":true}}\n", "xml-config": "ingest\n3\ntrue\n2\ndb:5432\nowner unset\n", "maps": "1\n3\nAda: 11\nMorgan has no score\nAda: 11\nGrace: 9\nNancy: 23\n4\n4\n", "choices": "running\nCould not start: network unavailable\n7\n0\n", "records": "Ada, age 36\n", "collections": "3\nGrace\n0: Ada\ncollection-iterating\n1: Grace\ncollection-iterating\n2: Lin\ncollection-iterating\n2\nLin\n1\n5\n0,0: 1\n0,1: 2\n0,2: 3\n1,0: 4\n1,1: 5\n1,2: 6\n6\n7\n12\n", "composition": "32\n", "hello": "Your name: Hello, Ada!\n", "fibonacci": "Fibonacci index (0..92): 55\n", "factorial": "Factorial of (0..20): 120\n", "palindrome": "Text to check: Palindrome\n", "gcd": "First number: Second number: 6\n", "times-table": "Table for: 7 x 1 = 7\n7 x 2 = 14\n7 x 3 = 21\n7 x 4 = 28\n7 x 5 = 35\n7 x 6 = 42\n7 x 7 = 49\n7 x 8 = 56\n7 x 9 = 63\n7 x 10 = 70\n", "text-stats": "3 lines  11 words  64 chars\n", "line-filter": "shipped: inv-1042, inv-1044\n", "score-report": "Ada: 36 / Grace: 9 / Lin: 41 / Nancy: 23\npassing: 3 of 4\ntotal: 100\nfirst above 40: 41\nsomeone is above 40, but not everyone\n", "sorting": "Grace: 23\nNancy: 23\nAda: 41\nLin: 41\nsource unchanged: Ada: 41\nsorted: apple, fig, pear\n", "file-write": "Save failed: file-write\nNothing was written; the report stays in memory.\nIn-memory report:\ninvoices: 2\ncredits: 7\n\n", "paths": "/tmp/zhex-out/june/invoices.csv\n/tmp/zhex-out/june\ninvoices.csv\n.csv\n/var/run/state.json\n/tmp/archive/old.csv\n", "file-work": "work folder: credits.txt, invoices.txt\ninvoices.txt: 12 bytes\nvolume still has space\nwork folder removed\n", "env-config": "title: Weekly status\nmode: set but empty\nlimit: not in the file\nreport configured\n", "modules/main": "First number: Second number: 30\n6\n144\n", "modules/math.test": "math module tests passed\n", "modules/md-report/main": "## Weekly status \\& latency\n\nThree services reported\\; nothing is down\\.\n\n| service            | region   | latency |\n| ------------------ | -------- | ------- |\n| edge\\-cache        | eu\\-west | 12 ms   |\n| search\\-api \\(v2\\) | eu\\-west | 41 ms   |\n| auth               | us\\-east | 87 ms   |\n\n- no renderer\n- no template engine\\, just words\n\n[CommonMark](https://commonmark.org/help)\n\n```zhex\nlet title = md.heading(2, \"Weekly\")\nflow `inline code`\n    text.upper\n```\n", "modules/html-page/main": "<!DOCTYPE html>\n<html>\n<head>\n<title>Zhex services — café &amp; &lt;status&gt;</title>\n</head>\n<body>\n<p>Everything below is escaped unless it arrives through html.raw.</p>\n<ul class=\"status &#34;live&#34; &amp; &lt;fresh&gt;\">\n<li>api: 2 ms</li>\n<li>web: 9 ms</li>\n<li>db: 41 ms &amp; &lt;degraded&gt;</li>\n</ul>\n<hr>\n<a href=\"https://example.io/zhex\">project home</a> / (link rejected: javascript: is not an allowed scheme)\n</body>\n</html>\n", "recovery": "That is not a whole number. I'll use 3 retries.\nRetries: 3\n", "exit-status": "Exit status (0..255): Exiting with status 0\n", "edit-distance": "kitten → sitting: 3\nempty → abc: 3\ncafé → cafe: 1\n", "collatz": "1: 0\n2: 1\n3: 7\n4: 2\n5: 5\n6: 8\n7: 16\n8: 3\n9: 19\n", "life": "Generation 0\n.......\n.......\n...#...\n....#..\n..###..\n.......\n.......\nGeneration 1\n.......\n.......\n.......\n..#.#..\n...##..\n...#...\n.......\nGeneration 2\n.......\n.......\n.......\n....#..\n..#.#..\n...##..\n.......\nGeneration 3\n.......\n.......\n.......\n...#...\n....##.\n...##..\n.......\nGeneration 4\n.......\n.......\n.......\n....#..\n.....#.\n...###.\n.......\n"}
	wantCommand := map[string]string{"text-stats": "zhex run examples/text-stats.zhex examples/text-stats-sample.txt", "modules/main": "zhex run examples/modules/main.zhex", "file-work": "zhex run examples/file-work.zhex .", "env-config": "zhex run examples/env-config.zhex ."}
	if len(data.Examples) != 36 {
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
		if example.ID == "modules/md-report/main" {
			if example.File != "main.zhex" || len(example.Modules) != 1 || example.Modules[0].File != "md.zhex" {
				t.Fatalf("md-report example labels: file %q, modules %+v", example.File, example.Modules)
			}
			module, err := os.ReadFile(filepath.Join(root, "examples", "modules", "md-report", "md.zhex"))
			if err != nil || example.Modules[0].Source != string(module) {
				t.Fatalf("module source drifted for %s: %v", example.ID, err)
			}
		}
		if example.ID == "modules/math.test" {
			if example.File != "math.test.zhex" || len(example.Modules) != 2 || example.Modules[0].File != "math.zhex" || example.Modules[1].File != "test.zhex" {
				t.Fatalf("math.test example labels: file %q, modules %+v", example.File, example.Modules)
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

// TestDemoSpecsAreWellFormedAndOnDisk pins the demos manifest: unique ids,
// every file on disk, each row grouped (a missing group fails the build —
// the page heads every distinct group, so a group-less row would render
// ungrouped), and each generated demo row carries its oracle stdout
// as Life frame output — a drifted row fails go test before the site build.
func TestDemoSpecsAreWellFormedAndOnDisk(t *testing.T) {
	root := filepath.Join("..", "..")
	seen := map[string]bool{}
	for _, spec := range demoSpecs {
		if spec.id == "" {
			t.Fatal("demo spec with empty id")
		}
		if seen[spec.id] {
			t.Fatalf("duplicate demo id %q", spec.id)
		}
		seen[spec.id] = true
		if spec.group == "" {
			t.Fatalf("demo %s: missing group", spec.id)
		}
		if !strings.HasSuffix(spec.file, ".zhex") {
			t.Fatalf("demo %s names non-source file %q", spec.id, spec.file)
		}
		if _, err := os.ReadFile(filepath.Join(root, "examples", spec.file)); err != nil {
			t.Fatalf("demo %s file: %v", spec.id, err)
		}
	}
	demos, err := loadDemos(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(demos) != len(demoSpecs) {
		t.Fatalf("got %d demos, want %d", len(demos), len(demoSpecs))
	}
	for i, demo := range demos {
		if demo.Group != demoSpecs[i].group {
			t.Fatalf("demo %s group %q, want %q", demo.ID, demo.Group, demoSpecs[i].group)
		}
		if !strings.HasPrefix(demo.Output, "Generation 0\n") || !strings.HasSuffix(demo.Output, "\n") {
			t.Fatalf("demo %s output is not Life frame output", demo.ID)
		}
		if demo.Stdin != "" {
			t.Fatalf("demo %s declared stdin %q; demos run without input", demo.ID, demo.Stdin)
		}
	}
}

// TestLoadAocRunsSolutionsBehindTheStatus pins the run-panel rule (#128): a
// non-empty status needs the day's sample file under aoc/2025/samples, a
// part1 or solved day also needs its solution — which supplies Stdin's
// prefill and the Output oracle — and an empty or working day carries
// neither, so its door keeps the honest status line.
func TestLoadAocRunsSolutionsBehindTheStatus(t *testing.T) {
	root := t.TempDir()
	write := func(name, contents string) {
		path := filepath.Join(root, "aoc", "2025", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("samples/day01.txt", "5\n")
	write("day01.zhex", "let n = text.to-int(read-line(\"\") try) try\nprint(n * 2)\n")
	write("samples/day02.txt", "7\n")
	write("day02.zhex", "print(1)\n")

	original := aocSpecs
	defer func() { aocSpecs = original }()

	// All-empty statuses stay byte-empty over a fully stocked tree.
	days, err := loadAoc(root)
	if err != nil {
		t.Fatalf("empty manifest: %v", err)
	}
	if len(days) != len(aocSpecs) {
		t.Fatalf("got %d days, want %d", len(days), len(aocSpecs))
	}
	for _, day := range days {
		if day.Stdin != "" || day.Output != "" || day.Wasm != "" || day.WasmNote != "" {
			t.Fatalf("day %d carries run-panel state behind an empty status: %+v", day.Day, day)
		}
	}

	// part1 and solved run the solution for the oracle; the sample becomes
	// the prefill and the stdin.
	specs := append([]aocSpec(nil), aocSpecs...)
	specs[0].status = "part1"
	specs[1].status = "solved"
	aocSpecs = specs
	days, err = loadAoc(root)
	if err != nil {
		t.Fatalf("stocked manifest: %v", err)
	}
	if days[0].Stdin != "5\n" || days[0].Output != "10\n" {
		t.Fatalf("day 1 stdin %q output %q; want sample prefill and oracle stdout", days[0].Stdin, days[0].Output)
	}
	if days[1].Output != "1\n" {
		t.Fatalf("day 2 output %q; want the solution's stdout", days[1].Output)
	}

	// A solved day without its sample fails, naming the day.
	specs[2].status = "solved"
	aocSpecs = specs
	if _, err := loadAoc(root); err == nil || !strings.Contains(err.Error(), "day 3") || !strings.Contains(err.Error(), "sample") {
		t.Fatalf("missing sample error = %v", err)
	}

	// A part1 day without its solution fails, naming the day.
	write("samples/day03.txt", "1\n")
	specs[2].status = "part1"
	aocSpecs = specs
	if _, err := loadAoc(root); err == nil || !strings.Contains(err.Error(), "day 3") || !strings.Contains(err.Error(), "solution") {
		t.Fatalf("missing solution error = %v", err)
	}

	// A broken solution fails the run, naming the day.
	write("day03.zhex", "print(unknown)\n")
	if _, err := loadAoc(root); err == nil || !strings.Contains(err.Error(), "run aoc day 3") {
		t.Fatalf("broken solution error = %v", err)
	}

	// A working day needs the sample but not the solution.
	specs[2].status = "working"
	aocSpecs = specs
	days, err = loadAoc(root)
	if err != nil {
		t.Fatalf("working day without solution: %v", err)
	}
	if days[2].Stdin != "1\n" || days[2].Output != "" {
		t.Fatalf("working day stdin %q output %q; want sample only", days[2].Stdin, days[2].Output)
	}
}
