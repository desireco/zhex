package lang

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runFile(t *testing.T, source string) (string, error) {
	t.Helper()
	var out strings.Builder
	err := Run("file.zhex", source, strings.NewReader(""), &out)
	return out.String(), err
}

// codeOf reports the JSON code and message of a failed run, so tests can
// assert that the text and JSON renderings agree.
func codeOf(t *testing.T, err error) (code, message string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	report, ok := Report(err)
	if !ok {
		t.Fatalf("error is not a diagnostic: %v", err)
	}
	var fields map[string]any
	if json.Unmarshal([]byte(report), &fields) != nil {
		t.Fatalf("report is not JSON: %s", report)
	}
	if !strings.Contains(err.Error(), fields["message"].(string)) {
		t.Fatalf("text and JSON disagree: %q vs %s", err.Error(), report)
	}
	return fields["code"].(string), fields["message"].(string)
}

func TestFileRead(t *testing.T) {
	path := writeFile(t, "héllo\n")
	out, err := runFile(t, "let t = file.read(\""+path+"\") try\nprint(t)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "héllo\n\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	_, err = runFile(t, "let t = file.read(\""+filepath.Join(t.TempDir(), "missing.txt")+"\") try\nprint(t)\n")
	code, message := codeOf(t, err)
	if code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}
	if !strings.Contains(message, "missing.txt") || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}

	path = writeFile(t, "a\xff")
	_, err = runFile(t, "let t = file.read(\""+path+"\") try\nprint(t)\n")
	if code, _ := codeOf(t, err); code != "file-utf8" {
		t.Fatalf("wanted file-utf8, got %s", code)
	}
}

// TestOldFileNamesAreGone pins the rename and retirement: the previous
// spellings fail checking as unknown words before anything runs.
func TestOldFileNamesAreGone(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"let t = file.read-all(\"x\") try\nprint(t)\n", `unknown word "file.read-all"`},
		{"let b = file.read-lines(\"x\") try\nprint(buffer.line-count(b))\n", `unknown word "file.read-lines"`},
		{"for line in file.lines(\"x\")\n    print(line)\n", `unknown word "file.lines"`},
	} {
		if got := runError(t, tc.source); !strings.Contains(got, tc.want) {
			t.Errorf("source %q: wanted %q, got %q", tc.source, tc.want, got)
		}
	}
}

func TestFileLineReadersAgree(t *testing.T) {
	for _, content := range []string{"", "\n", "one\r\n", "one\r", "\r", "one\r\ntwo\r", "one\n\n", "one\r\r\n"} {
		t.Run(content, func(t *testing.T) {
			path := writeFile(t, content)
			out, err := runFile(t, "let f = file.open(\""+path+"\") try\n"+
				"var i = 0\n"+
				"for line in file.stream(\""+path+"\")\n"+
				"    print(line == (file.read-line(f) try))\n"+
				"    i = i + 1\n"+
				"let last = file.read-line(f) catch problem\n    \"eof\"\n"+
				"print(last == \"eof\")\n"+
				"print(i)\n"+
				"file.close(f)\n")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(out, "false") {
				t.Fatalf("line readers disagree for %q: %s", content, out)
			}
		})
	}
}

func TestFileExplicitLayer(t *testing.T) {
	path := writeFile(t, "first\r\nsecond\n")

	// Open, read chomped lines, close twice: the second close stays Unit.
	out, err := runFile(t, "let f = file.open(\""+path+"\") try\n"+
		"print(file.read-line(f) try)\n"+
		"print(file.read-line(f) try)\n"+
		"file.close(f)\n"+
		"file.close(f)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "first\nsecond\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// Reading after close fails inside the Result with file-read.
	_, err = runFile(t, "let f = file.open(\""+path+"\") try\nfile.close(f)\nlet l = file.read-line(f) try\nprint(l)\n")
	if code, _ := codeOf(t, err); code != "file-read" {
		t.Fatalf("wanted file-read, got %s", code)
	}

	_, err = runFile(t, "let f = file.open(\"/nonexistent-zhex\") try\nprint(file.read-line(f) try)\n")
	if code, _ := codeOf(t, err); code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}

	if got := runError(t, "let f = file.open(\""+path+"\") try\nprint(f)\n"); !strings.Contains(got, "print does not support type File") {
		t.Fatalf("wrong print diagnostic: %q", got)
	}
	if got := runError(t, "let f = file.open(\""+path+"\") try\nif f == f\n    print(1)\n"); !strings.Contains(got, "cannot compare type File") {
		t.Fatalf("wrong equality diagnostic: %q", got)
	}

	// File in word signatures; try unwraps file.open to File and
	// file.read-line to String. The close also keeps this test from
	// abandoning an fd that would blur TestFileStreamGuaranteedClose.
	out, err = runFile(t, "word opener(p: String) -> Result<File, FileError>\n"+
		"    ok(file.open(p) try)\n"+
		"word first-line(f: File) -> Result<String, FileError>\n"+
		"    ok(file.read-line(f) try)\n"+
		"let f = opener(\""+path+"\") try\n"+
		"print(first-line(f) try)\n"+
		"file.close(f)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "first\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestFileReadLineAtEof(t *testing.T) {
	path := writeFile(t, "one\n")
	_, err := runFile(t, "let f = file.open(\""+path+"\") try\n"+
		"print(file.read-line(f) try)\n"+
		"let l = file.read-line(f) try\n"+
		"print(l)\n")
	code, message := codeOf(t, err)
	if code != "file-read" || !strings.Contains(message, "end of file") {
		t.Fatalf("wanted file-read \"end of file\", got %s: %q", code, message)
	}
}

func TestForInFileStream(t *testing.T) {
	path := writeFile(t, "one\r\ntwo\nthree")
	out, err := runFile(t, "for line in file.stream(\""+path+"\")\n    print(line)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "one\ntwo\nthree\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// The loop binding is a String: it composes with the text words.
	out, err = runFile(t, "for line in file.stream(\""+path+"\")\n    print(text.upper(line))\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ONE\nTWO\nTHREE\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	empty := writeFile(t, "")
	out, err = runFile(t, "for line in file.stream(\""+empty+"\")\n    print(line)\nprint(\"done\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "done\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestFileStreamContextRule(t *testing.T) {
	var out strings.Builder
	err := Run("bad.zhex", "let x = file.stream(\"a\")\n", strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "file.stream is only available in a for loop") {
		t.Fatalf("wrong context diagnostic: %v", err)
	}
	if code, _ := codeOf(t, err); code != "range-context" {
		t.Fatalf("wanted range-context, got %s", code)
	}
	if got := runError(t, "for line in file.stream(1)\n    print(line)\n"); !strings.Contains(got, "file.stream path must have type String") {
		t.Fatalf("wrong path diagnostic: %q", got)
	}
}

func TestFileStreamOpenFailure(t *testing.T) {
	_, err := runFile(t, "for line in file.stream(\"/nonexistent-zhex\")\n    print(line)\n")
	code, message := codeOf(t, err)
	if code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}
	if !strings.Contains(message, "/nonexistent-zhex") || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}
}

func readDisk(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFileWriteCreatesTruncatesAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.txt")

	// Omitted append creates a new file with UTF-8 multibyte contents.
	out, err := runFile(t, "file.write(\"γεια 🌍\", \""+path+"\") try\nprint(\"made\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "made\n" || readDisk(t, path) != "γεια 🌍" {
		t.Fatalf("out %q disk %q", out, readDisk(t, path))
	}

	// A second omitted-append write truncates instead of appending.
	if _, err := runFile(t, "file.write(\"new\", \""+path+"\") try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "new" {
		t.Fatalf("omitted append must truncate, disk %q", readDisk(t, path))
	}

	// Explicit append: false also truncates; true appends to the existing file.
	if _, err := runFile(t, "file.write(\"longer\", \""+path+"\", false) try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "longer" {
		t.Fatalf("explicit false must truncate, disk %q", readDisk(t, path))
	}
	if _, err := runFile(t, "file.write(\"!\", \""+path+"\", true) try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "longer!" {
		t.Fatalf("explicit true must append, disk %q", readDisk(t, path))
	}

	// Append creates a missing file (O_APPEND convention).
	appended := filepath.Join(dir, "log.txt")
	if _, err := runFile(t, "file.write(\"first\", \""+appended+"\", true) try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, appended) != "first" {
		t.Fatalf("append must create a missing file, disk %q", readDisk(t, appended))
	}

	// Empty contents truncate an existing file to zero bytes.
	if _, err := runFile(t, "file.write(\"\", \""+path+"\") try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "" {
		t.Fatalf("empty contents must truncate, disk %q", readDisk(t, path))
	}

	// The short-call form writes the same contents, path last.
	if _, err := runFile(t, "file.write \"short\", \""+appended+"\" try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, appended) != "short" {
		t.Fatalf("short form must truncate, disk %q", readDisk(t, appended))
	}

	// A flow passes its seed as the first, data-first argument.
	if _, err := runFile(t, "flow \"from flow\"\n    file.write(\""+path+"\") try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "from flow" {
		t.Fatalf("flow seed must be the contents, disk %q", readDisk(t, path))
	}

	// The write path follows an existing symlink to its target.
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := runFile(t, "file.write(\"via link\", \""+link+"\") try\n"); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, target) != "via link" {
		t.Fatalf("write must follow the symlink, disk %q", readDisk(t, target))
	}

	// A missing parent directory is never created implicitly; both arities
	// report the same failure.
	for _, call := range []string{
		"file.write(\"x\", \"" + filepath.Join(dir, "no-such-dir", "x.txt") + "\") try\n",
		"file.write(\"x\", \"" + filepath.Join(dir, "no-such-dir", "x.txt") + "\", true) try\n",
	} {
		code, message := codeOf(t, runFileErr(t, call))
		if code != "file-write" {
			t.Fatalf("wanted file-write, got %s", code)
		}
		if !strings.Contains(message, "no-such-dir") || !strings.Contains(message, "no such file or directory") {
			t.Fatalf("message must name the path and the OS error: %q", message)
		}
	}
}

// runFileErr runs a program that is expected to fail and returns its error.
func runFileErr(t *testing.T, source string) error {
	t.Helper()
	_, err := runFile(t, source)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

func TestFileWriteFailurePhasesAndDetails(t *testing.T) {
	dir := t.TempDir()

	// The open phase reuses the read-side reason taxonomy, and the failing
	// phase is visible through error.detail.
	path := filepath.Join(dir, "missing", "out.txt")
	out, err := runFile(t, "let u = file.write(\"x\", \""+path+"\") catch problem\n"+
		"    print(error.detail(problem, \"phase\"))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    print(error.detail(problem, \"path\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "open\nnot-found\n" + path + "\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// A write failure names the path and the injected cause.
	origWrite := osWrite
	osWrite = func(f *os.File, b []byte) (int, error) { return 0, errorString("disk on fire") }
	defer func() { osWrite = origWrite }()
	outPath := filepath.Join(dir, "out.txt")
	_, err = runFile(t, "file.write(\"data\", \""+outPath+"\") try\n")
	code, message := codeOf(t, err)
	if code != "file-write" || !strings.Contains(message, "writing "+strconv.Quote(outPath)+" failed") || !strings.Contains(message, "disk on fire") {
		t.Fatalf("wrong write failure: %s %q", code, message)
	}
	out, err = runFile(t, "let u = file.write(\"data\", \""+outPath+"\") catch problem\n    print(error.detail(problem, \"phase\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "write\n" {
		t.Fatalf("write phase detail: got %q", out)
	}

	// A writer that reports success after a partial write is converted to a
	// short-write failure.
	osWrite = func(f *os.File, b []byte) (int, error) { f.Write(b[:3]); return 3, nil }
	_, err = runFile(t, "file.write(\"report\", \""+outPath+"\") try\n")
	code, message = codeOf(t, err)
	if code != "file-write" || !strings.Contains(message, "short write") {
		t.Fatalf("wrong short-write failure: %s %q", code, message)
	}

	// A partial write followed by an error leaves the partial file behind:
	// file.write does not promise atomic replacement.
	osWrite = func(f *os.File, b []byte) (int, error) { f.Write(b[:3]); return 3, errorString("io error") }
	_, err = runFile(t, "file.write(\"report\", \""+outPath+"\") try\n")
	if code, _ := codeOf(t, err); code != "file-write" {
		t.Fatalf("wanted file-write, got %s", code)
	}
	if readDisk(t, outPath) != "rep" {
		t.Fatalf("partial write must stay on disk, disk %q", readDisk(t, outPath))
	}
	osWrite = origWrite

	// A close failure is reported after the contents were written.
	origClose := osClose
	osClose = func(f *os.File) error { return errorString("close failed") }
	defer func() { osClose = origClose }()
	_, err = runFile(t, "file.write(\"done\", \""+outPath+"\") try\n")
	code, message = codeOf(t, err)
	if code != "file-write" || !strings.Contains(message, "closing "+strconv.Quote(outPath)) || !strings.Contains(message, "close failed") {
		t.Fatalf("wrong close failure: %s %q", code, message)
	}
	if readDisk(t, outPath) != "done" {
		t.Fatalf("contents must be on disk before close, disk %q", readDisk(t, outPath))
	}
	osClose = origClose
}

func TestFileWriteArityTypeAndDiscardDiagnostics(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"file.write(\"only\") try\n", "file.write requires 2 to 3 arguments, found 1"},
		{"file.write(\"a\", \"b\", true, 4) try\n", "file.write requires 2 to 3 arguments, found 4"},
		{"file.write(1, \"b\") try\n", "argument 1 of file.write requires type String, found Int"},
		{"file.write(\"a\", 2) try\n", "argument 2 of file.write requires type String, found Int"},
		{"file.write(\"a\", \"b\", 3) try\n", "argument 3 of file.write requires type Bool, found Int"},
		{"file.write(\"a\", \"b\")\n", "Result must be handled or propagated"},
	} {
		if got := runError(t, tc.source); !strings.Contains(got, tc.want) {
			t.Errorf("source %q: wanted %q, got %q", tc.source, tc.want, got)
		}
	}
}

func TestFileWritePartialKeepsDefaultDeferred(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	// The omitted default stays deferred: one-argument invocations truncate.
	source := "let save = file.write _, \"" + path + "\"\n" +
		"save(\"first\") try\n" +
		"save(\"second\") try\n"
	if _, err := runFile(t, source); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "second" {
		t.Fatalf("deferred default must truncate, disk %q", readDisk(t, path))
	}

	// A partial that captures append: true appends per invocation.
	source = "let log = file.write _, \"" + path + "\", true\n" +
		"log(\"a\") try\n" +
		"log(\"b\") try\n"
	if _, err := runFile(t, source); err != nil {
		t.Fatal(err)
	}
	if readDisk(t, path) != "secondab" {
		t.Fatalf("captured append must apply, disk %q", readDisk(t, path))
	}
}

func TestFileWriteErrorFamilyComposition(t *testing.T) {
	// An escaping InputError and FileError in one word need unification.
	source := "word save-arg(i: Int) -> Unit\n" +
		"    let a = console.arg(i) try\n" +
		"    file.write(a, \"out.txt\") try\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "multiple error families") {
		t.Fatalf("wanted mixed family rejection, got %v", err)
	}

	// Wrapping each failure into Error compiles and keeps the origin code.
	dir := t.TempDir()
	source = "word save-arg(i: Int) -> Result<Unit, Error>\n" +
		"    let a = console.arg(i) catch problem\n" +
		"        return err(error.wrap(\"missing-arg\", \"no program argument\", problem))\n" +
		"    let done = file.write(a, \"" + filepath.Join(dir, "no", "out.txt") + "\") catch problem\n" +
		"        return err(error.wrap(\"write-failed\", \"could not write the report\", problem))\n" +
		"    ok(done)\n" +
		"let u = save-arg(0) catch problem\n" +
		"    print(error.code(problem))\n" +
		"    print(error.code(error.cause(problem) try))\n"
	var out strings.Builder
	if err := RunWithArgs("test.zhex", source, strings.NewReader(""), &out, nil, []string{"payload"}); err != nil {
		t.Fatal(err)
	}
	if want := "write-failed\nfile-write\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestFileWriteInfersResultWord(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	source := "word save(text: String, path: String)\n" +
		"    file.write(text, path) try\n" +
		"save(\"body\", \"" + path + "\") try\n" +
		"print(\"saved\")\n"
	var out strings.Builder
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "saved\n" {
		t.Fatalf("got %q", out.String())
	}

	var session strings.Builder
	s := NewSession(strings.NewReader(""), &session, nil, nil)
	if _, err := s.Enter("word save(text: String, path: String)\n    file.write(text, path) try\n\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "save(text: String, path: String) -> Result<Unit, FileError>") {
		t.Fatalf("words %v", s.Words())
	}
}

// TestFileStreamGuaranteedClose diffs the open-fd set (by readlink target)
// across the three exit paths — clean finish, mid-stream failure, and a
// failure raised by the loop body: no new fd may reference the fixture.
func TestFileStreamGuaranteedClose(t *testing.T) {
	fdTargets := func() (map[string]bool, bool) {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			return nil, false
		}
		targets := make(map[string]bool, len(entries))
		for _, e := range entries {
			if link, err := os.Readlink("/proc/self/fd/" + e.Name()); err == nil {
				targets[link] = true
			}
		}
		return targets, true
	}
	if _, ok := fdTargets(); !ok {
		t.Skip("fd counting needs /proc/self/fd")
	}
	for _, tc := range []struct {
		name, content, body string
		wantCode            string
	}{
		{"success", "one\ntwo\n", "print(line)", ""},
		{"utf8 mid-stream", "one\n\xff\n", "print(line)", "file-utf8"},
		{"body failure", "one\ntwo\n", "print(1 / 0)", "division-zero"},
		{"deliberate exit", "one\ntwo\n", "console.exit(23)", "exit"},
		{"deliberate fail", "one\ntwo\n", "console.fail(\"failed\")", "fail"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, tc.content)
			before, ok := fdTargets()
			if !ok {
				t.Fatal("/proc/self/fd disappeared")
			}
			_, err := runFile(t, "for line in file.stream(\""+path+"\")\n    "+tc.body+"\n")
			if tc.wantCode == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if tc.wantCode == "exit" || tc.wantCode == "fail" {
				wantStatus := 23
				if tc.wantCode == "fail" {
					wantStatus = 1
				}
				exit, ok := err.(*ExitError)
				if !ok || exit.Status != wantStatus {
					t.Fatalf("expected exit %d, got %v", wantStatus, err)
				}
			} else if code, _ := codeOf(t, err); code != tc.wantCode {
				t.Fatalf("wanted %s, got %s", tc.wantCode, code)
			}
			after, _ := fdTargets()
			for target := range after {
				if !before[target] && strings.Contains(target, path) {
					t.Fatalf("fd leak: %s still open after the run", target)
				}
			}
		})
	}
}
