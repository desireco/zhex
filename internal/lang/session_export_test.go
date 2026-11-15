package lang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Session.Export reconstructs engine-side session state as Zhex source; the
// #65 prototype's repl.save writes it and repl.load Enters it back.

func TestSessionExportHeaderUsesAndWordAndTypeSources(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math.zhex"), []byte("word gcd(a: Int, b: Int) -> Int\n    if b == 0\n        a\n    else\n        gcd(b, a % b)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, func(name string) (ModuleSource, bool) {
		source, err := os.ReadFile(filepath.Join(dir, name+".zhex"))
		if err != nil {
			return ModuleSource{}, false
		}
		return ModuleSource{Path: filepath.Join(dir, name+".zhex"), Source: string(source)}, true
	})
	for _, entry := range []string{
		"type Point = record\n    x: Int\n    y: Int\n",
		"use math\n",
		"word double(n: Int) -> Int\n    n * 2\n",
		"word double(n: Int) -> Int\n    n + n\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	want := "# zhex session v1\n" +
		"use math\n" +
		"type Point = record\n    x: Int\n    y: Int\n" +
		"word double(n: Int) -> Int\n    n + n\n"
	if out != want {
		t.Fatalf("export %q, want %q", out, want)
	}
}

func TestSessionExportBindingLiterals(t *testing.T) {
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	for _, entry := range []string{
		"type Point = record\n    x: Int\n    y: Int = 0\n",
		"type Status = choice\n    Pending\n    Failed(reason: String)\n",
		"var count = 4\n",
		"let flag = true\n",
		"let name = \"Ada \\\"Ace\\\"\"\n",
		"let nums: List<Int> = [1, -2]\n",
		"let empty: List<String> = []\n",
		"let scores: Map<String, Int> = {\"a\": 1, \"b\": 2}\n",
		"let bare: Map<String, Int> = {}\n",
		"let spot = Point(x: 2, y: 3)\n",
		"let defaulted = Point(x: 9)\n",
		"let state = Status.Failed(\"late\")\n",
		"let waiting = Status.Pending\n",
		"let some: Option<Int> = Option.Some(7)\n",
		"let none: Option<Int> = Option.None\n",
		"let tree = Json.Object({\"k\": Json.Number(\"1.5\"), \"z\": Json.List([Json.Null, Json.Bool(true)])})\n",
		"var grid = array.new([2, 2], 0) try\n",
		"grid[0, 1] = 5\n",
		"let day = date.parse(\"2026-11-14\") try\n",
		"let clock = time.parse(\"12:34:56.5\") try\n",
		"let span = duration.new(1500)\n",
		"let moment = instant.parse(\"2026-11-14T10:00:00.25Z\") try\n",
		"let zone = timezone.load(\"UTC\") try\n",
		"let stamp = zoned.new(day, clock, zone)\n",
		"var b = buffer.new()\n",
		"buffer.add-line(b, \"one\")\n",
		"buffer.add-line(b, \"\")\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatalf("%q: %v", entry, err)
		}
	}
	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	// Bindings export sorted by name; buffers and arrays carry follow-up
	// statements next to their binding.
	want := "# zhex session v1\n" +
		"type Point = record\n    x: Int\n    y: Int = 0\n" +
		"type Status = choice\n    Pending\n    Failed(reason: String)\n" +
		"var b: Buffer = buffer.new()\n" +
		"buffer.add-line(b, \"one\")\n" +
		"buffer.add-line(b, \"\")\n" +
		"let bare: Map<String, Int> = {}\n" +
		"let clock: Time = time.parse(\"12:34:56.5\") try\n" +
		"var count: Int = 4\n" +
		"let day: Date = date.parse(\"2026-11-14\") try\n" +
		"let defaulted: Point = Point(x: 9, y: 0)\n" +
		"let empty: List<String> = []\n" +
		"let flag: Bool = true\n" +
		"var grid: Array<Int> = array.new([2, 2], 0) try\n" +
		"grid[0, 1] = 5\n" +
		"let moment: Instant = instant.parse(\"2026-11-14T10:00:00.25Z\") try\n" +
		"let name: String = \"Ada \\\"Ace\\\"\"\n" +
		"let none: Option<Int> = Option.None\n" +
		"let nums: List<Int> = [1, -2]\n" +
		"let scores: Map<String, Int> = {\"a\": 1, \"b\": 2}\n" +
		"let some: Option<Int> = Option.Some(7)\n" +
		"let span: Duration = duration.new(1500)\n" +
		"let spot: Point = Point(x: 2, y: 3)\n" +
		"let stamp: ZonedDateTime = zoned.new(date.parse(\"2026-11-14\") try, time.parse(\"12:34:56.5\") try, timezone.load(\"UTC\") try)\n" +
		"let state: Status = Status.Failed(\"late\")\n" +
		"let tree: Json = Json.Object({\"k\": Json.Number(\"1.5\"), \"z\": Json.List([Json.Null, Json.Bool(true)])})\n" +
		"let waiting: Status = Status.Pending\n" +
		"let zone: TimeZone = timezone.load(\"UTC\") try\n"
	if out != want {
		t.Fatalf("export:\n%q\nwant:\n%q", out, want)
	}
}

func TestSessionExportSkipsUnserializableBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(path, []byte("line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	for _, entry := range []string{
		"word double(n: Int) -> Int\n    n * 2\n",
		"let unit-value = print(\"x\")\n",
		"let boom = error.new(\"app\", \"bad\")\n",
		"let doubled = double _\n",
		"let data = file.open(" + zstring(path) + ") try\n",
		"let keep = 5\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatalf("%q: %v", entry, err)
		}
	}
	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	for _, skipped := range []string{"unit-value", "boom", "doubled", "data"} {
		if !strings.Contains(out, "# note: skipped "+skipped+":") {
			t.Fatalf("export missing note for %s:\n%s", skipped, out)
		}
	}
	if !strings.Contains(out, "let keep: Int = 5\n") {
		t.Fatalf("export lost serializable binding:\n%s", out)
	}
}

func TestSessionExportReporter(t *testing.T) {
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	for _, entry := range []string{
		"word report(problem: Error) -> Unit\n    print(\"reported\")\n",
		"error.set-reporter(report)\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out, "error.set-reporter(report)\n") {
		t.Fatalf("export missing reporter emission:\n%s", out)
	}
}

func TestSessionExportRoundTripRestoresState(t *testing.T) {
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	for _, entry := range []string{
		"type Point = record\n    x: Int\n    y: Int\n",
		"word norm(p: Point) -> Int\n    p.x + p.y\n",
		"var count = 1\n",
		"let spot = Point(x: 20, y: 22)\n",
		"let b = buffer.new()\n",
		"buffer.add-line(b, \"alpha\")\n",
		"buffer.add-line(b, \"beta\")\n",
		"var log = buffer.new()\n",
		"buffer.add-line(log, \"old\")\n",
		"var cells = array.new([1, 2], 7) try\n",
		"cells[0, 1] = 9\n",
		"let tree = Json.List([Json.Number(\"2\"), Json.Text(\"x\")])\n",
		"let day = date.parse(\"2026-01-02\") try\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatalf("%q: %v", entry, err)
		}
	}
	source, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	fresh := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := fresh.Enter(source); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if _, err := fresh.Enter("count = count + 1\nnorm(spot)\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := fresh.Enter("count"); err != nil || echo != "2" {
		t.Fatalf("var did not round-trip: %q, %v", echo, err)
	}
	if echo, err := fresh.Enter("norm(spot)"); err != nil || echo != "42" {
		t.Fatalf("word+record did not round-trip: %q, %v", echo, err)
	}
	if echo, err := fresh.Enter("buffer.to-text(b)"); err != nil || echo != "alpha\nbeta" {
		t.Fatalf("buffer did not round-trip: %q, %v", echo, err)
	}
	// A var Buffer or Array keeps its var-ness: rebinding after load succeeds.
	if _, err := fresh.Enter("log = buffer.new()\nbuffer.add-line(log, \"next\")\n"); err != nil {
		t.Fatalf("var buffer lost mutability: %v", err)
	}
	if echo, err := fresh.Enter("buffer.to-text(log)"); err != nil || echo != "next" {
		t.Fatalf("rebound buffer wrong: %q, %v", echo, err)
	}
	if _, err := fresh.Enter("cells = array.new([1, 2], 3) try\n"); err != nil {
		t.Fatalf("var array lost mutability: %v", err)
	}
	if echo, err := fresh.Enter("cells[0, 1]"); err != nil || echo != "3" {
		t.Fatalf("rebound array wrong: %q, %v", echo, err)
	}
	if echo, err := fresh.Enter("json.encode(tree) try"); err != nil || echo != "[2,\"x\"]" {
		t.Fatalf("json tree did not round-trip: %q, %v", echo, err)
	}
	if echo, err := fresh.Enter("date.to-string(day)"); err != nil || echo != "2026-01-02" {
		t.Fatalf("date did not round-trip: %q, %v", echo, err)
	}
}

func TestSessionExportEmpty(t *testing.T) {
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	out, err := s.Export()
	if err != nil {
		t.Fatal(err)
	}
	if out != "# zhex session v1\n" {
		t.Fatalf("empty export %q", out)
	}
}
