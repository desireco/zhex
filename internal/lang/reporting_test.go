package lang

import (
	"bytes"
	"io"
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

func TestHostReporterReceivesRedactedEnvelopeAfterLanguageReporter(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    print(\"language\")\nerror.set-reporter(report)\ntext.to-int(\"private input\") try\n"
	p, err := Compile("private/test.zhex", source)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	var events []ErrorReport
	err = p.RunWithOptions(strings.NewReader(""), &out, RunOptions{Reporter: func(event ErrorReport) error {
		if out.String() != "language\n" {
			t.Errorf("host called before language: %q", out.String())
		}
		events = append(events, event)
		return nil
	}})
	if err == nil || len(events) != 1 {
		t.Fatalf("error %v, reports %v", err, events)
	}
	e := events[0]
	if e.SchemaVersion != 1 || e.Outcome != "unhandled" || e.Error.Code != "text-to-int" || e.Error.Operation != "text.to-int" || e.OccurrenceID == "" || e.Timestamp.IsZero() {
		t.Fatalf("bad envelope: %+v", e)
	}
	if e.Error.Message != "" || e.Error.Origin.File != "" || len(e.Error.Details) != 0 {
		t.Fatalf("sensitive data leaked: %+v", e)
	}
}

func TestHostReporterFailureIsIsolated(t *testing.T) {
	p, err := Compile("test.zhex", "text.to-int(\"bad\") try\n")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = p.RunWithOptions(strings.NewReader(""), &out, RunOptions{Reporter: func(ErrorReport) error { panic("reporter failure") }})
	if err == nil || !strings.Contains(err.Error(), `"bad" is not a signed`) {
		t.Fatalf("original failure changed: %v", err)
	}
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("write failed") }

func TestHostReporterRunsAfterPanickingLanguageReporter(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    print(error.code(problem))\nerror.set-reporter(report)\ntext.to-int(\"bad\") try\n"
	p, err := Compile("test.zhex", source)
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	err = p.RunWithOptions(strings.NewReader(""), panicWriter{}, RunOptions{Reporter: func(e ErrorReport) error { called++; return nil }})
	if err == nil || !strings.Contains(err.Error(), `"bad" is not a signed`) || called != 1 {
		t.Fatalf("error %v, host calls %d", err, called)
	}
}

func TestHostReporterCanOptIntoSensitiveDetails(t *testing.T) {
	p, err := Compile("dir/test.zhex", "let n = file.read-all(\"/nonexistent-zhex-file\") catch problem\n    error.report(problem)\n    \"\"\n")
	if err != nil {
		t.Fatal(err)
	}
	var event ErrorReport
	err = p.RunWithOptions(strings.NewReader(""), io.Discard, RunOptions{IncludeSensitive: true, Correlation: "request-42", Reporter: func(e ErrorReport) error { event = e; return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if event.Outcome != "handled" || event.Correlation != "request-42" || event.Error.Origin.File != "dir/test.zhex" || event.Error.Details["path"] != "/nonexistent-zhex-file" || event.Error.Details["reason"] != "not-found" || event.Boundary.Line != 2 {
		t.Fatalf("bad envelope: %+v", event)
	}
}

func TestHostReporterIncludesRuntimeOperation(t *testing.T) {
	for _, tc := range []struct{ source, operation string }{{"1 / 0\n", "/"}, {"console.exit(256)\n", "console.exit"}, {"for line in file.lines(\"/nonexistent-zhex-file\")\n    print(line)\n", "file.lines"}} {
		p, err := Compile("test.zhex", tc.source)
		if err != nil {
			t.Fatal(err)
		}
		var event ErrorReport
		err = p.RunWithOptions(strings.NewReader(""), io.Discard, RunOptions{Reporter: func(e ErrorReport) error { event = e; return nil }})
		if err == nil || event.Error.Operation != tc.operation {
			t.Fatalf("error %v, event %+v", err, event)
		}
	}
}

func TestHostReportBoundsCauseSnapshots(t *testing.T) {
	source := "word nested(n: Int) -> Result<Int, Error>\n    if n == 0\n        return err(error.new(\"base\", \"base\"))\n    let value = nested(n - 1) catch problem\n        return err(error.wrap(\"wrap\", \"wrap\", problem))\n    ok(value)\nnested(40) try\n"
	p, err := Compile("test.zhex", source)
	if err != nil {
		t.Fatal(err)
	}
	var event ErrorReport
	err = p.RunWithOptions(strings.NewReader(""), io.Discard, RunOptions{Reporter: func(e ErrorReport) error { event = e; return nil }})
	if err == nil || !strings.Contains(err.Error(), "wrap") {
		t.Fatalf("original error = %v", err)
	}
	node, count := &event.Error, 1
	for node.Cause != nil {
		node = node.Cause
		count++
	}
	if count != 32 || !node.CauseTruncated {
		t.Fatalf("cause depth %d, truncated %v", count, node.CauseTruncated)
	}
}

func TestDeliberateExitDoesNotAutomaticallyReport(t *testing.T) {
	for _, tc := range []struct {
		source  string
		reports int
	}{
		{"console.exit(0)\n", 0},
		{"let n = text.to-int(\"bad\") catch problem\n    error.report(problem)\n    console.exit(23)\n", 1},
	} {
		p, err := Compile("test.zhex", tc.source)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		err = p.RunWithOptions(strings.NewReader(""), io.Discard, RunOptions{Reporter: func(ErrorReport) error { count++; return nil }})
		if _, ok := err.(*ExitError); !ok || count != tc.reports {
			t.Fatalf("exit %v, reports %d", err, count)
		}
	}
}

func TestReporterExitCannotReplaceOriginalFailure(t *testing.T) {
	source := "word report(problem: Error) -> Unit\n    console.exit(99)\nerror.set-reporter(report)\ntext.to-int(\"bad\") try\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), `"bad" is not a signed`) {
		t.Fatalf("original failure changed: %v", err)
	}
}
