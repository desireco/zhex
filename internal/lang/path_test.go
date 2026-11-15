package lang

import (
	"strings"
	"testing"
)

// The path words are pure lexical operations on the host's native separator;
// these expectations are written for the "/"-separated platforms the suite
// runs on.

func runPathCases(t *testing.T, name string, cases []struct{ source, want string }) {
	t.Helper()
	for _, c := range cases {
		out, err := runFile(t, c.source+"\n")
		if err != nil {
			t.Errorf("%s %s: %v", name, c.source, err)
			continue
		}
		if out != c.want {
			t.Errorf("%s %s: got %q, want %q", name, c.source, out, c.want)
		}
	}
}

func TestFilePathJoin(t *testing.T) {
	runPathCases(t, "join", []struct{ source, want string }{
		// Joining concatenates with the native separator and cleans.
		{`print(file.join("reports", "june.csv"))`, "reports/june.csv\n"},
		{`print(file.join("/opt", "app/config"))`, "/opt/app/config\n"},
		{`print(file.join("/opt/", "app"))`, "/opt/app\n"},
		{`print(file.join("a//b", "c"))`, "a/b/c\n"},
		// Empty sides collapse to the other side.
		{`print(file.join("a", ""))`, "a\n"},
		{`print(file.join("", "b"))`, "b\n"},
		{`print(file.join("", ""))`, "\n"},
		// Lexical .. resolves during the clean; no containment promise.
		{`print(file.join("a/b", "../c"))`, "a/c\n"},
		{`print(file.join("a", ".."))`, ".\n"},
		{`print(file.join("a/b/c", "../.."))`, "a\n"},
		// An absolute child replaces the base (the Python PurePath rule).
		{`print(file.join("a/b", "/c"))`, "/c\n"},
		{`print(file.join("/base", "/child/x"))`, "/child/x\n"},
		{`print(file.join("a/b", "/"))`, "/\n"},
	})
}

func TestFilePathBase(t *testing.T) {
	runPathCases(t, "base", []struct{ source, want string }{
		{`print(file.base("/srv/www/index.html"))`, "index.html\n"},
		{`print(file.base("/a/b/"))`, "b\n"},           // trailing separators removed
		{`print(file.base("/"))`, "/\n"},               // the root is its own base
		{`print(file.base(""))`, ".\n"},                // empty is the working folder
		{`print(file.base("notes.md"))`, "notes.md\n"}, // a bare name is itself
		{`print(file.base("/a/b/.."))`, "..\n"},        // lexical: no clean, .. is an element
		{`print(file.base("/архив/данные.txt"))`, "данные.txt\n"},
	})
}

func TestFilePathExtension(t *testing.T) {
	runPathCases(t, "extension", []struct{ source, want string }{
		{`print(file.extension("/a/b/report.tar.gz"))`, ".gz\n"}, // the final dot wins
		{`print(file.extension("/a/b/notes"))`, "\n"},            // no dot, no extension
		{`print(file.extension(".bashrc"))`, ".bashrc\n"},        // Go rule: the whole dotfile name
		{`print(file.extension("cfg/.hidden.cfg"))`, ".cfg\n"},   // final dot of the final element
		{`print(file.extension("/a/b/"))`, "\n"},                 // the final element is empty
		{`print(file.extension(""))`, "\n"},
		{`print(file.extension("/архив/данные.txt"))`, ".txt\n"}, // UTF-8 components
	})
}

func TestFilePathDirectoryOf(t *testing.T) {
	runPathCases(t, "directory-of", []struct{ source, want string }{
		{`print(file.directory-of("/srv/www/index.html"))`, "/srv/www\n"},
		{`print(file.directory-of("notes.md"))`, ".\n"}, // nothing to drop
		{`print(file.directory-of("/a/b/"))`, "/a/b\n"}, // Go Dir: the trailing element is empty, nothing drops
		{`print(file.directory-of("/"))`, "/\n"},        // nothing above the root
		{`print(file.directory-of("/a"))`, "/\n"},
		{`print(file.directory-of(""))`, ".\n"},
		{`print(file.directory-of("a/b/.."))`, "a/b\n"}, // lexical: no clean
	})
}

func TestFilePathRoundTrip(t *testing.T) {
	// directory-of and base recompose a cleaned path through join.
	out, err := runFile(t, "let path = \"/srv/www/index.html\"\n"+
		"let again = file.join(file.directory-of(path), file.base(path))\n"+
		"print(again)\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "/srv/www/index.html\n" {
		t.Fatalf("absolute round-trip: got %q", out)
	}

	out, err = runFile(t, "let path = \"reports/june.csv\"\n"+
		"print(file.join(file.directory-of(path), file.base(path)))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "reports/june.csv\n" {
		t.Fatalf("relative round-trip: got %q", out)
	}

	out, err = runFile(t, "print(file.directory-of(file.join(\"logs\", \"app/errors.txt\")))\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "logs/app\n" {
		t.Fatalf("directory-of after join: got %q", out)
	}
}

func TestFilePathLexicalOnly(t *testing.T) {
	// No word touches the filesystem: nonexistent paths are ordinary String
	// values, and the + concatenation proves the results are not Results.
	out, err := runFile(t, "let path = file.join(\"/no/such/folder\", \"x.txt\")\n"+
		"print(file.base(path))\n"+
		"print(file.extension(path))\n"+
		"print(file.directory-of(path))\n"+
		"print(path + \"!\")\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "x.txt\n.txt\n/no/such/folder\n/no/such/folder/x.txt!\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	// A flow passes the path as the data-first argument; the word itself can
	// be the step.
	out, err = runFile(t, "flow \"/tmp/backup/x.tar.gz\"\n    file.base\n    print\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != "x.tar.gz\n" {
		t.Fatalf("flow: got %q", out)
	}
}

func TestFilePathTypes(t *testing.T) {
	_, err := runFile(t, "print(file.base(3))\n")
	code, message := codeOf(t, err)
	if code != "argument-type" ||
		!strings.Contains(message, "argument 1 of file.base requires type String, found Int") {
		t.Fatalf("wrong type: %s %q", code, message)
	}
	_, err = runFile(t, "print(file.join(\"a\", 2))\n")
	code, message = codeOf(t, err)
	if code != "argument-type" ||
		!strings.Contains(message, "argument 2 of file.join requires type String, found Int") {
		t.Fatalf("second argument: %s %q", code, message)
	}
	_, err = runFile(t, "print(file.extension(true))\n")
	if code, _ := codeOf(t, err); code != "argument-type" {
		t.Fatalf("Bool extension: %q", err)
	}
	_, err = runFile(t, "print(file.join(\"a\"))\n")
	code, message = codeOf(t, err)
	if code != "argument-count" ||
		!strings.Contains(message, "file.join requires 2 arguments, found 1") {
		t.Fatalf("arity: %s %q", code, message)
	}
	_, err = runFile(t, "print(file.directory-of(\"a\", \"b\"))\n")
	code, message = codeOf(t, err)
	if code != "argument-count" ||
		!strings.Contains(message, "file.directory-of requires 1 arguments, found 2") {
		t.Fatalf("directory-of arity: %s %q", code, message)
	}
}
