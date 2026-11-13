package lang

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input disconnected") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output disconnected") }

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }

type unreadReader struct{ reads int }

func (r *unreadReader) Read([]byte) (int, error) { r.reads++; return 0, io.EOF }

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestPromptIsFlushedBeforeReading(t *testing.T) {
	var underlying bytes.Buffer
	input := strings.NewReader("Ada\n")
	reader := readerFunc(func(p []byte) (int, error) {
		if underlying.String() != "Name: " {
			return 0, errors.New("prompt was not flushed before reading")
		}
		return input.Read(p)
	})
	err := Run("prompt.zhex", "let name = read-line(\"Name: \") try\nprint(name)\n", reader, bufio.NewWriter(&underlying))
	if err != nil {
		t.Fatal(err)
	}
	if underlying.String() != "Name: Ada\n" {
		t.Fatalf("output %q", underlying.String())
	}
}

func TestIOFailures(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		input        io.Reader
		output       io.Writer
		message      string
	}{
		{"read", "let n = read-line(\"\") try\n", failingReader{}, io.Discard, "input disconnected"},
		{"write", "print(1)\n", strings.NewReader(""), failingWriter{}, "output disconnected"},
		{"short write", "print(1)\n", strings.NewReader(""), shortWriter{}, "short write"},
		{"flush", "print(1)\n", strings.NewReader(""), bufio.NewWriter(failingWriter{}), "output disconnected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Run("io.zhex", tc.source, tc.input, tc.output)
			if err == nil || !strings.Contains(err.Error(), tc.message) || !strings.Contains(err.Error(), "io.zhex:1:") {
				t.Fatalf("expected %q, got %v", tc.message, err)
			}
		})
	}
}

func TestInvalidProgramDoesNotReadInput(t *testing.T) {
	input := &unreadReader{}
	var out bytes.Buffer
	source := "let n = read-line(\"Prompt: \") try\nprint(unknown)\n"
	if err := Run("invalid.zhex", source, input, &out); err == nil {
		t.Fatal("expected static error")
	}
	if input.reads != 0 || out.Len() != 0 {
		t.Fatalf("read %d times, output %q", input.reads, out.String())
	}
}

func TestSharedBufferedInputAndFlushedPrompt(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("command\nAda\nremaining\n"))
	if _, err := input.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	var underlying bytes.Buffer
	output := bufio.NewWriter(&underlying)
	if err := Run("input.zhex", "let name = read-line(\"Name: \") try\nprint(name)\n", input, output); err != nil {
		t.Fatal(err)
	}
	if underlying.String() != "Name: Ada\n" {
		t.Fatalf("output not flushed: %q", underlying.String())
	}
	remaining, err := input.ReadString('\n')
	if err != nil || remaining != "remaining\n" {
		t.Fatalf("remaining input %q, error %v", remaining, err)
	}
}

func TestLongLineAndBareCarriageReturn(t *testing.T) {
	for _, input := range []string{strings.Repeat("x", 100000) + "\n", "name\r"} {
		var out bytes.Buffer
		if err := Run("line.zhex", "let line = read-line(\"\") try\nprint(line)\n", strings.NewReader(input), &out); err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSuffix(input, "\n") + "\n"
		if out.String() != want {
			t.Fatalf("line was altered or truncated: got length %d, want %d", out.Len(), len(want))
		}
	}
}
