package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zhex/internal/lang"
)

// corpusRow pins one examples/ file's native-gate membership (#185),
// mirroring the wasm gate's contract: in = compile, build, and
// byte-compare stdout, stderr, and exit status against the interpreter;
// out = emission refuses with code not-available-in-build naming one of
// the row's blockers. stdin copies the wasm gate's table — the two gates
// share the manifest.
type corpusRow struct {
	in       bool
	stdin    string
	blockers []string
}

// corpusTable walks with the gate: every .zhex file under examples/ must
// have exactly one row, and every row must name a file on disk.
var corpusTable = map[string]corpusRow{
	// in-subset: the wasm gate's 27 minus xml-config (xml.decode is the
	// native blocker), plus the files native covers beyond wasm — word
	// and record defaults, error.*, console.*, env.get — because the Go
	// runtime already implements them
	"calls.zhex":                      {in: true},
	"collatz.zhex":                    {in: true},
	"collections.zhex":                {in: true},
	"composition.zhex":                {in: true},
	"demos/acorn.zhex":                {in: true},
	"demos/diehard.zhex":              {in: true},
	"demos/gun.zhex":                  {in: true},
	"edit-distance.zhex":              {in: true},
	"env-config.zhex":                 {in: true},
	"exit-status.zhex":                {in: true},
	"factorial.zhex":                  {in: true, stdin: "5\n"},
	"file-write.zhex":                 {in: true},
	"fibonacci.zhex":                  {in: true, stdin: "10\n"},
	"filter.zhex":                     {in: true},
	"fizzbuzz.zhex":                   {in: true, stdin: "15\n"},
	"gcd.zhex":                        {in: true, stdin: "48\n18\n"},
	"hello.zhex":                      {in: true, stdin: "Ada\n"},
	"json-tree.zhex":                  {in: true},
	"life.zhex":                       {in: true},
	"line-filter.zhex":                {in: true},
	"maps.zhex":                       {in: true},
	"modules/html-page/html.zhex":     {in: true},
	"modules/html-page/main.zhex":     {in: true},
	"modules/main.zhex":               {in: true, stdin: "48\n18\n"},
	"modules/math.test.zhex":          {in: true},
	"modules/math.zhex":               {in: true},
	"modules/md-report/main.zhex":     {in: true},
	"modules/md-report/md.test.zhex":  {in: true},
	"modules/md-report/md.zhex":       {in: true},
	"modules/md-report/test.zhex":     {in: true},
	"modules/epub-book/main.zhex":     {in: true},
	"modules/epub-book/test.zhex":     {in: true},
	"modules/epub-book/zip.test.zhex": {in: true},
	"modules/epub-book/zip.zhex":      {in: true},
	"modules/test.zhex":               {in: true},
	"palindrome.zhex":                 {in: true, stdin: "racecar\n"},
	"partials.zhex":                   {in: true},
	"paths.zhex":                      {in: true},
	"records.zhex":                    {in: true},
	"recovery.zhex":                   {in: true},
	"score-report.zhex":               {in: true},
	"sorting.zhex":                    {in: true},
	"text-stats.zhex":                 {in: true},
	"times-table.zhex":                {in: true, stdin: "7\n"},
	"with-blocks.zhex":                {in: true},
	"word-defaults.zhex":              {in: true},

	// out-of-subset: the first fence the emitter hits names a blocker
	"choices.zhex":     {blockers: []string{"generic choice"}},
	"file-work.zhex":   {blockers: []string{"file.free-space"}},
	"json-config.zhex": {blockers: []string{"json.decode"}},
	"temporal.zhex":    {blockers: []string{"timezone.load", "zoned.", "date.new", "time.new"}},
	"xml-config.zhex":  {blockers: []string{"xml.decode"}},
}

// corpusResolve mirrors the CLI's module reader.
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

const fenceSuffix = " is not available in this build"

// TestNativeCorpusGate walks every .zhex file under examples/ and asserts
// the pinned membership: in-subset files compile to Go, build, and match
// the interpreter's stdout, stderr, and exit status byte-for-byte;
// out-of-subset files fail emission with code not-available-in-build
// naming a row blocker (#185 acceptance criterion 1).
func TestNativeCorpusGate(t *testing.T) {
	requireGo(t)
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
			t.Errorf("examples/%s is not classified in corpusTable; classify it", rel)
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
				dir := t.TempDir()
				err := Compile(path, string(source), filepath.Join(dir, "out"), corpusResolve(path))
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
				for _, b := range row.blockers {
					if named == b || strings.HasPrefix(named, b) {
						return
					}
				}
				t.Fatalf("fence names %q; want one of %v", named, row.blockers)
			}

			// the oracle: interpreter with the manifest stdin, no args,
			// under the real path so diagnostics agree on file names
			var out, diag strings.Builder
			var wantOut, wantDiag string
			var wantCode int
			runErr := lang.RunWithModulesOptions(path, string(source), strings.NewReader(row.stdin), &out, lang.RunOptions{Diagnostics: &diag}, corpusResolve(path))
			if runErr != nil {
				if exit, ok := runErr.(*lang.ExitError); ok {
					wantCode = exit.Status
				} else {
					wantCode = 1
					fmt.Fprintln(&diag, runErr)
				}
			}
			wantOut, wantDiag = out.String(), diag.String()

			dir := t.TempDir()
			name := strings.TrimSuffix(filepath.Base(rel), ".zhex")
			outDir := filepath.Join(dir, name)
			if err := Compile(path, string(source), outDir, corpusResolve(path)); err != nil {
				t.Fatalf("compile: %v", err)
			}
			stdout, stderr, code := runBinary(t, filepath.Join(outDir, name), row.stdin)
			if stdout != wantOut || stderr != wantDiag || code != wantCode {
				t.Fatalf("native code=%d (want %d)\nnative stdout: %q\ninterp stdout: %q\nnative stderr: %q\ninterp stderr: %q",
					code, wantCode, stdout, wantOut, stderr, wantDiag)
			}
		})
	}
}
