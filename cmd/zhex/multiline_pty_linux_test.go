package main

import (
	"strings"
	"testing"
)

func TestInteractiveReplMultilineDelimiterBlankAndPaste(t *testing.T) {
	s := startPTY(t)
	s.send(t, "print(\r")
	s.until(t, "....>     ")
	s.send(t, "\r")
	s.until(t, "....>     ")
	s.send(t, "1 + 1\r")
	s.until(t, "....>     ")
	s.send(t, ")\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "2\r\n") {
		t.Fatalf("multiline result: %q", got)
	}
	s.send(t, "\x1b[200~print(\n  3 + 4\n)\n\x1b[201~")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "7\r\n") {
		t.Fatalf("multiline paste: %q", got)
	}
	s.send(t, "bye()\r")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestInteractiveReplOpenDelimiterCanCancel(t *testing.T) {
	s := startPTY(t)
	s.send(t, "print(\r")
	s.until(t, "....>     ")
	s.send(t, "\x03")
	s.until(t, "zhex> ")
	s.send(t, "print(2)\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "2\r\n") {
		t.Fatalf("cancel result: %q", got)
	}
	s.send(t, "print(\r")
	s.until(t, "....>     ")
	s.send(t, "\x04")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
