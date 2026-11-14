package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestReplPTYHelper(t *testing.T) {
	if os.Getenv("ZHEX_REPL_PTY_HELPER") != "1" {
		return
	}
	os.Exit(run([]string{"repl"}, os.Stdin, os.Stdout, os.Stderr))
}

type ptySession struct {
	tty     *os.File
	cmd     *exec.Cmd
	output  chan []byte
	pending []byte
}

func startPTY(t *testing.T) *ptySession {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestReplPTYHelper$")
	cmd.Env = append(os.Environ(), "ZHEX_REPL_PTY_HELPER=1", "TERM=xterm")
	tty, err := pty.Start(cmd)
	if err != nil {
		t.Fatal(err)
	}
	s := &ptySession{tty: tty, cmd: cmd, output: make(chan []byte, 32)}
	go func() {
		b := make([]byte, 1024)
		for {
			n, err := tty.Read(b)
			if n > 0 {
				data := append([]byte(nil), b[:n]...)
				s.output <- data
			}
			if err != nil {
				close(s.output)
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = tty.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	s.until(t, "zhex> ")
	return s
}
func (s *ptySession) send(t *testing.T, text string) {
	t.Helper()
	if _, err := io.WriteString(s.tty, text); err != nil {
		t.Fatal(err)
	}
}
func (s *ptySession) until(t *testing.T, needle string) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		if i := bytes.Index(s.pending, []byte(needle)); i >= 0 {
			end := i + len(needle)
			result := string(s.pending[:end])
			s.pending = s.pending[end:]
			return result
		}
		select {
		case chunk, ok := <-s.output:
			if !ok {
				t.Fatalf("terminal closed waiting for %q: %q", needle, s.pending)
			}
			s.pending = append(s.pending, chunk...)
		case <-timeout:
			t.Fatalf("timed out waiting for %q: %q", needle, s.pending)
		}
	}
}

func TestInteractiveReplAutoIndentTabDedentAndInput(t *testing.T) {
	s := startPTY(t)
	s.send(t, "word choose() -> Int\r")
	s.until(t, "....>     ")
	s.send(t, "if true\r")
	s.until(t, "....>         ")
	s.send(t, "1\r")
	s.until(t, "....>         ")
	s.send(t, strings.Repeat("\x7f", 4)+"else\r")
	s.until(t, "....>         ")
	s.send(t, "2\r")
	s.until(t, "....>         ")
	s.send(t, "\r")
	s.until(t, "zhex> ")
	s.send(t, "choose()\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "1\r\n") {
		t.Fatalf("result: %q", got)
	}
	s.send(t, "flow 5\r")
	s.until(t, "....>     ")
	s.send(t, strings.Repeat("\x7f", 4)+"\tint.to-string\r")
	s.until(t, "....>     ")
	s.send(t, "print\r")
	s.until(t, "....>     ")
	s.send(t, "\r")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "5\r\n") {
		t.Fatalf("flow: %q", got)
	}
	s.send(t, "let name = read-line(\"Name: \") try\r")
	s.until(t, "Name: ")
	s.send(t, "Ada\r")
	s.until(t, "zhex> ")
	s.send(t, "print(name)\r")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "Ada\r\n") {
		t.Fatalf("input: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplBracketedPasteAndCancellation(t *testing.T) {
	s := startPTY(t)
	s.send(t, "\x1b[200~word pasted() -> Int\n    7\n\n\x1b[201~")
	s.until(t, "zhex> ")
	s.send(t, "pasted()\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "7\r\n") {
		t.Fatalf("paste: %q", got)
	}
	s.send(t, "word cancelled() -> Int\r")
	s.until(t, "....>     ")
	s.send(t, "\x03")
	s.until(t, "zhex> ")
	s.send(t, "1 + 1\r")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "2\r\n") {
		t.Fatalf("cancel: %q", got)
	}
	s.send(t, "word discarded() -> Int\r")
	s.until(t, "....>     ")
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatalf("Ctrl-D: %v", err)
	}
}

func TestInteractiveReplPreservesQueuedProgramInput(t *testing.T) {
	s := startPTY(t)
	s.send(t, "let who = read-line(\"Name: \") try\rAda\rprint(who)\r")
	first := s.until(t, "zhex> ")
	second := s.until(t, "zhex> ")
	if !strings.Contains(second, "Ada\r\n") || strings.Contains(first+second, "unknown") {
		t.Fatalf("queued input: %q", first+second)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplPreservesPastedTabsAndContinuationIndent(t *testing.T) {
	s := startPTY(t)
	s.send(t, "word f() -> Int\r")
	s.until(t, "....>     ")
	s.send(t, "\x1b[200~    7\n\n\x1b[201~")
	s.until(t, "zhex> ")
	s.send(t, "f()\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "7\r\n") {
		t.Fatalf("pasted continuation: %q", got)
	}
	s.send(t, "\x1b[200~word bad() -> Int\n\t1\n\n\x1b[201~")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "tab") {
		t.Fatalf("pasted tab should remain a lexer error: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplLongSourceIsNotTruncated(t *testing.T) {
	s := startPTY(t)
	s.send(t, "let text = \""+strings.Repeat("a", 4200)+"\"\r")
	s.until(t, "zhex> ")
	s.send(t, "print(text.length(text))\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "4200\r\n") {
		t.Fatalf("long input was changed: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplRestoresTerminalOnExit(t *testing.T) {
	s := startPTY(t)
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	state, err := unix.IoctlGetTermios(int(s.tty.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if state.Lflag&(unix.ICANON|unix.ECHO) != unix.ICANON|unix.ECHO {
		t.Fatalf("terminal mode not restored: %#x", state.Lflag)
	}
}

func TestInteractiveReplLongPastedSourceIsNotTruncated(t *testing.T) {
	s := startPTY(t)
	s.send(t, "\x1b[200~let text = \""+strings.Repeat("a", 4200)+"\"\n\x1b[201~")
	s.until(t, "zhex> ")
	s.send(t, "print(text.length(text))\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "4200\r\n") {
		t.Fatalf("long paste was changed: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveProgramInputPreservesTab(t *testing.T) {
	s := startPTY(t)
	s.send(t, "let line = read-line(\"input> \") try\r")
	s.until(t, "input> \x1b[?2004h")
	s.send(t, "a\tb\r")
	s.until(t, "zhex> ")
	s.send(t, "print(text.length(line))\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "3\r\n") {
		t.Fatalf("program input tab changed: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplDoesNotTreatPastedNonASCIIWhitespaceAsBlank(t *testing.T) {
	s := startPTY(t)
	s.send(t, "\x1b[200~word invalid() -> Int\n\u00a0\n    1\n\n\x1b[201~")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "unexpected") {
		t.Fatalf("pasted non-ASCII whitespace should remain a lexer error: %q", got)
	}
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
