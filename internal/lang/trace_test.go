package lang

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func runTraced(t *testing.T, name, input string) (string, string) {
	t.Helper()
	path := "../../examples/" + name + ".zhex"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out, trace bytes.Buffer
	if err := RunWithTrace(path, string(src), strings.NewReader(input), &out, &trace); err != nil {
		t.Fatal(err)
	}
	return out.String(), trace.String()
}

func TestTraceComposition(t *testing.T) {
	out, trace := runTraced(t, "composition", "")
	if out != "32\n" {
		t.Fatalf("output %q", out)
	}
	want := strings.Join([]string{
		`{"event":"flow-step","word":"square","value":"4","file":"../../examples/composition.zhex","line":9}`,
		`{"event":"word-call","word":"square","file":"../../examples/composition.zhex","line":9,"args":["4"]}`,
		`{"event":"binding","name":"n","value":"4","file":"../../examples/composition.zhex","line":1}`,
		`{"event":"binding","name":"result","value":"16","file":"../../examples/composition.zhex","line":2}`,
		`{"event":"word-return","word":"square","value":"16","file":"../../examples/composition.zhex","line":1}`,
		`{"event":"flow-step","word":"double","value":"16","file":"../../examples/composition.zhex","line":10}`,
		`{"event":"word-call","word":"double","file":"../../examples/composition.zhex","line":10,"args":["16"]}`,
		`{"event":"binding","name":"n","value":"16","file":"../../examples/composition.zhex","line":5}`,
		`{"event":"word-return","word":"double","value":"32","file":"../../examples/composition.zhex","line":5}`,
		`{"event":"flow-step","word":"print","value":"32","file":"../../examples/composition.zhex","line":11}`,
	}, "\n") + "\n"
	if trace != want {
		t.Fatalf("trace:\n%s\nwant:\n%s", trace, want)
	}
}

func TestTraceGCD(t *testing.T) {
	out, trace := runTraced(t, "gcd", "48\n18\n")
	if out != "First number: Second number: 6\n" {
		t.Fatalf("output %q", out)
	}
	want := strings.Join([]string{
		`{"event":"binding","name":"a-line","value":"48","file":"../../examples/gcd.zhex","line":10}`,
		`{"event":"binding","name":"a","value":"48","file":"../../examples/gcd.zhex","line":11}`,
		`{"event":"binding","name":"b-line","value":"18","file":"../../examples/gcd.zhex","line":12}`,
		`{"event":"binding","name":"b","value":"18","file":"../../examples/gcd.zhex","line":13}`,
		`{"event":"word-call","word":"gcd","file":"../../examples/gcd.zhex","line":18,"args":["48","18"]}`,
		`{"event":"binding","name":"a","value":"48","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"binding","name":"b","value":"18","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-call","word":"gcd","file":"../../examples/gcd.zhex","line":8,"args":["18","12"]}`,
		`{"event":"binding","name":"a","value":"18","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"binding","name":"b","value":"12","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-call","word":"gcd","file":"../../examples/gcd.zhex","line":8,"args":["12","6"]}`,
		`{"event":"binding","name":"a","value":"12","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"binding","name":"b","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-call","word":"gcd","file":"../../examples/gcd.zhex","line":8,"args":["6","0"]}`,
		`{"event":"binding","name":"a","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"binding","name":"b","value":"0","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-return","word":"gcd","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-return","word":"gcd","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-return","word":"gcd","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"word-return","word":"gcd","value":"6","file":"../../examples/gcd.zhex","line":4}`,
		`{"event":"flow-step","word":"print","value":"6","file":"../../examples/gcd.zhex","line":19}`,
	}, "\n") + "\n"
	if trace != want {
		t.Fatalf("trace:\n%s\nwant:\n%s", trace, want)
	}
}

func TestTraceForLoopBinding(t *testing.T) {
	var out, trace bytes.Buffer
	src := "for i in range(0, 2)\n    print(i)\n"
	if err := RunWithTrace("loop.zhex", src, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n1\n" {
		t.Fatalf("output %q", out.String())
	}
	want := strings.Join([]string{
		`{"event":"binding","name":"i","value":"0","file":"loop.zhex","line":1}`,
		`{"event":"binding","name":"i","value":"1","file":"loop.zhex","line":1}`,
	}, "\n") + "\n"
	if trace.String() != want {
		t.Fatalf("trace:\n%s\nwant:\n%s", trace.String(), want)
	}
}

func TestTraceAssignmentBinding(t *testing.T) {
	var out, trace bytes.Buffer
	src := "var x = 1\nx = x + 1\nprint(x)\n"
	if err := RunWithTrace("assign.zhex", src, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatalf("output %q", out.String())
	}
	want := strings.Join([]string{
		`{"event":"binding","name":"x","value":"1","file":"assign.zhex","line":1}`,
		`{"event":"binding","name":"x","value":"2","file":"assign.zhex","line":2}`,
	}, "\n") + "\n"
	if trace.String() != want {
		t.Fatalf("trace:\n%s\nwant:\n%s", trace.String(), want)
	}
}

func TestTraceResultBinding(t *testing.T) {
	for _, tc := range []struct{ name, binding, source, input, value, output string }{
		{"read-line ok", "line", "let line = read-line(\"x: \")\nprint(\"done\")\n", "abc\n", "ok(\"abc\")", "x: done\n"},
		{"read-line err", "line", "let line = read-line(\"x: \")\nprint(\"done\")\n", "", "err(\"console input failed: EOF\")", "x: done\n"},
		{"to-int ok", "n", "let n = text.to-int(\"48\")\nprint(\"done\")\n", "", "ok(\"48\")", "done\n"},
		{"to-int err", "n", "let n = text.to-int(\"abc\")\nprint(\"done\")\n", "", "err(\"\\\"abc\\\" is not a signed 64-bit decimal integer\")", "done\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, trace bytes.Buffer
			if err := RunWithTrace("result.zhex", tc.source, strings.NewReader(tc.input), &out, &trace); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.output {
				t.Fatalf("output %q", out.String())
			}
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			want := `{"event":"binding","name":"` + tc.binding + `","value":` + string(encoded) + `,"file":"result.zhex","line":1}` + "\n"
			if trace.String() != want {
				t.Fatalf("trace:\n%s\nwant:\n%s", trace.String(), want)
			}
		})
	}
}

func TestTraceUnitValue(t *testing.T) {
	var out, trace bytes.Buffer
	src := "word log() -> Unit\n    print(\"hi\")\n\nlog()\n"
	if err := RunWithTrace("unit.zhex", src, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hi\n" {
		t.Fatalf("output %q", out.String())
	}
	want := strings.Join([]string{
		`{"event":"word-call","word":"log","file":"unit.zhex","line":4,"args":[]}`,
		`{"event":"word-return","word":"log","value":"","file":"unit.zhex","line":1}`,
	}, "\n") + "\n"
	if trace.String() != want {
		t.Fatalf("trace:\n%s\nwant:\n%s", trace.String(), want)
	}
}
