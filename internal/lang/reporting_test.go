package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestLanguageReporterHandlesExplicitAndUnhandledFailures(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    print(error.code(problem))\nerror.set-reporter(report)\nlet n = text.to-int(\"bad\") catch problem\n    error.report(problem)\n    7\nprint(n)\ntext.to-int(\"oops\") try\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), `"oops" is not a signed`) {
		t.Fatalf("original error = %v", err)
	}
	if out.String() != "text-to-int\n7\ntext-to-int\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestLanguageReporterFailureCannotReplaceOriginal(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    print(1 / 0)\nerror.set-reporter(report)\ntext.to-int(\"bad\") try\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), `"bad" is not a signed`) {
		t.Fatalf("original error = %v", err)
	}
}

func TestReporterDeduplicatesAndSuppressesReentry(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    print(\"report\")\n    error.report(problem)\nword fail() -> Result<Int, Error>\n    let n = text.to-int(\"bad\") catch problem\n        error.report(problem)\n        return err(error.wrap(\"converted\", \"converted\", problem))\n    ok(n)\nerror.set-reporter(report)\nfail() try\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "converted") {
		t.Fatalf("error = %v", err)
	}
	if out.String() != "report\n" {
		t.Fatalf("reporter invoked more than once: %q", out.String())
	}
}

func TestReporterMustBeNamedTopLevelWordWithErrorSignature(t *testing.T) {
	for _, source := range []string{
		"word report(problem: String) -> Unit\n    print(problem)\nerror.set-reporter(report)\n",
		"word report(problem: Error) -> Unit\n    print(error.code(problem))\nif true\n    error.set-reporter(report)\n",
		"error.set-reporter(\"report\")\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted bad reporter: %q", source)
		}
	}
}

func TestReplKeepsReporterAcrossEntries(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("word report(problem: Error) -> Unit\n    print(error.code(problem))\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("error.set-reporter(report)\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("text.to-int(\"bad\") try\n"); err == nil {
		t.Fatal("expected unhandled failure")
	}
	if out.String() != "text-to-int\n" {
		t.Fatalf("output = %q", out.String())
	}
}
