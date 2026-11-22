package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileWriteBytes pins the write-only byte surface: a List<Int> of bytes
// 0..255 lands on disk byte-for-byte (including bytes no UTF-8 text write
// can produce), and an empty list writes an empty file.
func TestFileWriteBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bytes.bin")
	out, err := runFile(t, "file.write-bytes(\""+path+"\", [0, 255, 65, 128, 10]) try\nprint(\"made\")\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "made\n" {
		t.Fatalf("got %q", out)
	}
	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if want := []byte{0, 255, 65, 128, 10}; string(got) != string(want) {
		t.Fatalf("wrote %v, want %v", got, want)
	}

	empty := filepath.Join(t.TempDir(), "empty.bin")
	if _, err := runFile(t, "file.write-bytes(\""+empty+"\", []) try\n"); err != nil {
		t.Fatal(err)
	}
	info, serr := os.Stat(empty)
	if serr != nil {
		t.Fatal(serr)
	}
	if info.Size() != 0 {
		t.Fatalf("empty list wrote %d bytes", info.Size())
	}

	// A second write truncates: no append mode on the lean surface.
	if _, err := runFile(t, "file.write-bytes(\""+path+"\", [7]) try\n"); err != nil {
		t.Fatal(err)
	}
	got, rerr = os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("rewrite left %v, want [7]", got)
	}
}

// TestFileWriteBytesRange pins the value contract: anything outside 0..255
// fails file-bytes-range inside the Result before the file is touched.
func TestFileWriteBytesRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"above", "256"},
		{"below", "-1"},
		{"wide", "170141183460469231731687303715884105727"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "out.bin")
			_, err := runFile(t, "file.write-bytes(\""+path+"\", [65, "+tc.value+"]) try\n")
			code, message := codeOf(t, err)
			if code != "file-bytes-range" {
				t.Fatalf("wanted file-bytes-range, got %s", code)
			}
			if !strings.Contains(message, "outside 0..255") || !strings.Contains(message, tc.value) {
				t.Fatalf("message %q must name the value and the range", message)
			}
			if _, serr := os.Stat(path); !os.IsNotExist(serr) {
				t.Fatalf("range failure must not create %v", serr)
			}
		})
	}

	// The caught code is inspectable like any file failure.
	path := filepath.Join(t.TempDir(), "caught.bin")
	out, err := runFile(t, "let u = file.write-bytes(\""+path+"\", [512]) catch problem\n"+
		"    print(error.code(problem))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "file-bytes-range\n" {
		t.Fatalf("got %q", out)
	}
}

// TestFileWriteBytesFailures mirrors file.write's OS-failure contract: a
// missing parent directory fails file-write inside the Result.
func TestFileWriteBytesFailures(t *testing.T) {
	dir := t.TempDir()
	for _, source := range []string{
		"file.write-bytes(\"" + filepath.Join(dir, "no-such-dir", "x.bin") + "\", [1]) try\n",
	} {
		_, err := runFile(t, source)
		code, _ := codeOf(t, err)
		if code != "file-write" {
			t.Fatalf("wanted file-write, got %s", code)
		}
	}
}

// TestFileWriteBytesContract pins the family contract: the Result must be
// handled, arguments are typed, a propagating word infers the FileError
// family, and a partial with a hole stays callable.
func TestFileWriteBytesContract(t *testing.T) {
	dir := t.TempDir()

	if _, err := runFile(t, "file.write-bytes(\""+filepath.Join(dir, "x.bin")+"\", [1])\n"); err == nil || !strings.Contains(err.Error(), "unhandled-result") {
		t.Fatalf("bare call must be rejected, got %v", err)
	}
	for _, tc := range []struct{ source, want string }{
		{"file.write-bytes(\"p\", [\"a\"]) try\n", "argument-type"},
		{"file.write-bytes(\"p\") try\n", "argument-count"},
		{"file.write-bytes(\"p\", [1], [2]) try\n", "argument-count"},
	} {
		if _, err := runFile(t, tc.source); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: wanted %s, got %v", tc.source, tc.want, err)
		}
	}

	// A word whose write escapes infers Result<Unit, FileError>.
	var session strings.Builder
	s := NewSession(strings.NewReader(""), &session, nil, nil)
	if _, err := s.Enter("word save(path: String)\n    file.write-bytes(path, [104, 105]) try\n\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(s.Words(), "\n"), "save(path: String) -> Result<Unit, FileError>") {
		t.Fatalf("words %v", s.Words())
	}

	// A partial with the path as its hole writes the captured bytes.
	captured := filepath.Join(dir, "captured.bin")
	out, err := runFile(t, "let put = file.write-bytes _, [7, 9]\n"+
		"put(\""+captured+"\") try\n"+
		"print(int.to-string(file.size(\""+captured+"\") try))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "2\n" {
		t.Fatalf("got %q", out)
	}
}
