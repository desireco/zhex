package wasm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zhex/internal/lang"
)

// corpusRow pins one examples/ file's gate membership (DESIGN-M4 §6): in =
// compile and byte-compare stdout plus exit status against the interpreter;
// out = emission refuses with code not-available-in-build and names one of
// the row's blockers. stdin copies tools/site-data's exampleSpecs — the
// manifest is the human-facing matrix, this table is the enforced one; keep
// both in sync when an example's inputs change.
type corpusRow struct {
	in       bool
	stdin    string
	blockers []string
}

// corpusTable walks with the gate: every .zhex file under examples/ must
// have exactly one row, and every row must name a file on disk — a new
// corpus file breaks the gate until classified here (zero silently-skipped
// files). Blocker spellings are the emitter's fence names, prefix-matched
// because some carry a dynamic suffix (e.g. "generic choice Maybe").
var corpusTable = map[string]corpusRow{
	// in-subset (DESIGN-M4 §1 after M4): 27 files, no args, stdin per the
	// manifest; xml-config flipped in with xml.decode and the checked list
	// read (DESIGN-M4 §1 "In-subset after M4 — +1"); the demos/*.zhex rows
	// are the demos page's runs (#116, trimmed by #118) — demos, not
	// examples-page cards
	"collatz.zhex":                {in: true},
	"composition.zhex":            {in: true},
	"demos/acorn.zhex":            {in: true},
	"demos/diehard.zhex":          {in: true},
	"demos/gun.zhex":              {in: true},
	"edit-distance.zhex":          {in: true},
	"factorial.zhex":              {in: true, stdin: "5\n"},
	"fibonacci.zhex":              {in: true, stdin: "10\n"},
	"fizzbuzz.zhex":               {in: true, stdin: "15\n"},
	"gcd.zhex":                    {in: true, stdin: "48\n18\n"},
	"hello.zhex":                  {in: true, stdin: "Ada\n"},
	"json-tree.zhex":              {in: true},
	"life.zhex":                   {in: true},
	"line-filter.zhex":            {in: true},
	"maps.zhex":                   {in: true},
	"modules/main.zhex":           {in: true, stdin: "48\n18\n"},
	"modules/math.zhex":           {in: true},
	"modules/md-report/main.zhex": {in: true},
	"modules/md-report/md.zhex":   {in: true},
	"palindrome.zhex":             {in: true, stdin: "racecar\n"},
	"paths.zhex":                  {in: true},
	"records.zhex":                {in: true},
	"recovery.zhex":               {in: true},
	"score-report.zhex":           {in: true},
	"sorting.zhex":                {in: true},
	"times-table.zhex":            {in: true, stdin: "7\n"},
	"xml-config.zhex":             {in: true},

	// out-of-subset (DESIGN-M4 §1 table): the first fence the emitter hits
	// must name one of the row's blockers — emission order picks which
	"calls.zhex":                  {blockers: []string{"word parameter defaults"}},
	"choices.zhex":                {blockers: []string{"generic choice"}},
	"collections.zhex":            {blockers: []string{"checked list.append", "error.code", "array.shape", "array.resize"}},
	"env-config.zhex":             {blockers: []string{"console.arg"}},
	"exit-status.zhex":            {blockers: []string{"error.code", "console.exit"}},
	"file-work.zhex":              {blockers: []string{"console.arg", "file.free-space"}},
	"file-write.zhex":             {blockers: []string{"error.code"}},
	"filter.zhex":                 {blockers: []string{"console.args-count", "console.fail", "console.arg"}},
	"json-config.zhex":            {blockers: []string{"record field defaults"}},
	"partials.zhex":               {blockers: []string{"word parameter defaults", "error.code"}},
	"temporal.zhex":               {blockers: []string{"timezone.load", "zoned.", "date.new", "time.new"}},
	"text-stats.zhex":             {blockers: []string{"console.arg", "console.args-count"}},
	"word-defaults.zhex":          {blockers: []string{"word parameter defaults"}},
	"modules/html-page/html.zhex": {blockers: []string{"error.new"}},
	"modules/html-page/main.zhex": {blockers: []string{"error.new"}},
	"modules/math.test.zhex":      {blockers: []string{"console.fail"}},
	"modules/test.zhex":           {blockers: []string{"console.fail"}},
}

// corpusResolve mirrors the CLI's module reader: `use name` resolves the
// sibling name.zhex next to the entry file, in the real examples tree.
func corpusResolve(entryPath string) func(string) (lang.ModuleSource, bool) {
	dir := filepath.Dir(entryPath)
	return func(name string) (lang.ModuleSource, bool) {
		path := filepath.Join(dir, name+".zhex")
		source, err := os.ReadFile(path)
		if err != nil {
			return lang.ModuleSource{}, false
		}
		return lang.ModuleSource{Path: path, Source: string(source)}, true
	}
}

// fenceSuffix terminates every fence diagnostic; the text before it names
// the fenced construct (watFence in internal/lang/wat.go).
const fenceSuffix = " is not available in this build"

// fenceMatches reports whether the named construct is a member of the row's
// blocker set: equal, or a blocker spelling used as a prefix ("generic
// choice" matches the named "generic choice Maybe").
func fenceMatches(named string, blockers []string) bool {
	for _, b := range blockers {
		if named == b || strings.HasPrefix(named, b) {
			return true
		}
	}
	return false
}

// TestCorpusGate (DESIGN-M4 §6) walks every .zhex file under examples/ and
// asserts the pinned membership: in-subset files compile to both skins and
// match the interpreter's stdout bytes and exit status under the browser
// host (and wasmtime, skip-safe); out-of-subset files fail emission with
// code not-available-in-build naming a row blocker.
func TestCorpusGate(t *testing.T) {
	requireTool(t, "wasm-tools")
	root := filepath.Join("..", "..", "examples")

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".zhex") {
			files = append(files, filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, root), "/")))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no .zhex files under %s", root)
	}
	pinned := map[string]bool{}
	for rel := range corpusTable {
		pinned[rel] = true
	}
	for _, rel := range files {
		if !pinned[rel] {
			t.Errorf("examples/%s is not classified in corpusTable; classify it per DESIGN-M4 §1", rel)
		}
		delete(pinned, rel)
	}
	for rel := range pinned {
		t.Errorf("corpusTable row examples/%s has no file on disk", rel)
	}
	if t.Failed() {
		t.Fatal("corpus walk and table disagree — see the errors above")
	}

	for _, rel := range files {
		row := corpusTable[rel]
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(rel))
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !row.in {
				outPath := filepath.Join(t.TempDir(), "out.wasm")
				err := Compile(path, string(source), outPath, corpusResolve(path), lang.WatSkinBrowser)
				if err == nil {
					t.Fatalf("out-of-subset file compiled; expected a not-available-in-build fence naming one of %v", row.blockers)
				}
				reportJSON, ok := lang.Report(err)
				if !ok {
					t.Fatalf("fence error is not a diagnostic: %v", err)
				}
				var report struct{ Code, Message string }
				if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
					t.Fatal(err)
				}
				if report.Code != "not-available-in-build" {
					t.Fatalf("fence code = %q, want not-available-in-build (%v)", report.Code, err)
				}
				named := strings.TrimSuffix(report.Message, fenceSuffix)
				if named == report.Message {
					t.Fatalf("diagnostic is not a fence: %q", report.Message)
				}
				if !fenceMatches(named, row.blockers) {
					t.Fatalf("fence names %q; want one of %v", named, row.blockers)
				}
				return
			}

			// the oracle: interpreter with the manifest stdin, no args
			var interpreted strings.Builder
			runErr := lang.RunWithModules(path, string(source), strings.NewReader(row.stdin), &interpreted, nil, nil, corpusResolve(path))
			wantOut, wantCode := interpreted.String(), exitCode(runErr)

			dir := t.TempDir()
			stdinPath := filepath.Join(dir, "stdin.txt")
			if err := os.WriteFile(stdinPath, []byte(row.stdin), 0o600); err != nil {
				t.Fatal(err)
			}

			browser := filepath.Join(dir, "browser.wasm")
			if err := Compile(path, string(source), browser, corpusResolve(path), lang.WatSkinBrowser); err != nil {
				t.Fatalf("compile browser skin: %v", err)
			}
			out, _, code := runNode(t, browser, "--stdin", stdinPath)
			if out != wantOut || code != wantCode {
				t.Fatalf("browser skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
			}

			wasi := filepath.Join(dir, "wasi.wasm")
			if err := Compile(path, string(source), wasi, corpusResolve(path), lang.WatSkinWASI); err != nil {
				t.Fatalf("compile wasi skin: %v", err)
			}
			out, _, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
			if out != wantOut || code != wantCode {
				t.Fatalf("wasi skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
			}
		})
	}
}

// TestDifferentialM4 runs the DESIGN-M4 §8 fixtures: xml.decode coverage
// and its pinned failure lines, the trio witness, and both NBSP quoting
// witnesses — browser host and wasmtime (skip-safe) against the
// interpreter oracle. Failure rows pin the exact stderr after the M1
// convention (runtime error: plus the interpreter's first line minus its
// position); success rows exit 0 with empty stderr.
func TestDifferentialM4(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct{ name, stderr string }{
		{name: "xml"},
		{name: "trio"},
		{name: "decode-alias"},
		{name: "xml-fail", stderr: "runtime error: unknown element \"ghost\" at \"/ghost\"\n"},
		{name: "xml-syntax", stderr: "runtime error: invalid XML: XML syntax error on line 1: element <body> closed by </doc> at \"\"\n"},
		{name: "xml-entity", stderr: "runtime error: invalid XML: XML syntax error on line 1: invalid character entity &bogus; at \"\"\n"},
		{name: "xml-dtd", stderr: "runtime error: DTD and entity declarations are rejected at \"\"\n"},
		{name: "nbsp", stderr: "runtime error: \"\\u00a0x\" is not a signed 64-bit decimal integer\n"},
		{name: "nbsp-file", stderr: "runtime error: cannot open \"\\u00a0name.txt\": open  name.txt: no such file or directory\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.name+".zhex")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var interpreted strings.Builder
			runErr := lang.Run(path, string(source), strings.NewReader(""), &interpreted)
			wantOut, wantCode := interpreted.String(), exitCode(runErr)

			dir := t.TempDir()
			stdinPath := filepath.Join(dir, "stdin.txt")
			if err := os.WriteFile(stdinPath, nil, 0o600); err != nil {
				t.Fatal(err)
			}

			browser := filepath.Join(dir, "browser.wasm")
			if err := Compile(path, string(source), browser, nil, lang.WatSkinBrowser); err != nil {
				t.Fatalf("compile browser skin: %v", err)
			}
			out, stderr, code := runNode(t, browser, "--stdin", stdinPath)
			if out != wantOut || code != wantCode || stderr != tc.stderr {
				t.Fatalf("browser skin: code=%d (want %d) stderr=%q (want %q)\ninterpreted: %q\ncompiled:    %q", code, wantCode, stderr, tc.stderr, wantOut, out)
			}

			wasi := filepath.Join(dir, "wasi.wasm")
			if err := Compile(path, string(source), wasi, nil, lang.WatSkinWASI); err != nil {
				t.Fatalf("compile wasi skin: %v", err)
			}
			out, stderr, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
			if out != wantOut || code != wantCode || stderr != tc.stderr {
				t.Fatalf("wasi skin: code=%d (want %d) stderr=%q (want %q)\ninterpreted: %q\ncompiled:    %q", code, wantCode, stderr, tc.stderr, wantOut, out)
			}
		})
	}
}
