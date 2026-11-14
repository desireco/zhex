package main

import (
	"strings"
	"testing"
)

func TestInteractiveReplBlockValues(t *testing.T) {
	s := startPTY(t)
	s.send(t, "let clean = flow \"  ADA  \"\r")
	s.until(t, "....>     ")
	s.send(t, "text.trim\r")
	s.until(t, "....>     ")
	s.send(t, "text.lower\r")
	s.until(t, "....>     ")
	s.send(t, "\r")
	s.until(t, "zhex> ")
	s.send(t, "clean\r")
	got := s.until(t, "zhex> ")
	if !strings.Contains(got, "ada\r\n") {
		t.Fatalf("flow result: %q", got)
	}
	s.send(t, "let value = match Option.Some(3)\r")
	s.until(t, "....>     ")
	s.send(t, "case Option.Some(n)\r")
	s.until(t, "....>         ")
	s.send(t, "n\r")
	s.until(t, "....>         ")
	s.send(t, strings.Repeat("\x7f", 4)+"case Option.None\r")
	s.until(t, "....>         ")
	s.send(t, "0\r")
	s.until(t, "....>         ")
	s.send(t, "\r")
	s.until(t, "zhex> ")
	s.send(t, "print(value)\r")
	got = s.until(t, "zhex> ")
	if !strings.Contains(got, "3\r\n") {
		t.Fatalf("match result: %q", got)
	}
	s.send(t, "bye()\r")
	if err := s.cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
