package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// trackEnv unsets keys after the test. env.load writes into the process
// environment, and t.Setenv cannot cover that case: it makes a key present,
// which first-wins preserves instead of overwriting.
func trackEnv(t *testing.T, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, key := range keys {
			os.Unsetenv(key)
		}
	})
}

// envShow renders env.get answers so set, set-but-empty, and missing differ.
const envShow = `word show(name: String) -> Unit
    match env.get(name)
        case Option.Some(value)
            print(name + "=" + value)
        case Option.None
            print(name + "=<none>")
`

func TestEnvGet(t *testing.T) {
	t.Setenv("ZHEX_TEST_ENV_SET", "production")
	t.Setenv("ZHEX_TEST_ENV_EMPTY", "")
	source := envShow +
		"show \"ZHEX_TEST_ENV_SET\"\n" +
		"show \"ZHEX_TEST_ENV_EMPTY\"\n" +
		"show \"ZHEX_TEST_ENV_MISSING_6C31\"\n"
	out, err := runFile(t, source)
	if err != nil {
		t.Fatal(err)
	}
	want := "ZHEX_TEST_ENV_SET=production\n" +
		"ZHEX_TEST_ENV_EMPTY=\n" +
		"ZHEX_TEST_ENV_MISSING_6C31=<none>\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestEnvLoad(t *testing.T) {
	keys := []string{"ZHEX_TEST_ENV_PLAIN", "ZHEX_TEST_ENV_SPACED", "ZHEX_TEST_ENV_DOUBLE",
		"ZHEX_TEST_ENV_SINGLE", "ZHEX_TEST_ENV_EMPTY", "ZHEX_TEST_ENV_INDENTED"}
	trackEnv(t, keys...)
	path := envFixture(t, "\n"+
		"   # whole-line comment, skipped after leading trim\n"+
		"ZHEX_TEST_ENV_PLAIN=production\n"+
		"  ZHEX_TEST_ENV_SPACED =  spaced value  \n"+
		"ZHEX_TEST_ENV_DOUBLE=\"double quoted\"\n"+
		"ZHEX_TEST_ENV_SINGLE='single quoted'\n"+
		"ZHEX_TEST_ENV_EMPTY=\n"+
		"    ZHEX_TEST_ENV_INDENTED=indented\n")
	out, err := runFile(t, "env.load(\""+path+"\") try\n"+envShow+
		strings.Join(prefixedEach("show ", keys), ""))
	if err != nil {
		t.Fatal(err)
	}
	want := "ZHEX_TEST_ENV_PLAIN=production\n" +
		"ZHEX_TEST_ENV_SPACED=spaced value\n" +
		"ZHEX_TEST_ENV_DOUBLE=double quoted\n" +
		"ZHEX_TEST_ENV_SINGLE=single quoted\n" +
		"ZHEX_TEST_ENV_EMPTY=\n" +
		"ZHEX_TEST_ENV_INDENTED=indented\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func prefixedEach(prefix string, keys []string) []string {
	out := make([]string, len(keys))
	for i, key := range keys {
		out[i] = prefix + "\"" + key + "\"\n"
	}
	return out
}

func TestEnvLoadPreservesExistingEnvironment(t *testing.T) {
	t.Setenv("ZHEX_TEST_ENV_PRESET", "original")
	trackEnv(t, "ZHEX_TEST_ENV_FRESH")
	path := envFixture(t, "ZHEX_TEST_ENV_PRESET=fromfile\nZHEX_TEST_ENV_FRESH=fresh\n")
	out, err := runFile(t, "env.load(\""+path+"\") try\n"+envShow+
		"show \"ZHEX_TEST_ENV_PRESET\"\nshow \"ZHEX_TEST_ENV_FRESH\"\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "ZHEX_TEST_ENV_PRESET=original\nZHEX_TEST_ENV_FRESH=fresh\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestEnvLoadFirstAssignmentWinsInFile(t *testing.T) {
	trackEnv(t, "ZHEX_TEST_ENV_DUP")
	path := envFixture(t, "ZHEX_TEST_ENV_DUP=first\nZHEX_TEST_ENV_DUP=second\n")
	out, err := runFile(t, "env.load(\""+path+"\") try\n"+envShow+
		"show \"ZHEX_TEST_ENV_DUP\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ZHEX_TEST_ENV_DUP=first\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestEnvLoadMalformedLine(t *testing.T) {
	trackEnv(t, "ZHEX_TEST_ENV_OK")
	for _, tc := range []struct {
		name, content, quoted string
	}{
		{"missing-equals", "ZHEX_TEST_ENV_OK=1\nJUSTAKEY\n", "JUSTAKEY"},
		{"empty-key", "ZHEX_TEST_ENV_OK=1\n=VALUE\n", "=VALUE"},
		{"key-with-spaces", "ZHEX_TEST_ENV_OK=1\nBAD KEY=VALUE\n", "BAD KEY=VALUE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := envFixture(t, tc.content)
			_, err := runFile(t, "env.load(\""+path+"\") try\nprint(\"unreachable\")\n")
			code, message := codeOf(t, err)
			if code != "env-parse" {
				t.Fatalf("wanted env-parse, got %s", code)
			}
			if !strings.Contains(message, "line 2") || !strings.Contains(message, tc.quoted) {
				t.Fatalf("message must name line 2 and the offending text %q: %q", tc.quoted, message)
			}

			// Lines before the malformed one stay applied: the fixture's
			// first line set ZHEX_TEST_ENV_OK before line 2 failed.
			out, err := runFile(t, "env.load(\""+path+"\") catch problem\n    print(\"caught\")\n"+envShow+
				"show \"ZHEX_TEST_ENV_OK\"\n")
			if err != nil {
				t.Fatal(err)
			}
			if want := "caught\nZHEX_TEST_ENV_OK=1\n"; out != want {
				t.Fatalf("lines before the malformed one must stay applied: got %q, want %q", out, want)
			}
		})
	}
}

func TestEnvLoadMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.env")
	_, err := runFile(t, "env.load(\""+missing+"\") try\nprint(\"unreachable\")\n")
	code, message := codeOf(t, err)
	if code != "env-read" {
		t.Fatalf("wanted env-read, got %s", code)
	}
	if !strings.Contains(message, missing) {
		t.Fatalf("message must name the path: %q", message)
	}
}

func TestEnvLoadDefaultPath(t *testing.T) {
	trackEnv(t, "ZHEX_TEST_ENV_DEFAULT")
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("ZHEX_TEST_ENV_DEFAULT=dotfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	out, err := runFile(t, "env.load() try\n"+envShow+"show \"ZHEX_TEST_ENV_DEFAULT\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ZHEX_TEST_ENV_DEFAULT=dotfile\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestEnvPartials(t *testing.T) {
	t.Setenv("ZHEX_TEST_ENV_PARTIAL", "yes")
	trackEnv(t, "ZHEX_TEST_ENV_LOADED")
	path := envFixture(t, "ZHEX_TEST_ENV_LOADED=loaded\n")
	source := "let read = env.get(_)\n" +
		"match read(\"ZHEX_TEST_ENV_PARTIAL\")\n" +
		"    case Option.Some(value)\n" +
		"        print(value)\n" +
		"    case Option.None\n" +
		"        print(\"none\")\n" +
		"let loader = env.load(_)\n" +
		"loader(\"" + path + "\") try\n" +
		"show \"ZHEX_TEST_ENV_LOADED\"\n"
	out, err := runFile(t, envShow+source)
	if err != nil {
		t.Fatal(err)
	}
	if want := "yes\nZHEX_TEST_ENV_LOADED=loaded\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
