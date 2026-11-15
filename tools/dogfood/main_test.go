package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The transcript matrix: diff the prototype against production `zhex repl` on
// the same piped scripts. Both run as subprocesses with stderr sharing
// stdout's file, so the merged transcript records the true write order — that
// is the parity gate. Split-stream runs pin where the bytes went: the only
// accepted divergence is entry diagnostics moving to stdout (Zhex has no
// stderr writer) plus the documented EOF final-line prompt divergence.

var (
	zhexBin    string
	dogfoodBin string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "dogfood-bins")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	for _, build := range []struct{ out, pkg string }{
		{filepath.Join(dir, "zhex"), "zhex/cmd/zhex"},
		{filepath.Join(dir, "dogfood"), "."},
	} {
		cmd := exec.Command("go", "build", "-o", build.out, build.pkg)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic(err)
		}
	}
	zhexBin, dogfoodBin = filepath.Join(dir, "zhex"), filepath.Join(dir, "dogfood")
	os.Exit(m.Run())
}

// runMerged runs one binary over a piped script with stderr sharing stdout's
// file, so the returned transcript records the true write order. The 30s cap
// keeps a future hang from stalling the matrix; healthy runs take milliseconds.
func runMerged(t *testing.T, bin, script, dir string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "repl")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = &out
	cmd.Stderr = &out // same writer: exec shares one fd, keeping write order
	status := exitStatus(t, cmd.Run())
	return out.String(), status
}

// runSplit captures stdout and stderr separately, under the same 30s cap.
func runSplit(t *testing.T, bin, script, dir string) (string, string, int) {
	t.Helper()
	var out, errs bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "repl")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout, cmd.Stderr = &out, &errs
	status := exitStatus(t, cmd.Run())
	return out.String(), errs.String(), status
}

func exitStatus(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	t.Fatal(err)
	return -1
}

// transcript runs both binaries merged for the parity gate.
func transcript(t *testing.T, script, dir string) (prod, dog string) {
	t.Helper()
	prod, _ = runMerged(t, zhexBin, script, dir)
	dog, _ = runMerged(t, dogfoodBin, script, dir)
	return prod, dog
}

func TestTranscriptMatrixWriteOrderParity(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{"binding echo", "let x = 5\nx\n"},
		{"string echo", "\"Ada\" + \" \" + \"L\"\n"},
		{"compatible redefinition", "word f() -> Int\n    1\n\nf()\nword f() -> Int\n    2\n\nf()\n"},
		{"incompatible redefinition", "word f() -> Int\n    1\n\nword f(n: Int) -> Int\n    n\n\n"},
		{"words lists session words", "word f(n: Int) -> Int\n    n\n\nwords\n"},
		{"check error", "let x = \"a\" + true\nlet y = 2\ny\n"},
		{"runtime error keeps effects", "print(\"a\")\n1 / 0\nlet keep = 9\nkeep\n"},
		{"session survives errors", "let x = 1 / 0\nx\n"},
		{"incomplete block blank submits", "word f() -> Int\n    42\n\nf()\n"},
		{"open delimiter blank keeps entry open", "let x = [1,\n2]\n\nx\n"},
		{"comment line", "# just a comment\n1 + 1\n"},
		{"EOF mid-block discards", "word f() -> Int\n    42\n"},
		{"EOF mid-block after body line", "word f() -> Int\n    42\n    43\n"},
		{"EOF unclosed delimiter", "let x = [1,\n"},
		{"EOF unclosed delimiter with body", "let x = [1,\n2\n"},
		{"queued program input", "let n = read-line(\"\") try\nprint(n)\nhello\n"},
		{"queued input then entries", "let n = read-line(\"\") try\nn\n6 * 7\n"},
		{"bye exits zero", "bye()\n"},
		{"console exit status", "console.exit(3)\n"},
		{"console fail message", "console.fail(\"bad thing\")\n"},
		{"invalid exit status", "console.exit(999)\n"},
		{"use module", "use math\nmath.gcd(12, 18)\nwords\n"},
		{"use missing module", "use nosuch\n"},
		{"empty script", ""},
		{"blank lines only", "\n\n\n"},
		{"var assignment", "var count = 1\ncount = count + 1\ncount\n"},
		{"match on choice", "type Status = choice\n    Pending\n    Done\n\nlet s = Status.Pending\nmatch s\n    case Status.Pending\n        print(\"waiting\")\n    case Status.Done\n        print(\"finished\")\n"},
		{"flow entry", "flow \"hello\"\n    text.upper\n    print\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if strings.Contains(tc.script, "use math\n") {
				if err := os.WriteFile(filepath.Join(dir, "math.zhex"), []byte("word gcd(a: Int, b: Int) -> Int\n    if b == 0\n        a\n    else\n        gcd(b, a % b)\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			prod, dog := transcript(t, tc.script, dir)
			if prod != dog {
				t.Fatalf("transcripts differ:\nproduction:\n%q\nprototype:\n%q", prod, dog)
			}
		})
	}
}

func TestTranscriptFinalLineWithoutNewline(t *testing.T) {
	dir := t.TempDir()
	prod, dog := transcript(t, "let x = 5", dir)
	// Accepted EOF divergence: #72's read-line returns a partial final line as
	// a successful read, so the driver prompts once more before clean EOF.
	if dog != prod+"zhex> " {
		t.Fatalf("prototype %q, want production transcript plus one trailing prompt:\n%q", dog, prod+"zhex> ")
	}
}

func TestTranscriptFinalPendingLineWithoutNewline(t *testing.T) {
	dir := t.TempDir()
	prod, dog := transcript(t, "let x = [1,\n2]", dir)
	if dog != prod+"zhex> " {
		t.Fatalf("prototype %q, want production transcript plus one trailing prompt:\n%q", dog, prod+"zhex> ")
	}
}

func TestTranscriptStreamsDiagnosticsMoveToStdout(t *testing.T) {
	script := "print(\"before\")\n1 / 0\n5 + 5\n"
	dir := t.TempDir()
	prodMerged, _ := runMerged(t, zhexBin, script, dir)
	dogMerged, _ := runMerged(t, dogfoodBin, script, dir)
	if prodMerged != dogMerged {
		t.Fatalf("write order differs:\n%q\n%q", prodMerged, dogMerged)
	}
	// The split run pins the divergence: every prototype byte lands on stdout.
	dogOut, dogErr, _ := runSplit(t, dogfoodBin, script, dir)
	if dogOut != dogMerged || dogErr != "" {
		t.Fatalf("prototype split streams out=%q err=%q, merged=%q", dogOut, dogErr, dogMerged)
	}
}

func TestTranscriptConsoleFailKeepsStderr(t *testing.T) {
	script := "console.fail(\"bad thing\")\n"
	dir := t.TempDir()
	for _, bin := range []string{zhexBin, dogfoodBin} {
		out, errs, status := runSplit(t, bin, script, dir)
		if status != 1 {
			t.Fatalf("%s: status %d", bin, status)
		}
		if out != "Zhex REPL — end multi-line entries with a blank line; bye() or Ctrl-D exits.\nzhex> " || errs != "bad thing\n" {
			t.Fatalf("%s: out=%q err=%q", bin, out, errs)
		}
	}
}

func TestTranscriptExitStatuses(t *testing.T) {
	cases := []struct {
		name   string
		script string
		status int
	}{
		{"eof", "", 0},
		{"bye", "bye()\n", 0},
		{"exit three", "console.exit(3)\n", 3},
		{"fail", "console.fail(\"x\")\n", 1},
		{"invalid exit", "console.exit(999)\n", 0}, // a runtime diagnostic; the session continues to EOF
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, bin := range []string{zhexBin, dogfoodBin} {
				if _, status := runMerged(t, bin, tc.script, dir); status != tc.status {
					t.Fatalf("%s: status %d, want %d", bin, status, tc.status)
				}
			}
		})
	}
}

func TestTranscript350Entries(t *testing.T) {
	var script strings.Builder
	for i := 0; i < 350; i++ {
		script.WriteString("let v" + itoa(i) + " = " + itoa(i) + "\n")
	}
	script.WriteString("v349\n")
	dir := t.TempDir()
	prod, dog := transcript(t, script.String(), dir)
	if prod != dog {
		i := commonPrefixLen(prod, dog)
		t.Fatalf("transcripts differ at %d:\nproduction: ...%q\nprototype: ...%q", i, prod[i:min(i+80, len(prod))], dog[i:min(i+80, len(dog))])
	}
	if !strings.Contains(dog, "349\n") {
		t.Fatal("last entry did not run")
	}
}

// The ping recursion — a word whose body re-enters itself through the seam —
// hits the session's entry nesting cap and unwinds as an EnterResult instead
// of exhausting the Go stack; the binary finishes cleanly.
func TestDogfoodPingRecursionHitsNestingCap(t *testing.T) {
	dir := t.TempDir()
	script := "word ping()\n    repl.enter(\"ping()\")\n\nping()\n"
	merged, status := runMerged(t, dogfoodBin, script, dir)
	if status != 0 {
		t.Fatalf("status %d, transcript %s", status, merged)
	}
	if !strings.Contains(merged, "choice(EnterResult.Ran)") || strings.Contains(merged, "goroutine") {
		t.Fatalf("transcript %s", merged)
	}
}

func TestSessionSaveLoadFreshProcesses(t *testing.T) {
	dir := t.TempDir()
	build := "word double(n: Int) -> Int\n    n * 2\n\ntype Point = record\n    x: Int\n    y: Int\n\nvar count = 1\nlet spot = Point(x: 20, y: 22)\nlet b = buffer.new()\nbuffer.add-line(b, \"alpha\")\nbuffer.add-line(b, \"beta\")\nlet tree = Json.List([Json.Number(\"2\"), Json.Text(\"x\")])\nlet boom = error.new(\"app\", \"bad\")\nlet saved = repl.save(\"s.zhex\") try\n"
	merged, status := runMerged(t, dogfoodBin, build, dir)
	if status != 0 {
		t.Fatalf("save run failed: %s", merged)
	}
	source, err := os.ReadFile(filepath.Join(dir, "s.zhex"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"# zhex session v1\n",
		"word double(n: Int) -> Int\n    n * 2\n",
		"type Point = record\n    x: Int\n    y: Int\n",
		"var count: Int = 1\n",
		"let spot: Point = Point(x: 20, y: 22)\n",
		"buffer.add-line(b, \"alpha\")\n",
		"let tree: Json = Json.List([Json.Number(\"2\"), Json.Text(\"x\")])\n",
		"# note: skipped boom:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("export missing %q:\n%s", want, text)
		}
	}
	load := "let loaded = repl.load(\"s.zhex\") try\ndouble(spot.x + spot.y)\ncount = count + 1\ncount\nbuffer.to-text(b)\njson.encode(tree) try\nwords\n"
	dogMerged, status := runMerged(t, dogfoodBin, load, dir)
	if status != 0 {
		t.Fatalf("load run failed: %s", dogMerged)
	}
	for _, want := range []string{"redefined bye\n", "84\n", "2\n", "alpha\nbeta\n", "[2,\"x\"]\n", "double(n: Int) -> Int\n"} {
		if !strings.Contains(dogMerged, want) {
			t.Fatalf("load transcript missing %q:\n%s", want, dogMerged)
		}
	}
}

func TestSessionLoadMissingFileFails(t *testing.T) {
	dir := t.TempDir()
	merged, status := runMerged(t, dogfoodBin, "let loaded = repl.load(\"missing.zhex\") try\n", dir)
	if status != 0 || !strings.Contains(merged, "cannot open \"missing.zhex\"") {
		t.Fatalf("status %d, transcript %s", status, merged)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func commonPrefixLen(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}
