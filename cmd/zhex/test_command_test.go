package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// buildZhexOnce builds the CLI a single time for the test-command tests;
// every .test.zhex file runs as its own process, so the runner needs a real
// binary to re-execute.
var buildZhexOnce struct {
	sync.Once
	path string
}

func zhexBinary(t *testing.T) string {
	t.Helper()
	buildZhexOnce.Do(func() {
		dir, err := os.MkdirTemp("", "zhex-test-command")
		if err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, "zhex")
		if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
			t.Fatalf("go build: %v: %s", err, out)
		}
		buildZhexOnce.path = bin
	})
	return buildZhexOnce.path
}

// runZhexTest points the runner's re-executable at the built binary.
func runZhexTest(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	useTestBinary(t, zhexBinary(t))
	var out, diagnostics bytes.Buffer
	code := run(args, strings.NewReader(""), &out, &diagnostics)
	return code, out.String(), diagnostics.String()
}

func useTestBinary(t *testing.T, bin string) {
	t.Helper()
	previous := testBinary
	testBinary = func() (string, error) { return bin, nil }
	t.Cleanup(func() { testBinary = previous })
}

func writeZhex(t *testing.T, dir, name, source string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// copyTestModule places the shipped assertion module beside a test file, the
// sibling resolution rule use applies.
func copyTestModule(t *testing.T, dir string) {
	t.Helper()
	source, err := os.ReadFile("../../examples/modules/test.zhex")
	if err != nil {
		t.Fatal(err)
	}
	writeZhex(t, dir, "test.zhex", string(source))
}

const passingTestFile = `use test

test.check(1 + 1 == 2, "arithmetic holds")
print "ok"
`

const failingTestFile = `use test

test.check(1 == 2, "first fails")
print "unreachable"
`

const crashingTestFile = "print(1 / 0)\n"

func TestTestCommandRunsEachTestFileAsItsOwnProcess(t *testing.T) {
	dir := t.TempDir()
	copyTestModule(t, dir)
	crash := writeZhex(t, dir, "crash.test.zhex", crashingTestFile)
	fail := writeZhex(t, dir, "fail.test.zhex", failingTestFile)
	pass := writeZhex(t, dir, "pass.test.zhex", passingTestFile)

	code, out, diagnostics := runZhexTest(t, []string{"test", dir})
	if code != 1 {
		t.Fatalf("code %d, stdout %q stderr %q", code, out, diagnostics)
	}
	want := strings.Join([]string{
		"PASS " + filepath.Join(dir, "test.zhex"),
		"FAIL " + crash + ": " + crash + ":1:9: division or remainder by zero",
		"FAIL " + fail + ": first fails",
		"PASS " + pass,
		"4 files: 2 passed, 2 failed",
	}, "\n") + "\n"
	if out != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out, want)
	}
	// The failing files' captured stderr replays in full, and a crash in one
	// process never stops the others; the passing file's own output stays
	// captured rather than printed.
	for _, want := range []string{"first fails\n", "division or remainder by zero\n", "    print(1 / 0)"} {
		if !strings.Contains(diagnostics, want) {
			t.Fatalf("stderr %q misses %q", diagnostics, want)
		}
	}
	if strings.Contains(out, "ok") {
		t.Fatalf("passing file's stdout must stay captured: %q", out)
	}
}

func TestTestCommandNarrowsToAFileOrFolder(t *testing.T) {
	root := t.TempDir()
	for _, folder := range []string{"a", "b", ".git"} {
		inner := filepath.Join(root, folder)
		if err := os.MkdirAll(inner, 0o755); err != nil {
			t.Fatal(err)
		}
		writeZhex(t, inner, "x.test.zhex", "print \"fine\"\n")
	}

	// A folder narrows to the .test.zhex files under it.
	code, out, diagnostics := runZhexTest(t, []string{"test", filepath.Join(root, "a")})
	if code != 0 || out != "PASS "+filepath.Join(root, "a", "x.test.zhex")+"\n1 files: 1 passed, 0 failed\n" || diagnostics != "" {
		t.Fatalf("folder scope: code %d stdout %q stderr %q", code, out, diagnostics)
	}

	// A single file narrows to that file alone.
	code, out, diagnostics = runZhexTest(t, []string{"test", filepath.Join(root, "b", "x.test.zhex")})
	if code != 0 || out != "PASS "+filepath.Join(root, "b", "x.test.zhex")+"\n1 files: 1 passed, 0 failed\n" || diagnostics != "" {
		t.Fatalf("file scope: code %d stdout %q stderr %q", code, out, diagnostics)
	}

	// The working-folder sweep skips .git, node_modules, _site, editors, and tools.
	for _, skipped := range []string{"node_modules", "_site", "editors", "tools"} {
		if err := os.MkdirAll(filepath.Join(root, skipped), 0o755); err != nil {
			t.Fatal(err)
		}
		writeZhex(t, filepath.Join(root, skipped), "x.test.zhex", "print(\"must not run\")\n")
	}
	code, out, diagnostics = runZhexTest(t, []string{"test", root})
	if code != 0 || !strings.Contains(out, "2 files: 2 passed, 0 failed") || strings.Contains(out, "must not run") || diagnostics != "" {
		t.Fatalf("skip dirs: code %d stdout %q stderr %q", code, out, diagnostics)
	}
}

func TestTestCommandRunsDoctests(t *testing.T) {
	dir := t.TempDir()
	bad := writeZhex(t, dir, "bad.zhex", "## > double(2)\n## 5\nword double(n: Int) -> Int\n    n + n\n")
	good := writeZhex(t, dir, "good.zhex", "## > double(2)\n## 4\nword double(n: Int) -> Int\n    n + n\n")
	noisy := writeZhex(t, dir, "noisy.zhex", "## > noisy()\n## 42\nword noisy() -> Int\n    print(\"should not leak\")\n    42\n")

	code, out, diagnostics := runZhexTest(t, []string{"test", dir})
	if code != 1 {
		t.Fatalf("code %d, stdout %q stderr %q", code, out, diagnostics)
	}
	want := strings.Join([]string{
		"FAIL " + bad + ": " + bad + ":1:1: doctest mismatch in double: expected 5, found 4",
		"PASS " + good,
		"PASS " + noisy,
		"3 files: 2 passed, 1 failed",
	}, "\n") + "\n"
	if out != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", out, want)
	}
	if strings.Contains(out, "should not leak") {
		t.Fatalf("doctest print output leaked to stdout: %q", out)
	}
	if !strings.Contains(diagnostics, "doctest mismatch in double") || !strings.Contains(diagnostics, "hint:") {
		t.Fatalf("stderr %q", diagnostics)
	}
}

func TestTestCommandReportsJSONFailures(t *testing.T) {
	dir := t.TempDir()
	copyTestModule(t, dir)
	writeZhex(t, dir, "bad.zhex", "## > double(2)\n## 5\nword double(n: Int) -> Int\n    n + n\n")
	fail := writeZhex(t, dir, "fail.test.zhex", failingTestFile)
	writeZhex(t, dir, "unknown.test.zhex", "print(missing(1))\n")

	code, out, diagnostics := runZhexTest(t, []string{"test", dir, "--report=json"})
	if code != 1 || out != "" {
		t.Fatalf("code %d stdout %q", code, out)
	}
	lines := strings.Split(strings.TrimSuffix(diagnostics, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want three failure lines, got %q", diagnostics)
	}
	codes := map[string]bool{}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("failure line is not JSON: %q", line)
		}
		var report map[string]any
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			t.Fatalf("failure line %q: %v", line, err)
		}
		codes[report["code"].(string)] = true
		for _, key := range []string{"severity", "file", "line", "column", "message", "hint"} {
			if _, ok := report[key]; !ok {
				t.Fatalf("failure line %q misses %q", line, key)
			}
		}
	}
	// Doctest failures keep their diagnostic envelope, checker failures are
	// relayed from the child process, and a plain failed check becomes
	// test-failed with the captured line in the message.
	for _, code := range []string{"doctest-mismatch", "unknown-word", "test-failed"} {
		if !codes[code] {
			t.Fatalf("codes %v miss %s", codes, code)
		}
	}
	var testFailed string
	for _, line := range lines {
		if strings.Contains(line, "test-failed") {
			testFailed = line
		}
	}
	if !strings.Contains(testFailed, `"file":"`+fail+`"`) || !strings.Contains(testFailed, "first fails") || !strings.Contains(testFailed, `"line":0`) {
		t.Fatalf("test-failed envelope %q", testFailed)
	}
}

func TestTestCommandGapsListsUntestedPublicWords(t *testing.T) {
	dir := t.TempDir()
	copyTestModule(t, dir)
	calc := writeZhex(t, dir, "calc.zhex", strings.Join([]string{
		"## > twice(3)",
		"## 6",
		"word twice(n: Int) -> Int",
		"    n + n",
		"",
		"word thrice(n: Int) -> Int",
		"    n * 3",
		"",
		"word untouched(n: Int) -> Int",
		"    n",
		"",
		"word th(n: Int) -> Int",
		"    n",
		"",
		"word _hidden(n: Int) -> Int",
		"    n",
		"",
	}, "\n"))
	writeZhex(t, dir, "calc.test.zhex", "use calc\nuse test\n\ntest.check(calc.thrice(2) == 6, \"thrice(2) should be 6\")\n")

	// twice is doctested, thrice is referenced by the test file, _hidden is
	// private, and the test module's own check is referenced — only
	// untouched is a gap. The check module's word needs no doctest.
	code, out, diagnostics := runZhexTest(t, []string{"test", dir, "--gaps"})
	if code != 0 || diagnostics != "" {
		t.Fatalf("code %d stderr %q", code, diagnostics)
	}
	want := calc + ": word untouched has no doctest and no test file\n" + calc + ": word th has no doctest and no test file\n"
	if out != want {
		t.Fatalf("stdout %q want %q", out, want)
	}
}

func TestTestCommandGapsReportsBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	writeZhex(t, dir, "broken.zhex", "print(missing(1))\n")

	code, out, diagnostics := runZhexTest(t, []string{"test", dir, "--gaps"})
	if code != 1 || out != "" || !strings.Contains(diagnostics, `unknown word "missing"`) {
		t.Fatalf("code %d stdout %q stderr %q", code, out, diagnostics)
	}
}

func TestTestCommandUsage(t *testing.T) {
	for _, args := range [][]string{
		{"test", "--report=xml", "a.zhex"},
		{"test", "a.zhex", "b.zhex"},
		{"test", "--gaps", "--report=json", "a.zhex"},
	} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
	}
	var out, diagnostics bytes.Buffer
	if code := run([]string{"test", filepath.Join(t.TempDir(), "missing")}, strings.NewReader(""), &out, &diagnostics); code != 1 || !strings.Contains(diagnostics.String(), "missing") {
		t.Fatalf("missing path: code %d stderr %q", code, diagnostics.String())
	}
	dir := t.TempDir()
	notes := writeZhex(t, dir, "notes.txt", "hello\n")
	out, diagnostics = bytes.Buffer{}, bytes.Buffer{}
	if code := run([]string{"test", notes}, strings.NewReader(""), &out, &diagnostics); code != 1 || !strings.Contains(diagnostics.String(), "not a .zhex file") {
		t.Fatalf("non-zhex path: code %d stderr %q", code, diagnostics.String())
	}
}
