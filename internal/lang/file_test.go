package lang

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestFileReadAll(t *testing.T) {
	path := writeFile(t, "héllo\n")
	out, err := runFile(t, "let t = file.read-all(\""+path+"\") try\nprint(t)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "héllo\n\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	_, err = runFile(t, "let t = file.read-all(\""+filepath.Join(t.TempDir(), "missing.txt")+"\") try\nprint(t)\n")
	code, message := codeOf(t, err)
	if code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}
	if !strings.Contains(message, "missing.txt") || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}

	path = writeFile(t, "a\xff")
	_, err = runFile(t, "let t = file.read-all(\""+path+"\") try\nprint(t)\n")
	if code, _ := codeOf(t, err); code != "file-utf8" {
		t.Fatalf("wanted file-utf8, got %s", code)
	}
}

func TestFileReadLines(t *testing.T) {
	path := writeFile(t, "one\r\ntwo\nthree")
	out, err := runFile(t, "let b = file.read-lines(\""+path+"\") try\n"+
		"print(buffer.line-count(b))\n"+
		"print(buffer.line(b, 0) try)\n"+
		"print(buffer.line(b, 1) try)\n"+
		"print(buffer.to-text(b))\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "3\none\ntwo\none\ntwo\nthree\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	_, err = runFile(t, "let b = file.read-lines(\""+filepath.Join(t.TempDir(), "missing.txt")+"\") try\nprint(buffer.line-count(b))\n")
	if code, _ := codeOf(t, err); code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}
}

func TestFileLineReadersAgree(t *testing.T) {
	for _, content := range []string{"", "\n", "one\r\n", "one\r", "\r", "one\r\ntwo\r", "one\n\n", "one\r\r\n"} {
		t.Run(content, func(t *testing.T) {
			path := writeFile(t, content)
			out, err := runFile(t, "let b = file.read-lines(\""+path+"\") try\n"+
				"let f = file.open(\""+path+"\") try\n"+
				"var i = 0\n"+
				"for line in file.lines(\""+path+"\")\n"+
				"    print(line == (buffer.line(b, i) try))\n"+
				"    print(line == (file.read-line(f) try))\n"+
				"    i = i + 1\n"+
				"print(i == buffer.line-count(b))\n"+
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
	// abandoning an fd that would blur TestFileLinesGuaranteedClose.
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

func TestForInFileLines(t *testing.T) {
	path := writeFile(t, "one\r\ntwo\nthree")
	out, err := runFile(t, "for line in file.lines(\""+path+"\")\n    print(line)\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "one\ntwo\nthree\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// The loop binding is a String: it composes with the text words.
	out, err = runFile(t, "for line in file.lines(\""+path+"\")\n    print(text.upper(line))\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ONE\nTWO\nTHREE\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	empty := writeFile(t, "")
	out, err = runFile(t, "for line in file.lines(\""+empty+"\")\n    print(line)\nprint(\"done\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "done\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestFileLinesContextRule(t *testing.T) {
	var out strings.Builder
	err := Run("bad.zhex", "let x = file.lines(\"a\")\n", strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "file.lines is only available in a for loop") {
		t.Fatalf("wrong context diagnostic: %v", err)
	}
	if code, _ := codeOf(t, err); code != "range-context" {
		t.Fatalf("wanted range-context, got %s", code)
	}
	if got := runError(t, "for line in file.lines(1)\n    print(line)\n"); !strings.Contains(got, "file.lines path must have type String") {
		t.Fatalf("wrong path diagnostic: %q", got)
	}
}

func TestFileLinesOpenFailure(t *testing.T) {
	_, err := runFile(t, "for line in file.lines(\"/nonexistent-zhex\")\n    print(line)\n")
	code, message := codeOf(t, err)
	if code != "file-open" {
		t.Fatalf("wanted file-open, got %s", code)
	}
	if !strings.Contains(message, "/nonexistent-zhex") || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}
}

// TestFileLinesGuaranteedClose diffs the open-fd set (by readlink target)
// across the three exit paths — clean finish, mid-stream failure, and a
// failure raised by the loop body: no new fd may reference the fixture.
func TestFileLinesGuaranteedClose(t *testing.T) {
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeFile(t, tc.content)
			before, ok := fdTargets()
			if !ok {
				t.Fatal("/proc/self/fd disappeared")
			}
			_, err := runFile(t, "for line in file.lines(\""+path+"\")\n    "+tc.body+"\n")
			if tc.wantCode == "" {
				if err != nil {
					t.Fatal(err)
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
