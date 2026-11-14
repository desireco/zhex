package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestDateTimeDurationValues(t *testing.T) {
	source := `let date: Date = date.new(2024, 2, 29)
print(date.to-string(date.add-days(date, 1)))
let clock: Time = time.parse("12:34:56.123456789") try
print(time.to-string(clock))
let d: Duration = duration.parse("1h30m") try
print(duration.to-string(d))
print(duration.to-string(duration.new(-1)))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2024-03-01\n12:34:56.123456789\n1h30m0s\n-1ns\n" {
		t.Fatal(out.String())
	}
}

func TestTemporalInvalidAndRecoverableInputs(t *testing.T) {
	for _, source := range []string{"let d = date.new(2023,2,29)\n", "let t = time.new(24,0,0)\n", "let d = date.add-days(date.new(9999,12,31), 1)\n"} {
		if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
	source := `let value = date.parse("2023-02-29") catch problem
    print(error.code(problem))
    date.new(2020, 1, 1)
print(date.to-string(value))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "date-parse\n2020-01-01\n" {
		t.Fatal(out.String())
	}
}

func TestTemporalParsingPrecisionBoundsAndEffects(t *testing.T) {
	source := `word parse(raw: String) -> Date
    date.parse(raw) try
let earliest = parse("0001-01-01") try
print(date.to-string(earliest))
let latest = date.parse("9999-12-31") try
print(date.to-string(latest))
let clock = time.parse("23:59:59.000000001") try
print(time.to-string(clock))
`
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0001-01-01\n9999-12-31\n23:59:59.000000001\n" || !strings.Contains(trace.String(), "0001-01-01") {
		t.Fatalf("output %q trace %q", out.String(), trace.String())
	}
	for _, bad := range []string{"00:00:00.1234567890", "12:00:60", "10:20:30,1"} {
		source := "let t = time.parse(\"" + bad + "\") try\n"
		if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("accepted time %q", bad)
		}
	}
	for _, bad := range []string{"let d = date.parse(\"2024-01-01\")\n", "let d = json.encode(date.new(2024,1,1)) try\n", "type Date = record\n    n: Int\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
