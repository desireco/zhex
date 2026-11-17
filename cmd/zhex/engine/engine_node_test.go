package main

// The browser engine's Node-driven gate (#91): the module built from this
// package (GOOS=js GOARCH=wasm) runs under Node through testdata/engine.cjs,
// and every assertion is differential — the interpreter in this process is
// the oracle. It runs under `go test ./...`, so `just test` gates it.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"zhex/internal/lang"
)

// engineRow pins one examples/ file's gate membership, mirroring
// internal/wasm's corpusTable discipline: every .zhex file under examples/
// must have exactly one row. boundary marks the files that use a fence word
// (file.* filesystem words, file.stream, console.arg, env.load) — they run,
// but their output legitimately differs from a filesystem-equipped zhex run,
// so the gate asserts no trap and a coded report instead of byte identity.
// stdin copies the site manifest's values (tools/site-data exampleSpecs);
// keep both in sync when an example's inputs change.
type engineRow struct {
	stdin    string
	boundary bool
}

var engineTable = map[string]engineRow{
	// boundary users: filesystem words and console.arg
	"env-config.zhex": {boundary: true},
	"file-work.zhex":  {boundary: true},
	"file-write.zhex": {boundary: true},
	"filter.zhex":     {boundary: true},
	"text-stats.zhex": {boundary: true},

	// qualifying: output must be byte-identical to the interpreter
	"calls.zhex":                  {},
	"choices.zhex":                {},
	"collatz.zhex":                {},
	"collections.zhex":            {},
	"composition.zhex":            {},
	"demos/acorn.zhex":            {},
	"demos/diehard.zhex":          {},
	"demos/gun.zhex":              {},
	"edit-distance.zhex":          {},
	"exit-status.zhex":            {stdin: "0\n"},
	"factorial.zhex":              {stdin: "5\n"},
	"fibonacci.zhex":              {stdin: "10\n"},
	"fizzbuzz.zhex":               {stdin: "15\n"},
	"gcd.zhex":                    {stdin: "48\n18\n"},
	"hello.zhex":                  {stdin: "Ada\n"},
	"json-config.zhex":            {},
	"json-tree.zhex":              {},
	"life.zhex":                   {},
	"line-filter.zhex":            {},
	"maps.zhex":                   {},
	"modules/html-page/html.zhex": {},
	"modules/html-page/main.zhex": {},
	"modules/main.zhex":           {stdin: "48\n18\n"},
	"modules/math.test.zhex":      {},
	"modules/math.zhex":           {},
	"modules/md-report/main.zhex": {},
	"modules/md-report/md.zhex":   {},
	"modules/test.zhex":           {},
	"palindrome.zhex":             {stdin: "racecar\n"},
	"partials.zhex":               {},
	"paths.zhex":                  {},
	"records.zhex":                {},
	"recovery.zhex":               {},
	"score-report.zhex":           {},
	"sorting.zhex":                {},
	"temporal.zhex":               {},
	"times-table.zhex":            {stdin: "7\n"},
	"word-defaults.zhex":          {},
	"xml-config.zhex":             {},
}

// The build needs the go toolchain and its lib/wasm/wasm_exec.js; the runs
// need node. Each missing piece skips the gate, matching internal/wasm's
// requireTool convention.
var (
	enginePath string
	wasmExecJS string
	engineErr  error
	engineSkip string
)

func TestMain(m *testing.M) {
	if goBin, err := exec.LookPath("go"); err == nil {
		goroot, err := exec.Command(goBin, "env", "GOROOT").Output()
		if err != nil {
			engineErr = err
		} else {
			execJS := filepath.Join(strings.TrimSpace(string(goroot)), "lib", "wasm", "wasm_exec.js")
			if _, statErr := os.Stat(execJS); statErr != nil {
				engineSkip = "wasm_exec.js is not in the Go install"
			} else {
				wasmExecJS = execJS
				build, err := os.MkdirTemp("", "zhex-engine")
				if err != nil {
					engineErr = err
				} else {
					module := filepath.Join(build, "zhex-engine.wasm")
					cmd := exec.Command(goBin, "build", "-o", module, "./cmd/zhex/engine")
					cmd.Dir = filepath.Join("..", "..", "..")
					cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
					if out, err := cmd.CombinedOutput(); err != nil {
						engineErr = fmt.Errorf("GOOS=js go build: %v\n%s", err, out)
						os.RemoveAll(build)
					} else {
						enginePath = module
					}
				}
			}
		}
	}
	code := m.Run()
	if enginePath != "" {
		os.RemoveAll(filepath.Dir(enginePath))
	}
	os.Exit(code)
}

func engineModule(t *testing.T) {
	t.Helper()
	switch {
	case engineSkip != "":
		t.Skipf("%s; skipping", engineSkip)
	case engineErr != nil:
		t.Fatal(engineErr)
	case enginePath == "":
		t.Skip("the engine module was not built; skipping")
	}
}

// runEngineRaw executes the Node driver and returns its stdout.
func runEngineRaw(t *testing.T, mode string, args ...string) []byte {
	t.Helper()
	engineModule(t)
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; skipping")
	}
	driver := append([]string{filepath.Join("testdata", "engine.cjs"), wasmExecJS, enginePath, mode}, args...)
	cmd := exec.Command(node, driver...)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("engine driver (%s): %v\n%s", mode, err, stderr.String())
	}
	return out.Bytes()
}

// TestEngineDifferentialExamples is the acceptance gate (#91 criterion 1):
// every examples/ file without a boundary word runs one-shot in the module
// with stdout and exit status byte-identical to the interpreter; boundary
// users must come back coded, never trapped.
func TestEngineDifferentialExamples(t *testing.T) {
	engineModule(t)
	root := filepath.Join("..", "..", "..", "examples")
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
	pinned := map[string]bool{}
	for rel := range engineTable {
		pinned[rel] = true
	}
	for _, rel := range files {
		if !pinned[rel] {
			t.Errorf("examples/%s is not classified in engineTable; classify it with its site stdin", rel)
		}
		delete(pinned, rel)
	}
	for rel := range pinned {
		t.Errorf("engineTable row examples/%s has no file on disk", rel)
	}
	if t.Failed() {
		t.Fatal("examples walk and engineTable disagree — see the errors above")
	}

	for _, rel := range files {
		row := engineTable[rel]
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(rel))
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var interpreted bytes.Buffer
			runErr := lang.RunWithModules(path, string(source), strings.NewReader(row.stdin), &interpreted, nil, nil, siblingResolve(path))
			wantOut, wantCode := interpreted.String(), exitStatus(runErr)

			dir := t.TempDir()
			stdinPath := filepath.Join(dir, "stdin.txt")
			if err := os.WriteFile(stdinPath, []byte(row.stdin), 0o600); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Output string `json:"output"`
				Report string `json:"report"`
				Trace  string `json:"trace"`
				Exit   int    `json:"exit"`
			}
			if err := json.Unmarshal(runEngineRaw(t, "run", path, stdinPath), &result); err != nil {
				t.Fatal(err)
			}
			if !row.boundary {
				if result.Output != wantOut {
					t.Fatalf("engine output %q, want interpreter output %q", result.Output, wantOut)
				}
				if result.Exit != wantCode {
					t.Fatalf("engine exit %d, want interpreter exit %d", result.Exit, wantCode)
				}
				return
			}
			if result.Exit != 0 && result.Exit != 1 {
				t.Fatalf("boundary example exited %d with report %q", result.Exit, result.Report)
			}
			if result.Report != "" {
				var parsed struct{ Code string }
				if err := json.Unmarshal([]byte(result.Report), &parsed); err != nil {
					t.Fatalf("report %q is not JSON: %v", result.Report, err)
				}
				if !knownCode(parsed.Code) {
					t.Fatalf("report code %q is not in the diagnostic catalog", parsed.Code)
				}
			}
		})
	}
}

// probeResult is one runSource answer.
type probeResult struct {
	Output      string `json:"output"`
	Diagnostics string `json:"diagnostics"`
	Report      string `json:"report"`
	Trace       string `json:"trace"`
	Exit        int    `json:"exit"`
}

// runProbe runs one source string through the module.
func runProbe(t *testing.T, source, stdin string, extra ...string) probeResult {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "probe.zhex")
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, []byte(stdin), 0o600); err != nil {
		t.Fatal(err)
	}
	var result probeResult
	if err := json.Unmarshal(runEngineRaw(t, "run", append([]string{sourcePath, stdinPath}, extra...)...), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

// TestEngineRunSourceTrace pins the opt-in trace: off by default, on it
// streams the --trace JSONL vocabulary.
func TestEngineRunSourceTrace(t *testing.T) {
	source := "let x = 1\nprint(x)\n"
	if result := runProbe(t, source, ""); result.Output != "1\n" || result.Trace != "" {
		t.Fatalf("traceless run gave %+v", result)
	}
	result := runProbe(t, source, "", "--trace")
	if result.Output != "1\n" || !strings.Contains(result.Trace, `"event":"binding"`) {
		t.Fatalf("traced run gave output %q trace %q", result.Output, result.Trace)
	}
}

// TestEngineBoundaryWords pins each fence: every fenced word fails one-shot
// with code not-available-in-build, file.stream the same, while the catch
// path, the lexical path words, and read-line against the supplied stdin
// keep working.
func TestEngineBoundaryWords(t *testing.T) {
	run := func(t *testing.T, source, stdin string) (output, report string, exit int) {
		t.Helper()
		result := runProbe(t, source, stdin)
		return result.Output, result.Report, result.Exit
	}
	reportCode := func(t *testing.T, report string) string {
		t.Helper()
		var parsed struct{ Code string }
		if err := json.Unmarshal([]byte(report), &parsed); err != nil {
			t.Fatalf("report %q is not JSON: %v", report, err)
		}
		return parsed.Code
	}
	fenced := map[string]string{
		"file.read":          `let v = file.read("x") try`,
		"file.open":          `let v = file.open("x") try`,
		"file.write":         `let v = file.write("x", "x") try`,
		"file.list":          `let v = file.list("x") try`,
		"file.exists":        `let v = file.exists("x") try`,
		"file.size":          `let v = file.size("x") try`,
		"file.free-space":    `let v = file.free-space("x") try`,
		"file.make-folder":   `let v = file.make-folder("x") try`,
		"file.delete":        `let v = file.delete("x") try`,
		"file.delete-folder": `let v = file.delete-folder("x") try`,
		"file.move":          `let v = file.move("x", "y") try`,
		"console.arg":        `let v = console.arg(0) try`,
		"env.load":           `let v = env.load("x") try`,
	}
	for word, statement := range fenced {
		t.Run(word, func(t *testing.T) {
			_, report, _ := run(t, statement+"\n", "")
			if code := reportCode(t, report); code != "not-available-in-build" {
				t.Fatalf("report code = %q, want not-available-in-build", code)
			}
		})
	}
	t.Run("file.stream", func(t *testing.T) {
		_, report, _ := run(t, "for line in file.stream(\"x.txt\")\n    print(line)\n", "")
		if code := reportCode(t, report); code != "not-available-in-build" {
			t.Fatalf("report code = %q, want not-available-in-build", code)
		}
	})
	t.Run("catch selects the fallback", func(t *testing.T) {
		output, _, exit := run(t, "let t = file.read(\"x\") catch \"none\"\nprint(t)\n", "")
		if output != "none\n" || exit != 0 {
			t.Fatalf("catch run gave output %q exit %d", output, exit)
		}
	})
	t.Run("lexical path words work", func(t *testing.T) {
		output, _, exit := run(t, "print(file.join(\"a\", \"b\"))\n", "")
		if output != "a/b\n" || exit != 0 {
			t.Fatalf("file.join run gave output %q exit %d", output, exit)
		}
	})
	t.Run("read-line reads the supplied stdin", func(t *testing.T) {
		output, _, exit := run(t, "let name = read-line(\"\") try\nprint(name)\n", "Ada\n")
		if output != "Ada\n" || exit != 0 {
			t.Fatalf("read-line run gave output %q exit %d", output, exit)
		}
	})
	t.Run("read-line past the stdin fails coded end-of-input", func(t *testing.T) {
		output, report, exit := run(t, "let a = read-line(\"\") try\nlet b = read-line(\"\") try\nprint(a)\n", "Ada\n")
		if output != "" || exit != 1 {
			t.Fatalf("second read-line gave output %q exit %d", output, exit)
		}
		if code := reportCode(t, report); code != "console-input" {
			t.Fatalf("report code = %q, want console-input", code)
		}
	})
}

// TestEngineSessionLaws drives scripted entries through the module's session
// seam and through the interpreter's own Session, requiring identical law:
// echo values, the redefined note in the output buffer, module adoption
// through the host resolve hook, stdin reads, survival past a runtime
// failure, and a requested exit.
func TestEngineSessionLaws(t *testing.T) {
	mathPath, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", "modules", "math.zhex"))
	if err != nil {
		t.Fatal(err)
	}
	entries := []map[string]any{
		{"source": "word double(n: Int) -> Int\n    n * 2\n"},
		{"source": "double(21)\n"},
		{"source": "use math\n"},
		{"source": "math.gcd(48, 18)\n"},
		{"source": "word double(n: Int) -> Int\n    n * 3\n", "words": true},
		{"source": "double(21)\n"},
		{"source": "let n = read-line(\"> \") try\nprint(n)\n"},
		{"source": "1 / 0\n"},
		{"source": "2 + 2\n"},
		{"source": "console.exit(3)\n"},
	}
	dir := t.TempDir()
	jobPath := filepath.Join(dir, "job.json")
	job, err := json.Marshal(map[string]any{"stdin": "41\n", "modules": map[string]string{"math": mathPath}, "entries": entries})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobPath, job, 0o600); err != nil {
		t.Fatal(err)
	}

	// the oracle: the same entries against the interpreter's own session
	var out, diagnostics bytes.Buffer
	oracle := lang.NewSession(strings.NewReader("41\n"), &out, nil, func(name string) (lang.ModuleSource, bool) {
		if name != "math" {
			return lang.ModuleSource{}, false
		}
		source, err := os.ReadFile(mathPath)
		if err != nil {
			return lang.ModuleSource{}, false
		}
		return lang.ModuleSource{Path: mathPath, Source: string(source)}, true
	})
	oracle.SetDiagnostics(&diagnostics)
	var want []map[string]any
	for _, entry := range entries {
		echo, err := oracle.Enter(entry["source"].(string))
		result := map[string]any{"echo": echo, "error": "", "report": nil, "exit": nil, "output": out.String(), "diagnostics": diagnostics.String()}
		out.Reset()
		diagnostics.Reset()
		if err != nil {
			var requested *lang.ExitError
			if errors.As(err, &requested) {
				result["exit"] = requested.Status
			} else {
				result["error"] = err.Error()
				if report, ok := lang.Report(err); ok {
					result["report"] = report
				}
			}
		}
		if _, capture := entry["words"]; capture {
			result["words"] = oracle.Words()
		}
		want = append(want, result)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got := runEngineRaw(t, "session", jobPath)
	if !jsonEqual(t, got, wantJSON) {
		t.Fatalf("session results differ:\nengine:      %s\ninterpreter: %s", got, wantJSON)
	}
	joined := fmt.Sprint(want[4]["words"])
	if !strings.Contains(joined, "double(n: Int) -> Int") || !strings.Contains(joined, "math.gcd(a: Int, b: Int) -> Int") {
		t.Fatalf("words listing after adoption misses entries: %v", want[4]["words"])
	}
}

// TestEngineProbes compares the module's continuation probes against the
// interpreter's own IsIncomplete/DelimiterIndent/LastStatementIndent on the
// same sources — the CLI REPL's editable-indentation law.
func TestEngineProbes(t *testing.T) {
	sources := []string{
		"word f() -> Int\n",
		"1 + 1\n",
		"let x = [1,\n",
		"let x = [1]\n",
		"word f() -> Int\n    42\n",
		"let a = 1\nprint(a)\n",
	}
	want := make([]map[string]any, 0, len(sources))
	for _, source := range sources {
		_, err := lang.Compile("repl", source)
		indent, open := lang.DelimiterIndent(source, err)
		last, has := lang.LastStatementIndent(source)
		want = append(want, map[string]any{
			"incomplete":          lang.IsIncomplete(err),
			"delimiterIndent":     nullableInt(indent, open),
			"lastStatementIndent": nullableInt(last, has),
		})
	}
	inputJSON, err := json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	sourcesPath := filepath.Join(dir, "sources.json")
	if err := os.WriteFile(sourcesPath, inputJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	got := runEngineRaw(t, "probes", sourcesPath)
	if !jsonEqual(t, got, wantJSON) {
		t.Fatalf("probe results differ:\nengine:      %s\ninterpreter: %s", got, wantJSON)
	}
}

// siblingResolve mirrors the CLI's module reader for the oracle runs.
func siblingResolve(entryPath string) func(string) (lang.ModuleSource, bool) {
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

func nullableInt(value int, ok bool) any {
	if !ok {
		return nil
	}
	return value
}

func exitStatus(err error) int {
	var requested *lang.ExitError
	if errors.As(err, &requested) {
		return requested.Status
	}
	if err != nil {
		return 1
	}
	return 0
}

func knownCode(code string) bool {
	for _, known := range lang.DiagnosticCodes() {
		if known == code {
			return true
		}
	}
	return false
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		t.Fatalf("engine JSON %q: %v", a, err)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		t.Fatalf("oracle JSON %q: %v", b, err)
	}
	return reflect.DeepEqual(va, vb)
}
