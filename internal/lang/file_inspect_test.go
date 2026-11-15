package lang

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// makeFolder creates a real temp folder with the named entries: names ending
// in "/" become folders, everything else a file. It returns the folder path.
func makeFolder(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if strings.HasSuffix(name, "/") {
			if err := os.Mkdir(filepath.Join(dir, strings.TrimSuffix(name, "/")), 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func statDenied(name string) error {
	return &fs.PathError{Op: "stat", Path: name, Err: syscall.EACCES}
}

func TestFileList(t *testing.T) {
	// os.ReadDir order is lexical byte order: dot first, digits before
	// letters, uppercase before lowercase, multibyte code points last.
	dir := makeFolder(t, "b.txt", "A.txt", "10.txt", "2.txt", "zé.txt", ".hidden", "nested/")
	out, err := runFile(t, "let names = file.list(\""+dir+"\") try\nprint(text.join(names, \"|\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	want := ".hidden|10.txt|2.txt|A.txt|b.txt|nested|zé.txt\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// An empty folder lists as an empty list.
	empty := t.TempDir()
	out, err = runFile(t, "print(list.length(file.list(\""+empty+"\") try))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "0\n" {
		t.Fatalf("empty folder: got %q", out)
	}

	// Each call returns a fresh list.
	out, err = runFile(t, "let a = file.list(\""+dir+"\") try\n"+
		"let b = file.list(\""+dir+"\") try\n"+
		"list.append(a, \"late\")\n"+
		"print(list.length(a))\nprint(list.length(b))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "8\n7\n" {
		t.Fatalf("fresh lists: got %q", out)
	}

	// The names are Strings and compose with the text words.
	out, err = runFile(t, "let names = file.list(\""+dir+"\") try\nprint(text.upper(names[0]))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != ".HIDDEN\n" {
		t.Fatalf("element type: got %q", out)
	}

	// A flow passes the folder as the data-first argument.
	out, err = runFile(t, "flow \""+dir+"\"\n    file.list try\n    list.length\n    print\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "7\n" {
		t.Fatalf("flow: got %q", out)
	}
}

func TestFileListFailureTaxonomy(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-folder")
	_, err := runFile(t, "let names = file.list(\""+missing+"\") try\nprint(list.length(names))\n")
	code, message := codeOf(t, err)
	if code != "file-list" {
		t.Fatalf("wanted file-list, got %s", code)
	}
	if !strings.Contains(message, missing) || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}

	// A file passed as the folder argument fails with the OS reason; ENOTDIR
	// is neither not-found nor permission, so the reason is other.
	path := writeFile(t, "x")
	out, err := runFile(t, "let names = file.list(\""+path+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    text.split(\"\", \"\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-list\nother\n" {
		t.Fatalf("file as folder: got %q", out)
	}

	// A permission failure classifies as permission-denied.
	origReadDir := osReadDir
	osReadDir = func(string) ([]os.DirEntry, error) { return nil, statDenied("locked") }
	defer func() { osReadDir = origReadDir }()
	out, err = runFile(t, "let names = file.list(\""+filepath.Join(t.TempDir(), "locked")+"\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    text.split(\"\", \"\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "permission-denied\n" {
		t.Fatalf("permission reason: got %q", out)
	}
}

// TestFileListElementLimit pins the shared collection limit: a listing beyond
// the runtime element limit fails inside the Result before materializing.
func TestFileListElementLimit(t *testing.T) {
	origReadDir := osReadDir
	osReadDir = func(string) ([]os.DirEntry, error) {
		return make([]os.DirEntry, maxCollectionElements+1), nil
	}
	defer func() { osReadDir = origReadDir }()
	out, err := runFile(t, "let names = file.list(\"huge\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    text.split(\"\", \"\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "collection-limit\n" {
		t.Fatalf("element limit: got %q", out)
	}
	_, err = runFile(t, "let names = file.list(\"huge\") try\nprint(list.length(names))\n")
	if code, message := codeOf(t, err); code != "collection-limit" || !strings.Contains(message, "file.list exceeds element limit") {
		t.Fatalf("wrong limit failure: %s %q", code, message)
	}
}

func TestFileExists(t *testing.T) {
	dir := makeFolder(t, "a.txt")
	path := filepath.Join(dir, "a.txt")

	// Plain Bool: true for file and folder, false for missing — no Result.
	out, err := runFile(t, "print(file.exists(\""+path+"\"))\n"+
		"print(file.exists(\""+dir+"\"))\n"+
		"print(file.exists(\""+filepath.Join(dir, "missing")+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\ntrue\nfalse\n" {
		t.Fatalf("got %q", out)
	}

	// The plain value is a Bool: usable as a condition and a flow step.
	out, err = runFile(t, "if file.exists(\""+path+"\")\n    print(\"there\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "there\n" {
		t.Fatalf("condition: got %q", out)
	}
	out, err = runFile(t, "flow \""+path+"\"\n    file.exists\n    print\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\n" {
		t.Fatalf("flow: got %q", out)
	}

	// The checked form is still a Result: missing stays the answer false.
	out, err = runFile(t, "print(file.exists(\""+filepath.Join(dir, "missing")+"\") try)\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\n" {
		t.Fatalf("checked missing: got %q", out)
	}
}

func TestFileExistsCheckedVersusPlain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locked")

	// Any stat failure beyond not-found is a runtime diagnostic in the plain
	// form, and a caught FileError in the checked form.
	origStat := osStat
	osStat = func(string) (fs.FileInfo, error) { return nil, statDenied(path) }
	defer func() { osStat = origStat }()

	_, err := runFile(t, "print(file.exists(\""+path+"\"))\n")
	code, message := codeOf(t, err)
	if code != "file-exists" {
		t.Fatalf("wanted file-exists, got %s", code)
	}
	if !strings.Contains(message, path) {
		t.Fatalf("message must name the path: %q", message)
	}

	out, err := runFile(t, "let ok = file.exists(\""+path+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    true\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-exists\npermission-denied\n" {
		t.Fatalf("checked form: got %q", out)
	}

	// try propagates the FileError family to the enclosing word.
	_, err = runFile(t, "word has(p: String)\n    file.exists(p) try\nhas(\""+path+"\") try\n")
	if code, _ := codeOf(t, err); code != "file-exists" {
		t.Fatalf("propagation: wanted file-exists, got %s", code)
	}
}

func TestFileExistsWordSignatures(t *testing.T) {
	// A plain exists keeps a word pure: the success type is Bool.
	source := "word has(p: String)\n    file.exists(p)\nprint(has(\"no-such-zhex-path\"))\n"
	out, err := runFile(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\n" {
		t.Fatalf("pure word: got %q", out)
	}

	// An escaping try infers the effective Result<Bool, FileError> signature.
	var session strings.Builder
	s := NewSession(strings.NewReader(""), &session, nil, nil)
	if _, err := s.Enter("word has(p: String)\n    file.exists(p) try\n\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "has(p: String) -> Result<Bool, FileError>") {
		t.Fatalf("words %v", s.Words())
	}
}

func TestFileSize(t *testing.T) {
	dir := makeFolder(t)
	empty := writeFile(t, "")
	multibyte := writeFile(t, "héllo 🌍\n") // bytes, not code points

	out, err := runFile(t, "print(file.size(\""+empty+"\") try)\n"+
		"print(file.size(\""+multibyte+"\") try)\n"+
		"print(file.size(\""+dir+"\") try)\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "0\n12\n0\n" {
		t.Fatalf("got %q", out)
	}

	// The flow seed is the path.
	out, err = runFile(t, "flow \""+multibyte+"\"\n    file.size try\n    print\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "12\n" {
		t.Fatalf("flow: got %q", out)
	}

	missing := filepath.Join(t.TempDir(), "missing.txt")
	_, err = runFile(t, "print(file.size(\""+missing+"\") try)\n")
	code, message := codeOf(t, err)
	if code != "file-size" {
		t.Fatalf("wanted file-size, got %s", code)
	}
	if !strings.Contains(message, missing) || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}

	out, err = runFile(t, "let n = file.size(\""+missing+"\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    0\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "not-found\n" {
		t.Fatalf("reason: got %q", out)
	}

	origStat := osStat
	osStat = func(string) (fs.FileInfo, error) { return nil, statDenied("locked") }
	defer func() { osStat = origStat }()
	out, err = runFile(t, "let n = file.size(\"locked\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    0\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "permission-denied\n" {
		t.Fatalf("permission reason: got %q", out)
	}
}

func TestFileFreeSpace(t *testing.T) {
	dir := makeFolder(t, "a.txt")

	// Any existing path answers the bytes available on its volume: a folder
	// and a file on the same volume agree.
	out, err := runFile(t, "let free = file.free-space(\""+dir+"\") try\n"+
		"print(free > 0)\n"+
		"print(file.free-space(\""+filepath.Join(dir, "a.txt")+"\") try == free)\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\ntrue\n" {
		t.Fatalf("got %q", out)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	_, err = runFile(t, "print(file.free-space(\""+missing+"\") try)\n")
	code, message := codeOf(t, err)
	if code != "file-free-space" {
		t.Fatalf("wanted file-free-space, got %s", code)
	}
	if !strings.Contains(message, missing) {
		t.Fatalf("message must name the path: %q", message)
	}

	out, err = runFile(t, "let n = file.free-space(\""+missing+"\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    0\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "not-found\n" {
		t.Fatalf("reason: got %q", out)
	}

	origFree := osFreeSpace
	osFreeSpace = func(string) (int64, error) { return 0, statDenied("locked") }
	defer func() { osFreeSpace = origFree }()
	out, err = runFile(t, "let n = file.free-space(\"locked\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    0\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "permission-denied\n" {
		t.Fatalf("permission reason: got %q", out)
	}
}

// TestFileListInfersResultWord pins the inferred effective signature, like
// file.write's.
func TestFileListInfersResultWord(t *testing.T) {
	dir := makeFolder(t, "a.txt", "b.txt")
	source := "word names(folder: String)\n" +
		"    file.list(folder) try\n" +
		"let names = names(\"" + dir + "\") try\n" +
		"print(list.length(names))\n"
	var out strings.Builder
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatalf("got %q", out.String())
	}

	var session strings.Builder
	s := NewSession(strings.NewReader(""), &session, nil, nil)
	if _, err := s.Enter("word names(folder: String)\n    file.list(folder) try\n\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "names(folder: String) -> Result<List<String>, FileError>") {
		t.Fatalf("words %v", s.Words())
	}
}

func TestFileInspectCallDiagnostics(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ source, want string }{
		{"print(file.list(\"x\", \"y\") try)\n", "file.list requires 1 arguments, found 2"},
		{"print(file.list(1) try)\n", "argument 1 of file.list requires type String, found Int"},
		{"print(file.exists(1))\n", "argument 1 of file.exists requires type String, found Int"},
		{"print(file.exists())\n", "file.exists requires 1 arguments, found 0"},
		{"print(file.size(1) try)\n", "argument 1 of file.size requires type String, found Int"},
		{"print(file.free-space(1) try)\n", "argument 1 of file.free-space requires type String, found Int"},
		{"file.list(\"" + dir + "\")\n", "Result must be handled or propagated"},
		{"file.size(\"" + dir + "\")\n", "Result must be handled or propagated"},
		{"file.free-space(\"" + dir + "\")\n", "Result must be handled or propagated"},
	} {
		if got := runError(t, tc.source); !strings.Contains(got, tc.want) {
			t.Errorf("source %q: wanted %q, got %q", tc.source, tc.want, got)
		}
	}
}
