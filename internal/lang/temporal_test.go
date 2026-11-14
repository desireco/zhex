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

func TestInstantAndZonedDSTOperations(t *testing.T) {
	source := `let zone = timezone.load("America/New_York") try
let start = zoned.new(date.new(2024, 3, 9), time.new(2, 30, 0), zone)
print(zoned.to-string(zoned.add-days(start, 1)))
let span = duration.parse("24h") try
print(zoned.to-string(zoned.add-duration(start, span)))
let overlap = zoned.new(date.new(2024, 11, 3), time.new(1, 30, 0), zone)
print(instant.to-string(zoned.instant(overlap)))
let utc = timezone.load("UTC") try
print(zoned.to-string(zoned.in-zone(overlap, utc)))
let origin = instant.parse("2024-01-01T12:00:00+02:00") try
print(instant.to-string(instant.add-duration(origin, duration.new(1))))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	want := "2024-03-10T03:30:00-04:00[America/New_York]\n2024-03-10T03:30:00-04:00[America/New_York]\n2024-11-03T05:30:00Z\n2024-11-03T05:30:00Z[UTC]\n2024-01-01T10:00:00.000000001Z\n"
	if out.String() != want {
		t.Fatalf("output %q", out.String())
	}
}

func TestInstantAndZoneInputFailures(t *testing.T) {
	for _, source := range []string{"let t = instant.parse(\"2024-01-01T12:00:00\") try\n", "let z = timezone.load(\"Local\") try\n", "let z = timezone.load(\"not/a/zone\") try\n"} {
		if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
}

func TestZonedCalendarAndElapsedArithmeticDiffer(t *testing.T) {
	source := `let zone = timezone.load("America/New_York") try
let day = zoned.new(date.new(2024,3,9), time.new(12,0,0), zone)
print(zoned.to-string(zoned.add-days(day, 1)))
print(zoned.to-string(zoned.add-duration(day, duration.parse("24h") try)))
let old = zoned.new(date.new(1890,1,1), time.new(12,0,0), timezone.load("Europe/Paris") try)
print(zoned.to-string(old))
print(instant.to-string(zoned.instant(old)))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	want := "2024-03-10T12:00:00-04:00[America/New_York]\n2024-03-10T13:00:00-04:00[America/New_York]\n1890-01-01T12:00:00+00:09:21[Europe/Paris]\n1890-01-01T11:50:39Z\n"
	if out.String() != want {
		t.Fatalf("output %q", out.String())
	}
}

func TestInstantStrictRFC3339AndYearBoundaries(t *testing.T) {
	for _, input := range []string{"0000-01-01T00:00:00Z", "2024-01-01T1:00:00Z", "2024-01-01T12:00:00,1Z", "2024-01-01T12:00:00+24:00", "2024-01-01T12:00:00+01:60", "0001-01-01T00:00:00+01:00", "9999-12-31T23:00:00-02:00"} {
		source := "let x = instant.parse(\"" + input + "\") try\n"
		if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	source := "let i = instant.parse(\"9999-12-31T23:59:59.999999999Z\") try\nlet x = instant.add-duration(i, duration.new(1))\n"
	if err := Run("test.zhex", source, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Fatal("accepted instant overflow")
	}
}

func TestZonedAddZeroDaysPreservesLaterOverlapInstant(t *testing.T) {
	source := `let zone = timezone.load("America/New_York") try
let first = zoned.new(date.new(2024,11,3), time.new(1,30,0), zone)
let later = zoned.add-duration(first, duration.parse("1h") try)
print(instant.to-string(zoned.instant(later)))
print(instant.to-string(zoned.instant(zoned.add-days(later, 0))))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2024-11-03T06:30:00Z\n2024-11-03T06:30:00Z\n" {
		t.Fatal(out.String())
	}
}

func TestTemporalInvalidComponentDetailsAndOrigin(t *testing.T) {
	p, err := Compile("test.zhex", "let value = date.new(2023, 2, 29)\n")
	if err != nil {
		t.Fatal(err)
	}
	var report ErrorReport
	err = p.RunWithOptions(strings.NewReader(""), &bytes.Buffer{}, RunOptions{IncludeSensitive: true, Reporter: func(e ErrorReport) error { report = e; return nil }})
	if err == nil || report.Error.Operation != "date.new" || report.Error.Origin.Line != 1 || report.Error.Details["year"] != "2023" || report.Error.Details["month"] != "2" || report.Error.Details["day"] != "29" {
		t.Fatalf("error %v, report %+v", err, report)
	}
}

func TestTimezoneNamePreservesLoadedIdentity(t *testing.T) {
	source := `let utc = timezone.load("UTC") try
print(timezone.name(utc))
let zone = timezone.load("America/New_York") try
print(timezone.name(zone))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "UTC\nAmerica/New_York\n" {
		t.Fatalf("zone names %q", out.String())
	}
	if _, err := Compile("test.zhex", "print(timezone.name(\"UTC\"))\n"); err == nil {
		t.Fatal("accepted String in place of TimeZone")
	}
}
