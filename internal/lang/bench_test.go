package lang

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// benchFixtureLines is the line count of the generated text-stats fixture.
const benchFixtureLines = 20000

// benchFixture generates the text-stats input once per test or benchmark:
// varied lines with non-ASCII, punctuation, and blanks.
func benchFixture(tb testing.TB) string {
	tb.Helper()
	templates := []string{
		"the café opens at nine and closes at five",
		"naïve parsers trip over Ünicode and π",
		"Zürich reports 42 mm of rain — again!",
		"pack my box with five dozen liquor jugs",
		"the quick brown fox jumps over the lazy dog",
		"数字と文字が混ざった行もある",
		"",
		"tabs\tand  double spaces are just characters",
		"3,14159 radians and 2€ coins count as words",
		"Schrödinger's cat is neither tired nor awake",
	}
	var body strings.Builder
	for i := 0; i < benchFixtureLines; i++ {
		body.WriteString(templates[i%len(templates)])
		body.WriteByte('\n')
	}
	path := filepath.Join(tb.TempDir(), "fixture.txt")
	if err := os.WriteFile(path, []byte(body.String()), 0o644); err != nil {
		tb.Fatal(err)
	}
	return path
}

// TestBenchmarksRunRealPrograms guards the benchmarks against timing no-ops:
// each benchmarked program must run and produce output here.
func TestBenchmarksRunRealPrograms(t *testing.T) {
	statsSrc, err := os.ReadFile("../../examples/text-stats.zhex")
	if err != nil {
		t.Fatal(err)
	}
	fixture := benchFixture(t)

	t.Run("text-stats-counts", func(t *testing.T) {
		var out bytes.Buffer
		if err := RunWithArgs("text-stats.zhex", string(statsSrc), strings.NewReader(""), &out, nil, []string{fixture}); err != nil {
			t.Fatal(err)
		}
		if prefix := "20000 lines  "; !strings.HasPrefix(out.String(), prefix) {
			t.Fatalf("got %q, want prefix %q", out.String(), prefix)
		}
		if !strings.HasSuffix(out.String(), " chars\n") {
			t.Fatalf("got %q", out.String())
		}
	})

	t.Run("text-stats-upper", func(t *testing.T) {
		var out bytes.Buffer
		if err := RunWithArgs("text-stats.zhex", string(statsSrc), strings.NewReader(""), &out, nil, []string{fixture, "upper"}); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(out.String(), "\n"); n != benchFixtureLines {
			t.Fatalf("printed %d lines, want %d", n, benchFixtureLines)
		}
		if strings.ContainsAny(out.String(), "abcdefghijklmnopqrstuvwxyz") {
			t.Fatal("upper mode left lowercase letters in the output")
		}
	})

	t.Run("compile-text-stats", func(t *testing.T) {
		program, err := Compile("text-stats.zhex", string(statsSrc))
		if err != nil {
			t.Fatal(err)
		}
		if program == nil {
			t.Fatal("Compile returned a nil program")
		}
	})

	t.Run("fibonacci", func(t *testing.T) {
		src, err := os.ReadFile("../../examples/fibonacci.zhex")
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := RunWithArgs("fibonacci.zhex", string(src), strings.NewReader("20\n"), &out, nil, nil); err != nil {
			t.Fatal(err)
		}
		if want := "Fibonacci index (0..92): 6765\n"; out.String() != want {
			t.Fatalf("got %q, want %q", out.String(), want)
		}
	})
}

func BenchmarkTextStatsCounts(b *testing.B) {
	src, err := os.ReadFile("../../examples/text-stats.zhex")
	if err != nil {
		b.Fatal(err)
	}
	fixture := benchFixture(b)
	args := []string{fixture}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := RunWithArgs("text-stats.zhex", string(src), strings.NewReader(""), io.Discard, nil, args); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTextStatsUpper(b *testing.B) {
	src, err := os.ReadFile("../../examples/text-stats.zhex")
	if err != nil {
		b.Fatal(err)
	}
	fixture := benchFixture(b)
	args := []string{fixture, "upper"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := RunWithArgs("text-stats.zhex", string(src), strings.NewReader(""), io.Discard, nil, args); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileTextStats(b *testing.B) {
	src, err := os.ReadFile("../../examples/text-stats.zhex")
	if err != nil {
		b.Fatal(err)
	}
	source := string(src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Compile("text-stats.zhex", source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFibonacci(b *testing.B) {
	src, err := os.ReadFile("../../examples/fibonacci.zhex")
	if err != nil {
		b.Fatal(err)
	}
	source := string(src)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := RunWithArgs("fibonacci.zhex", source, strings.NewReader("20\n"), io.Discard, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}
