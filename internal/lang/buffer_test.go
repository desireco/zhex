package lang

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBufferRoundTrip pins the whole seed vocabulary, including that
// buffer.new() parses and runs with zero arguments.
func TestBufferRoundTrip(t *testing.T) {
	source := "let b = buffer.new()\n" +
		"buffer.add-line(b, \"one\")\n" +
		"buffer.add-line(b, \"two\")\n" +
		"print(buffer.line-count(b))\n" +
		"print(buffer.line(b, 0) try)\n" +
		"buffer.set-line(b, 1, \"TWO\")\n" +
		"print(buffer.line(b, 1) try)\n" +
		"print(buffer.to-text(b))\n"
	var out strings.Builder
	if err := Run("buffer.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if want := "2\none\nTWO\none\nTWO\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

// TestBufferAliasing documents the ratified semantics: let binds the name,
// not a copy, so two names can observe one buffer.
func TestBufferAliasing(t *testing.T) {
	source := "let b = buffer.new()\n" +
		"let alias = b\n" +
		"buffer.add-line(alias, \"one\")\n" +
		"buffer.add-line(alias, \"two\")\n" +
		"print(buffer.line-count(b))\n"
	var out strings.Builder
	if err := Run("alias.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if want := "2\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

// TestBufferRange pins the out-of-range policy: the failure lives inside the
// Result and surfaces via try as buffer-range, with text and JSON agreeing.
func TestBufferRange(t *testing.T) {
	for _, tc := range []struct{ name, source, message string }{
		{"line", "let b = buffer.new()\nbuffer.add-line(b, \"one\")\nbuffer.add-line(b, \"two\")\nprint(buffer.line(b, 5) try)\n", "buffer index 5 is outside 0..1"},
		{"set-line", "let b = buffer.new()\nbuffer.add-line(b, \"one\")\nbuffer.set-line(b, -1, \"x\") try\n", "buffer index -1 is outside 0..0"},
		{"empty buffer", "let s = buffer.line(buffer.new(), 0) try\n", "buffer index 0 is outside 0..-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatalf("expected buffer-range, ran clean")
			}
			if !strings.Contains(err.Error(), tc.message) || !strings.Contains(err.Error(), "bad.zhex:") {
				t.Fatalf("wrong diagnostic: %v", err)
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"buffer-range"`) {
				t.Fatalf("wanted code buffer-range, got %s", report)
			}
			var fields map[string]any
			if json.Unmarshal([]byte(report), &fields) != nil || fields["message"] != tc.message {
				t.Fatalf("text and JSON disagree: %s vs %v", err.Error(), fields["message"])
			}
		})
	}
}

func TestBufferRejectedByPrintAndEquality(t *testing.T) {
	got := runError(t, "let b = buffer.new()\nprint(b)\n")
	if !strings.HasPrefix(got, "bad.zhex:2:1: print does not support type Buffer") {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "buffer.to-text") {
		t.Fatalf("hint must name buffer.to-text: %q", got)
	}
	got = runError(t, "let b = buffer.new()\nif b == b\n    print(1)\n")
	if !strings.Contains(got, "cannot compare type Buffer") {
		t.Fatalf("wrong equality diagnostic: %q", got)
	}
}

// TestBufferTypePositions covers Buffer in word signatures and try unwrapping
// buffer.line to String and buffer.set-line to Unit.
func TestBufferTypePositions(t *testing.T) {
	source := "word first(b: Buffer) -> String\n" +
		"    buffer.line(b, 0) try\n" +
		"word fresh() -> Buffer\n" +
		"    buffer.new()\n" +
		"word seed(b: Buffer) -> Unit\n" +
		"    buffer.set-line(b, 0, \"set\") try\n" +
		"let b = fresh()\n" +
		"buffer.add-line(b, \"line\")\n" +
		"seed(b)\n" +
		"print(first(b))\n"
	var out strings.Builder
	if err := Run("types.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if want := "set\n"; out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}
