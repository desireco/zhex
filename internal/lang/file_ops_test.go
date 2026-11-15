package lang

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFileMakeFolder(t *testing.T) {
	// One level by default; the parent must already exist.
	dir := t.TempDir()
	target := filepath.Join(dir, "work")
	out, err := runFile(t, "print(file.exists(\""+target+"\"))\n"+
		"file.make-folder(\""+target+"\") try\n"+
		"print(file.exists(\""+target+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\ntrue\n" {
		t.Fatalf("one level: got %q", out)
	}

	// The trailing parents default is false: one level only, missing parent
	// fails inside the Result.
	missing := filepath.Join(dir, "no", "child")
	_, err = runFile(t, "file.make-folder(\""+missing+"\") try\nprint(\"made\")\n")
	code, message := codeOf(t, err)
	if code != "file-make-folder" {
		t.Fatalf("wanted file-make-folder, got %s", code)
	}
	if !strings.Contains(message, missing) || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the path and the OS error: %q", message)
	}

	// parents: true (positional) creates every missing ancestor.
	deep := filepath.Join(dir, "a", "b", "c")
	out, err = runFile(t, "file.make-folder(\""+deep+"\", true) try\n"+
		"print(file.exists(\""+deep+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\n" {
		t.Fatalf("parents: got %q", out)
	}

	// parents: true tolerates an already-existing folder, like os.MkdirAll.
	out, err = runFile(t, "file.make-folder(\""+target+"\", true) try\n"+
		"print(\"ok\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "ok\n" {
		t.Fatalf("existing with parents: got %q", out)
	}

	// parents: false fails on an existing target with the OS EEXIST error.
	_, err = runFile(t, "file.make-folder(\""+target+"\") try\n")
	if code, _ := codeOf(t, err); code != "file-make-folder" {
		t.Fatalf("existing without parents: wanted file-make-folder, got %s", code)
	}

	// An existing file at the target is a collision under both modes.
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"file.make-folder(\"" + blocked + "\") try\n",
		"file.make-folder(\"" + blocked + "\", true) try\n",
	} {
		if _, err := runFile(t, source); err == nil {
			t.Fatalf("file collision accepted: %q", source)
		}
	}

	// A flow passes the path as the first argument of a bare step.
	flowTarget := filepath.Join(dir, "flow")
	out, err = runFile(t, "flow \""+flowTarget+"\"\n"+
		"    file.make-folder try\n"+
		"print(file.exists(\""+flowTarget+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\n" {
		t.Fatalf("flow: got %q", out)
	}
}

func TestFileMakeFolderFailureTaxonomy(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	child := filepath.Join(missing, "child")
	_, err := runFile(t, "file.make-folder(\""+child+"\") try\n")
	code, message := codeOf(t, err)
	if code != "file-make-folder" || !strings.Contains(message, "cannot create folder") {
		t.Fatalf("wanted file-make-folder naming the operation, got %s %q", code, message)
	}

	out, err := runFile(t, "let made = file.make-folder(\""+child+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    print(error.detail(problem, \"path\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-make-folder\nnot-found\n"+child+"\n" {
		t.Fatalf("details: got %q", out)
	}

	origMkdir := osMkdir
	osMkdir = func(string, os.FileMode) error {
		return &fs.PathError{Op: "mkdir", Path: "locked", Err: syscall.EACCES}
	}
	defer func() { osMkdir = origMkdir }()
	out, err = runFile(t, "let made = file.make-folder(\"locked\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "permission-denied\n" {
		t.Fatalf("permission reason: got %q", out)
	}
}

func TestFileDelete(t *testing.T) {
	dir := t.TempDir()

	// A file and an empty folder both delete; os.Remove semantics.
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runFile(t, "file.delete(\""+file+"\") try\n"+
		"file.delete(\""+empty+"\") try\n"+
		"print(file.exists(\""+file+"\"))\n"+
		"print(file.exists(\""+empty+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\nfalse\n" {
		t.Fatalf("file and empty folder: got %q", out)
	}

	// A nonempty folder is the safety default's failure.
	nonempty := makeFolder(t, "nested/", "nested/inner.txt")
	_, err = runFile(t, "file.delete(\""+nonempty+"\") try\n")
	code, message := codeOf(t, err)
	if code != "file-delete" {
		t.Fatalf("nonempty folder: wanted file-delete, got %s", code)
	}
	if !strings.Contains(message, nonempty) || !strings.Contains(message, "not empty") {
		t.Fatalf("message must name the path and the not-empty reason: %q", message)
	}

	// A missing path fails not-found inside the Result.
	missing := filepath.Join(t.TempDir(), "gone")
	out, err = runFile(t, "let gone = file.delete(\""+missing+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    print(error.detail(problem, \"path\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-delete\nnot-found\n"+missing+"\n" {
		t.Fatalf("missing: got %q", out)
	}
}

func TestFileDeleteFolder(t *testing.T) {
	// The default removes an empty folder and refuses a nonempty one.
	empty := t.TempDir()
	inner := filepath.Join(empty, "inner")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runFile(t, "file.delete-folder(\""+inner+"\") try\n"+
		"print(file.exists(\""+inner+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\n" {
		t.Fatalf("empty folder: got %q", out)
	}

	nonempty := makeFolder(t, "sub/", "sub/one.txt", "sub/two.txt")
	_, err = runFile(t, "file.delete-folder(\""+filepath.Join(nonempty, "sub")+"\") try\n")
	code, message := codeOf(t, err)
	if code != "file-delete-folder" {
		t.Fatalf("nonempty default: wanted file-delete-folder, got %s", code)
	}
	if !strings.Contains(message, "not empty") {
		t.Fatalf("message must carry the not-empty reason: %q", message)
	}

	// recursive: true removes the whole tree in one deliberate call.
	tree := makeFolder(t, "work/", "work/a.txt", "work/nested/", "work/nested/b.txt")
	out, err = runFile(t, "file.delete-folder(\""+filepath.Join(tree, "work")+"\", true) try\n"+
		"print(file.exists(\""+filepath.Join(tree, "work")+"\"))\n"+
		"print(file.exists(\""+filepath.Join(tree, "work", "nested", "b.txt")+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "false\nfalse\n" {
		t.Fatalf("recursive tree: got %q", out)
	}

	// recursive: true on a missing path is a no-op success, like os.RemoveAll.
	out, err = runFile(t, "file.delete-folder(\""+filepath.Join(t.TempDir(), "gone")+"\", true) try\n"+
		"print(\"clean\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "clean\n" {
		t.Fatalf("recursive missing: got %q", out)
	}
}

// TestFileDeleteFolderRecursiveBestEffort pins the documented best-effort
// contract: RemoveAll may delete part of the tree before the first failure,
// and the failure reports file-delete-folder with the reason taxonomy.
func TestFileDeleteFolderRecursiveBestEffort(t *testing.T) {
	work := makeFolder(t, "work/", "work/a.txt", "work/b.txt", "work/sub/", "work/sub/c.txt")
	target := filepath.Join(work, "work")

	origRemoveAll := osRemoveAll
	osRemoveAll = func(path string) error {
		// Best-effort depth-first: one entry is gone before the failure.
		_ = os.Remove(filepath.Join(path, "a.txt"))
		return &fs.PathError{Op: "remove", Path: path, Err: syscall.EACCES}
	}
	defer func() { osRemoveAll = origRemoveAll }()

	out, err := runFile(t, "let gone = file.delete-folder(\""+target+"\", true) catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-delete-folder\npermission-denied\n" {
		t.Fatalf("first failure: got %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(target, "a.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("partial deletion must be observable: a.txt stat %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(target, "b.txt")); statErr != nil {
		t.Fatalf("earlier effects stand, b.txt must remain: %v", statErr)
	}
}

func TestFileMove(t *testing.T) {
	dir := t.TempDir()

	// A file moves and lands readable under the new name.
	source := filepath.Join(dir, "draft.txt")
	target := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(source, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runFile(t, "file.move(\""+source+"\", \""+target+"\") try\n"+
		"print(file.read(\""+target+"\") try)\n"+
		"print(file.exists(\""+source+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "body\nfalse\n" {
		t.Fatalf("file move: got %q", out)
	}

	// A folder moves with its contents; rename(2) covers both kinds.
	folder := t.TempDir()
	from := filepath.Join(folder, "pack")
	into := filepath.Join(folder, "archived")
	if err := os.Mkdir(from, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from, "data.txt"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runFile(t, "file.move(\""+from+"\", \""+into+"\") try\n"+
		"print(file.read(\""+filepath.Join(into, "data.txt")+"\") try)\n"+
		"print(file.exists(\""+from+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "kept\nfalse\n" {
		t.Fatalf("folder move: got %q", out)
	}

	// A flow passes the source as the first argument; the step adds the
	// target.
	flowSource := filepath.Join(dir, "flow.txt")
	flowTarget := filepath.Join(dir, "flow-moved.txt")
	if err := os.WriteFile(flowSource, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runFile(t, "flow \""+flowSource+"\"\n"+
		"    file.move(\""+flowTarget+"\") try\n"+
		"print(file.exists(\""+flowTarget+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "true\n" {
		t.Fatalf("flow move: got %q", out)
	}

	// A missing source fails not-found inside the Result.
	missing := filepath.Join(t.TempDir(), "gone.txt")
	_, err = runFile(t, "file.move(\""+missing+"\", \""+filepath.Join(t.TempDir(), "t.txt")+"\") try\n")
	code, message := codeOf(t, err)
	if code != "file-move" {
		t.Fatalf("missing source: wanted file-move, got %s", code)
	}
	if !strings.Contains(message, missing) || !strings.Contains(message, "no such file or directory") {
		t.Fatalf("message must name the source and the OS error: %q", message)
	}
}

func TestFileMoveOverwriteContracts(t *testing.T) {
	dir := t.TempDir()

	// overwrite: false (the default) refuses an existing target and leaves
	// both files in place.
	source := filepath.Join(dir, "new.txt")
	target := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(source, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runFile(t, "let moved = file.move(\""+source+"\", \""+target+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-move\nother\n" {
		t.Fatalf("existing target default: got %q", out)
	}
	if readDisk(t, target) != "old" {
		t.Fatalf("refused move must leave the target: %q", readDisk(t, target))
	}
	if readDisk(t, source) != "new" {
		t.Fatalf("refused move must leave the source: %q", readDisk(t, source))
	}

	// overwrite: true replaces an existing file.
	out, err = runFile(t, "file.move(\""+source+"\", \""+target+"\", true) try\n"+
		"print(file.read(\""+target+"\") try)\n"+
		"print(file.exists(\""+source+"\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "new\nfalse\n" {
		t.Fatalf("overwrite file: got %q", out)
	}

	// An existing folder target fails under both overwrite values: overwrite
	// replaces files only, and the source stays in place.
	for _, mode := range []string{
		"file.move(\"%s\", \"%s\") catch problem\n    print(\"refused\")\n",
		"file.move(\"%s\", \"%s\", true) catch problem\n    print(\"refused\")\n",
	} {
		s := filepath.Join(dir, "s.txt")
		folderTarget := filepath.Join(dir, "folder-target")
		if err := os.WriteFile(s, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(folderTarget, 0o755); err != nil {
			t.Fatal(err)
		}
		source := strings.Replace(mode, "%s", s, 1)
		source = strings.Replace(source, "%s", folderTarget, 1)
		out, err = runFile(t, source)
		if err != nil {
			t.Fatal(err)
		}
		if out != "refused\n" {
			t.Fatalf("folder target must be refused, got %q", out)
		}
		if readDisk(t, s) != "x" {
			t.Fatalf("folder-target refusal must leave the source: %q", readDisk(t, s))
		}
		if _, statErr := os.Stat(folderTarget); statErr != nil {
			t.Fatalf("folder target must survive: %v", statErr)
		}
	}
}

func TestFileMoveFailureTaxonomy(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "s.txt")
	target := filepath.Join(dir, "t.txt")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Cross-device moves fail with the platform's error: no copy fallback.
	origRename := osRename
	osRename = func(oldname, newname string) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EXDEV}
	}
	defer func() { osRename = origRename }()
	out, err := runFile(t, "let moved = file.move(\""+source+"\", \""+target+"\") catch problem\n"+
		"    print(error.code(problem))\n"+
		"    print(error.detail(problem, \"reason\"))\n"+
		"    print(error.detail(problem, \"target\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-move\nother\n"+target+"\n" {
		t.Fatalf("cross-device: got %q", out)
	}
	if readDisk(t, source) != "x" {
		t.Fatalf("cross-device refusal must leave the source: %q", readDisk(t, source))
	}

	// A rename the platform refuses classifies by the shared taxonomy.
	osRename = func(oldname, newname string) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: syscall.EACCES}
	}
	out, err = runFile(t, "let moved = file.move(\""+source+"\", \""+target+"\") catch problem\n"+
		"    print(error.detail(problem, \"reason\"))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "permission-denied\n" {
		t.Fatalf("permission reason: got %q", out)
	}
}

// TestFileOpsWordSignatures pins the inferred effective signature and the
// propagation of the FileError family through try, like file.write's.
func TestFileOpsWordSignatures(t *testing.T) {
	dir := t.TempDir()
	source := "word ensure(folder: String)\n" +
		"    file.make-folder(folder, true) try\n" +
		"ensure(\"" + filepath.Join(dir, "w") + "\") try\n" +
		"print(\"ready\")\n"
	var out strings.Builder
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ready\n" {
		t.Fatalf("got %q", out.String())
	}

	var session strings.Builder
	s := NewSession(strings.NewReader(""), &session, nil, nil)
	if _, err := s.Enter("word ensure(folder: String)\n    file.make-folder(folder, true) try\n\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "ensure(folder: String) -> Result<Unit, FileError>") {
		t.Fatalf("words %v", s.Words())
	}

	// A word mixing a destructive file family with InputError is rejected.
	mixed := "word prepare(i: Int) -> Unit\n" +
		"    let a = console.arg(i) try\n" +
		"    file.delete(a) try\n"
	if _, err := Compile("test.zhex", mixed); err == nil || !strings.Contains(err.Error(), "multiple error families") {
		t.Fatalf("wanted mixed family rejection, got %v", err)
	}
}

func TestFileOpsDefaultsAndPartials(t *testing.T) {
	dir := t.TempDir()

	// Omitted mode defaults are false: one level, nonrecursive, no overwrite.
	nested := filepath.Join(dir, "a", "b")
	if _, err := runFile(t, "file.make-folder(\""+nested+"\") try\n"); err == nil {
		t.Fatal("omitted parents default must stay false")
	}
	leaf := filepath.Join(dir, "leaf")
	occupied := filepath.Join(leaf, "x.txt")
	out, err := runFile(t, "file.make-folder(\""+leaf+"\", true) try\n"+
		"file.write(\"x\", \""+occupied+"\") try\n"+
		"let gone = file.delete-folder(\""+leaf+"\") catch problem\n"+
		"    print(\"kept by default\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "kept by default\n" {
		t.Fatalf("omitted recursive default: got %q", out)
	}

	// A partial keeps the omitted default deferred across invocations.
	deferred := filepath.Join(dir, "deferred.txt")
	collision := filepath.Join(dir, "collision.txt")
	if err := os.WriteFile(collision, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := "let mover = file.move _, \"" + collision + "\"\n" +
		"let refused = mover(\"" + deferred + "\") catch problem\n" +
		"    print(\"refused\")\n"
	out, err = runFile(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if out != "refused\n" {
		t.Fatalf("deferred overwrite default: got %q", out)
	}

	// A captured mode argument applies on every invocation.
	if err := os.WriteFile(deferred, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	source = "let replacer = file.move _, \"" + collision + "\", true\n" +
		"replacer(\"" + deferred + "\") try\n" +
		"print(file.read(\"" + collision + "\") try)\n"
	out, err = runFile(t, source)
	if err != nil {
		t.Fatal(err)
	}
	if out != "new\n" {
		t.Fatalf("captured overwrite: got %q", out)
	}
}

func TestFileOpsCallDiagnostics(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ source, want string }{
		{"file.make-folder() try\n", "file.make-folder requires 1 to 2 arguments, found 0"},
		{"file.make-folder(\"a\", true, 1) try\n", "file.make-folder requires 1 to 2 arguments, found 3"},
		{"file.make-folder(1) try\n", "argument 1 of file.make-folder requires type String, found Int"},
		{"file.make-folder(\"a\", 1) try\n", "argument 2 of file.make-folder requires type Bool, found Int"},
		{"file.delete(\"a\", \"b\") try\n", "file.delete requires 1 arguments, found 2"},
		{"file.delete(1) try\n", "argument 1 of file.delete requires type String, found Int"},
		{"file.delete-folder() try\n", "file.delete-folder requires 1 to 2 arguments, found 0"},
		{"file.delete-folder(\"a\", 1) try\n", "argument 2 of file.delete-folder requires type Bool, found Int"},
		{"file.move(\"a\") try\n", "file.move requires 2 to 3 arguments, found 1"},
		{"file.move(\"a\", \"b\", true, 4) try\n", "file.move requires 2 to 3 arguments, found 4"},
		{"file.move(1, \"b\") try\n", "argument 1 of file.move requires type String, found Int"},
		{"file.move(\"a\", 2) try\n", "argument 2 of file.move requires type String, found Int"},
		{"file.move(\"a\", \"b\", 3) try\n", "argument 3 of file.move requires type Bool, found Int"},
		{"file.make-folder(\"" + dir + "\")\n", "Result must be handled or propagated"},
		{"file.delete(\"" + filepath.Join(dir, "x") + "\")\n", "Result must be handled or propagated"},
		{"file.delete-folder(\"" + dir + "\")\n", "Result must be handled or propagated"},
		{"file.move(\"a\", \"b\")\n", "Result must be handled or propagated"},
	} {
		if got := runError(t, tc.source); !strings.Contains(got, tc.want) {
			t.Errorf("source %q: wanted %q, got %q", tc.source, tc.want, got)
		}
	}
}
