package lang

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// engineBoundaryWords is pinned exactly: the browser engine build fences the
// filesystem words plus console.arg and env.load (itself a file read), and
// nothing else. The pure lexical path words (file.join, file.base,
// file.extension, file.directory-of), read-line against the supplied stdin,
// console.args-count (always 0), and env.get (always Option.None) keep
// working there, so a fence on any of them would break working programs.
const wantBoundaryWords = "console.arg env.load file.delete file.delete-folder file.exists file.free-space file.list file.make-folder file.move file.open file.read file.size file.write file.write-bytes"

func TestEngineBoundaryWordsPinned(t *testing.T) {
	names := make([]string, 0, len(engineBoundaryWords))
	for name := range engineBoundaryWords {
		names = append(names, name)
	}
	sort.Strings(names)
	if got := strings.Join(names, " "); got != wantBoundaryWords {
		t.Fatalf("boundary words = %q, want %q", got, wantBoundaryWords)
	}
	for name := range engineBoundaryWords {
		if !HostWord(name) {
			t.Errorf("%q is not a host word", name)
		}
	}
	for _, working := range []string{"file.join", "file.base", "file.extension", "file.directory-of", "read-line", "console.args-count", "env.get", "file.read-line", "file.close"} {
		if engineBoundaryWords[working] {
			t.Errorf("%q is fenced; it works in the browser engine build", working)
		}
	}
}

// The fence diagnostic itself: the code cataloged for both builds, the
// message naming the word, the hint naming the honest fix, and the JSON
// report carrying the code.
func TestEngineBoundaryDiagnostic(t *testing.T) {
	err := engineBoundary(position{file: "playground", line: 3, column: 5}, "file.read")
	d, ok := err.(*diagnostic)
	if !ok {
		t.Fatalf("boundary error is a %T, want *diagnostic", err)
	}
	if d.code != CodeNotAvailableInBuild {
		t.Fatalf("code = %q, want %q", d.code, CodeNotAvailableInBuild)
	}
	if d.message != "file.read is not available in this build" {
		t.Fatalf("message = %q", d.message)
	}
	if !strings.Contains(d.hint, "zhex run") {
		t.Fatalf("hint %q does not name zhex run", d.hint)
	}
	reportJSON, ok := Report(err)
	if !ok {
		t.Fatalf("boundary error does not report: %v", err)
	}
	var report struct{ Code string }
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		t.Fatal(err)
	}
	if report.Code != string(CodeNotAvailableInBuild) {
		t.Fatalf("report code = %q", report.Code)
	}
}

// The interpreter's own builds never fence a word; only the js engine build
// carries the capability boundary (boundary_js.go).
func TestEngineBoundarySilentOffJs(t *testing.T) {
	for name := range engineBoundaryWords {
		x := &expression{text: name, pos: position{file: "playground", line: 1, column: 1}}
		if err := wordBoundary(x); err != nil {
			t.Errorf("%s fenced on a non-js build: %v", name, err)
		}
	}
	if err := streamBoundary(position{}); err != nil {
		t.Errorf("file.stream fenced on a non-js build: %v", err)
	}
}
