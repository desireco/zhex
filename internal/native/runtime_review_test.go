package native

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeFileReadLineFailuresCanBeCaught(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.txt")
	invalid := filepath.Join(dir, "invalid.txt")
	for path, contents := range map[string][]byte{valid: []byte("hello\n"), invalid: {0xff, '\n'}} {
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, path, setup string }{
		{"closed", valid, "file.close(f)\n"},
		{"invalid-utf8", invalid, ""},
		{"read-error", dir, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("let f = file.open(%q) try\n%slet line = file.read-line(f) catch e\n    print(error.code(e))\n    print(error.detail(e, \"reason\"))\n    \"fallback\"\nprint(line)\nfile.close(f)\n", tc.path, tc.setup)
			diffCase(t, "file-read-line-"+tc.name, source, "")
		})
	}
}

func TestNativeFileCloseProducesUnit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diffCase(t, "close-unit", fmt.Sprintf("word consume(value: Unit) -> Int\n    7\nlet f = file.open(%q) try\nprint(consume(file.close(f)))\n", path), "")
}

func TestNativeBuiltinFailureInspection(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"parse", "let n = text.to-int(\"x\") catch e\n    print(error.operation(e))\n    0\nprint(n)\n"},
		{"partial-parse", "let parse = text.to-int(_)\nlet n = parse(\"x\") catch e\n    print(error.operation(e))\n    0\nprint(n)\n"},
		{"input", "let line = read-line(\"\") catch e\n    print(error.operation(e))\n    print(error.detail(e, \"reason\"))\n    \"fallback\"\nprint(line)\n"},
		{"argument", "let arg = console.arg(9223372036854775808) catch e\n    print(error.operation(e))\n    print(error.detail(e, \"index\"))\n    print(error.detail(e, \"argument-count\"))\n    \"fallback\"\nprint(arg)\n"},
		{"buffer-line", "let b = buffer.new()\nlet line = buffer.line(b, 9) catch e\n    print(error.operation(e))\n    print(error.detail(e, \"index\"))\n    print(error.detail(e, \"line-count\"))\n    \"fallback\"\nprint(line)\n"},
		{"file-open", fmt.Sprintf("let f = file.open(%q) catch e\n    print(error.operation(e))\n    console.exit(0)\n", filepath.Join(t.TempDir(), "missing"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diffCase(t, "failure-inspection-"+tc.name, tc.source, "")
		})
	}
}
