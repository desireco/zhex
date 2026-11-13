package lang

import (
	"strings"
	"testing"
)

func TestConsoleArgs(t *testing.T) {
	run := func(source string, args ...string) (string, error) {
		var out strings.Builder
		err := RunWithArgs("args.zhex", source, strings.NewReader(""), &out, nil, args)
		return out.String(), err
	}
	out, err := run("print(console.arg(0) try)\nprint(console.arg(1) try)\n", "alpha", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if want := "alpha\nbeta\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	out, err = run("print(console.args-count())\n", "alpha", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if want := "2\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	for _, index := range []string{"2", "-1"} {
		_, err := run("let a = console.arg("+index+") try\nprint(a)\n", "alpha", "beta")
		if err == nil {
			t.Fatalf("index %s: expected argument-missing", index)
		}
		code, message := codeOf(t, err)
		if code != "argument-missing" {
			t.Fatalf("wanted argument-missing, got %s: %q", code, message)
		}
		if !strings.Contains(message, "index "+index) || !strings.Contains(message, "received 2") {
			t.Fatalf("message must name the index and the count: %q", message)
		}
		if strings.Contains(err.Error(), "hint:") {
			t.Fatalf("argument-missing carries no hint: %q", err.Error())
		}
	}

	// A bare console.arg Result keeps earning the try hint.
	var out2 strings.Builder
	err = RunWithArgs("bad.zhex", "let a = console.arg(0)\nprint(a)\n", strings.NewReader(""), &out2, nil, []string{"alpha"})
	if err == nil || !strings.Contains(err.Error(), "print does not support type Result<String, InputError>") || !strings.Contains(err.Error(), "add postfix try") {
		t.Fatalf("wrong diagnostic: %v", err)
	}
}
