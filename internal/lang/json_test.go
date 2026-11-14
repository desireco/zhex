package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONDecodeRecordDefaultsAndEncode(t *testing.T) {
	source := "type Settings = record\n    retries: Int\n    verbose: Bool\nlet settings = json.decode(\"{\\\"retries\\\":5}\", Settings(3, false)) try\nprint(settings.retries)\nprint(settings.verbose)\nprint(json.encode(settings) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\nfalse\n{\"retries\":5,\"verbose\":false}\n" {
		t.Fatal(out.String())
	}
}

func TestJSONRejectsInvalidFieldsAndExactIntegers(t *testing.T) {
	for i, raw := range []string{"{\\\"retries\\\":1.5}", "{\\\"retries\\\":9223372036854775808}", "{\\\"retries\\\":1,\\\"retries\\\":2}", "{\\\"unknown\\\":1}", "{\\\"retries\\\":null}"} {
		source := "type Settings = record\n    retries: Int\nlet result = json.decode(\"" + raw + "\", Settings(3)) catch problem\n    print(error.code(problem))\n    Settings(0)\n"
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != []string{"json-range\n", "json-range\n", "json-field\n", "json-field\n", "json-type\n"}[i] {
			t.Fatalf("%s output %q", raw, out.String())
		}
	}
}

func TestJSONDefaultsAreClonedAndAliasesPreserved(t *testing.T) {
	source := "type Settings = record\n    left: List<Int>\n    right: List<Int>\nlet shared = [1]\nlet defaults = Settings(shared, shared)\nlet first = json.decode(\"{}\", defaults) try\nfirst.left[0] = 9\nlet second = json.decode(\"{}\", defaults) try\nprint(first.right[0] try)\nprint(defaults.left[0] try)\nprint(second.left[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestJSONOptionNullAndNestedMissingRecord(t *testing.T) {
	source := "type Inner = record\n    count: Int\n    enabled: Bool\ntype Settings = record\n    nested: Inner\n    timeout: Option<Int>\nlet defaults = Settings(Inner(3, true), Option.Some(8))\nlet result = json.decode(\"{\\\"nested\\\":{\\\"count\\\":5},\\\"timeout\\\":null}\", defaults) try\nprint(result.nested.count)\nprint(result.nested.enabled)\nmatch result.timeout\n    case Option.None\n        print(\"none\")\n    case Option.Some(value)\n        print(value)\nprint(json.encode(result) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\ntrue\nnone\n{\"nested\":{\"count\":5,\"enabled\":true},\"timeout\":null}\n" {
		t.Fatal(out.String())
	}
}

func TestJSONRejectsUnsupportedSchemaBeforeExecution(t *testing.T) {
	for _, source := range []string{"let data = buffer.new()\njson.encode(data) try\n", "let m = {1: \"x\"}\njson.encode(m) try\n", "let grid = array.new([2, 2], 0) try\njson.encode(grid) try\n", "type Status = choice\n    On\njson.encode(Status.On) try\n"} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted unsupported schema:\n%s", source)
		}
	}
}

func TestJSONReportsEscapedPointerAndReason(t *testing.T) {
	source := "let defaults: Map<String, Int> = {}\nlet result = json.decode(\"{\\\"a/b~c\\\":false}\", defaults) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"path\"))\n    defaults\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "json-type\n/a~1b~0c\n" {
		t.Fatal(out.String())
	}
	source = "type Settings = record\n    retries: Int\nlet result = json.decode(\"{\\\"other\\\":1}\", Settings(3)) catch problem\n    print(error.detail(problem, \"path\"))\n    print(error.detail(problem, \"reason\"))\n    Settings(0)\n"
	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "/other\nunknown\n" {
		t.Fatal(out.String())
	}
}

func TestJSONPresentCollectionsReplaceDefaults(t *testing.T) {
	source := "type Settings = record\n    left: List<Int>\n    right: List<Int>\nlet shared = [1, 2]\nlet defaults = Settings(shared, shared)\nlet value = json.decode(\"{\\\"left\\\":[7]}\", defaults) try\nvalue.left[0] = 9\nprint(list.length(value.left))\nprint(value.right[0] try)\nprint(defaults.left[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestJSONCollectionRecordsRequireAllFields(t *testing.T) {
	source := "type Person = record\n    name: String\n    age: Int\nlet defaults: List<Person> = []\nlet result = json.decode(\"[{\\\"name\\\":\\\"Ada\\\"}]\", defaults) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"path\"))\n    print(error.detail(problem, \"reason\"))\n    defaults\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "json-field\n/0/age\nmissing\n" {
		t.Fatal(out.String())
	}
}

func TestJSONIntegerBoundariesAndTokenForm(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
		ok   bool
	}{{"-9223372036854775808", -9223372036854775808, true}, {"9223372036854775807", 9223372036854775807, true}, {"9223372036854775808", 0, false}, {"1.0", 0, false}, {"1e0", 0, false}, {"-0", 0, true}} {
		n, ok := exactJSONInt(tc.raw)
		if ok != tc.ok || ok && n != tc.want {
			t.Errorf("%s => %d %v", tc.raw, n, ok)
		}
	}
}

func TestJSONDecodeUsesExpectedTypeForEmptyDefaults(t *testing.T) {
	source := "word load() -> List<Int>\n    json.decode(\"[1,2]\", []) try\nlet result = load() try\nprint(list.length(result))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
}

func TestJSONWorksThroughInferredFlow(t *testing.T) {
	source := "type Settings = record\n    retries: Int\nword parse(raw: String) -> Settings\n    flow raw\n        json.decode(Settings(3)) try\nlet settings = parse(\"{}\") try\nprint(settings.retries)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Fatal(out.String())
	}
}

func TestJSONRejectsAmbiguousNestedOption(t *testing.T) {
	source := "let value: Option<Option<Int>> = Option.Some(Option.None)\njson.encode(value) try\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("accepted non-round-trippable nested Option")
	}
	source = "let value: Option<List<Option<Int>>> = Option.Some([Option.None])\njson.encode(value) try\n"
	if _, err := Compile("test.zhex", source); err != nil {
		t.Fatalf("rejected representable nested option: %v", err)
	}
}

func TestJSONRejectsLoneSurrogateEscapes(t *testing.T) {
	for _, raw := range []string{`"\ud800"`, `"\udc00"`, `"\ud800x"`} {
		v := decodeJSON(position{file: "test.zhex", line: 1, column: 1}, raw, value{text: ""}, &jsonSchema{typ: stringType, kind: "string"})
		if v.result == nil || v.result.err == nil || v.result.err.(*diagnostic).code != CodeJSONSyntax {
			t.Errorf("accepted malformed surrogate %q", raw)
		}
	}
	v := decodeJSON(position{file: "test.zhex", line: 1, column: 1}, `"\ud83d\ude00"`, value{text: ""}, &jsonSchema{typ: stringType, kind: "string"})
	if v.result.err != nil || v.result.value.text != "😀" {
		t.Fatalf("valid surrogate pair: %+v", v.result)
	}
}
