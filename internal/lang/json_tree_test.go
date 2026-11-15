package lang

import (
	"bytes"
	"strings"
	"testing"
)

func runTree(t *testing.T, source string) string {
	t.Helper()
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatalf("run failed: %v\noutput so far: %q", err, out.String())
	}
	return out.String()
}

func TestJSONTreeParsesEveryKindAndMatchesPayloads(t *testing.T) {
	source := `word kind(doc: Json) -> String
    match doc
        case Json.Null
            "null"
        case Json.Bool(value)
            "bool"
        case Json.Text(value)
            "text"
        case Json.Number(literal)
            "number"
        case Json.List(values)
            "list"
        case Json.Object(fields)
            "object"
let doc = json.parse("{\"flag\":true,\"none\":null,\"name\":\"Ada\",\"ratio\":1.50,\"tags\":[\"a\",3],\"inner\":{\"deep\":false}}") try
print(kind(doc))
match doc
    case Json.Object(fields)
        print(map.length(fields))
        for key, value in fields
            print(key + ":" + kind(value))
        match fields["flag"]
            case Option.Some(value)
                match value
                    case Json.Bool(flag)
                        print(flag)
                    case _
                        print("not a bool")
            case Option.None
                print("missing")
        match fields["none"]
            case Option.Some(value)
                print(kind(value))
            case Option.None
                print("missing")
        match fields["name"]
            case Option.Some(value)
                match value
                    case Json.Text(name)
                        print(name)
                    case _
                        print("not text")
            case Option.None
                print("missing")
        match fields["ratio"]
            case Option.Some(value)
                match value
                    case Json.Number(literal)
                        print(literal)
                    case _
                        print("not a number")
            case Option.None
                print("missing")
        match fields["tags"]
            case Option.Some(value)
                match value
                    case Json.List(items)
                        print(list.length(items))
                        for item in items
                            print(kind(item))
                    case _
                        print("not a list")
            case Option.None
                print("missing")
        match fields["inner"]
            case Option.Some(value)
                match value
                    case Json.Object(inner)
                        match inner["deep"]
                            case Option.Some(flag)
                                match flag
                                    case Json.Bool(deep)
                                        print(deep)
                                    case _
                                        print("not a bool")
                            case Option.None
                                print("missing")
                    case _
                        print("not an object")
            case Option.None
                print("missing")
    case _
        print("not an object")
`
	want := "object\n6\n" +
		"flag:bool\nnone:null\nname:text\nratio:number\ntags:list\ninner:object\n" +
		"true\nnull\nAda\n1.50\n2\ntext\nnumber\nfalse\n"
	if got := runTree(t, source); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSONTreeRoundTripPreservesFidelity(t *testing.T) {
	source := `let raw = "{ \"b\" : 1.50 , \"a\" : [ null , -0.5e+2 , 7 ] , \"c\" : \"x y\" }"
let doc = json.parse(raw) try
print(json.encode(doc) try)
`
	want := "{\"b\":1.50,\"a\":[null,-0.5e+2,7],\"c\":\"x y\"}\n"
	if got := runTree(t, source); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestJSONTreeRejectsDuplicateKeys(t *testing.T) {
	source := `let result = json.parse("{\"a\":1,\"a\":2}") catch problem
    print(error.code(problem))
    Json.Null
`
	if got := runTree(t, source); got != "json-field\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeDepthLimitMatchesSharedSubstrate(t *testing.T) {
	deep := strings.Repeat("[", 256) + "1" + strings.Repeat("]", 256)
	source := "let doc = json.parse(\"" + deep + "\") try\nprint(json.encode(doc) try)\n"
	if got := runTree(t, source); got != deep+"\n" {
		t.Fatalf("256 levels did not round-trip: %q", got[:min(80, len(got))])
	}
	tooDeep := strings.Repeat("[", 257) + "1" + strings.Repeat("]", 257)
	source = "let result = json.parse(\"" + tooDeep + "\") catch problem\n    print(error.code(problem))\n    Json.Null\n"
	if got := runTree(t, source); got != "json-depth\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeListElementCap(t *testing.T) {
	big := "[" + strings.Repeat("1,", 1000000) + "1]"
	source := "let result = json.parse(\"" + big + "\") catch problem\n    print(error.code(problem))\n    Json.Null\n"
	if got := runTree(t, source); got != "json-range\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeEncodeValidatesNumberLiterals(t *testing.T) {
	source := `let result = json.encode(Json.Number("1.2.3")) catch problem
    print(error.code(problem))
    print(error.detail(problem, "path"))
    ""
`
	if got := runTree(t, source); got != "json-type\n\n" {
		t.Fatalf("got %q", got)
	}
	source = `let fields: Map<String, Json> = {}
fields["n"] = Json.Number("abc")
let result = json.encode(Json.Object(fields)) catch problem
    print(error.code(problem))
    print(error.detail(problem, "path"))
    ""
`
	if got := runTree(t, source); got != "json-type\n/n\n" {
		t.Fatalf("got %q", got)
	}
	source = `print(json.encode(Json.Number("1.50")) try)
print(json.encode(Json.Number("-0")) try)
print(json.encode(Json.Number("1e3")) try)
print(json.encode(Json.Null) try)
print(json.encode(Json.List([Json.Null, Json.Bool(true)])) try)
`
	if got := runTree(t, source); got != "1.50\n-0\n1e3\nnull\n[null,true]\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeOptionLookupDistinguishesNullFromMissing(t *testing.T) {
	source := `let doc = json.parse("{\"there\":null}") try
match doc
    case Json.Object(fields)
        match fields["there"]
            case Option.Some(value)
                print("present")
                match value
                    case Json.Null
                        print("null value")
                    case _
                        print("other")
            case Option.None
                print("missing")
        match fields["absent"]
            case Option.Some(value)
                print("present")
            case Option.None
                print("missing")
    case _
        print("not an object")
`
	if got := runTree(t, source); got != "present\nnull value\nmissing\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeRecursiveWordFitsAtDepthEdge(t *testing.T) {
	word := `word depth(doc: Json) -> Int
    match doc
        case Json.List(values)
            1 + depth(values[0])
        case _
            0
`
	// 255 nested containers plus the leaf value are exactly 256 word frames.
	fits := strings.Repeat("[", 255) + "1" + strings.Repeat("]", 255)
	source := word + "let doc = json.parse(\"" + fits + "\") try\nprint(depth(doc))\n"
	if got := runTree(t, source); got != "255\n" {
		t.Fatalf("got %q", got)
	}
	// One more container parses, but traversing it needs a 257th frame.
	over := strings.Repeat("[", 256) + "1" + strings.Repeat("]", 256)
	source = word + "let doc = json.parse(\"" + over + "\") try\nprint(depth(doc))\n"
	var out bytes.Buffer
	err := Run("test.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "word call depth limit of 256 exceeded") {
		t.Fatalf("wanted word call depth error, got %v", err)
	}
}

func TestJSONTreeRecordHoldingJsonField(t *testing.T) {
	source := `type Entry = record
    name: String
    payload: Json
    extra: List<Json>
let entry = Entry("metrics", Json.Null, [Json.Number("7")])
print(entry.name)
print(json.encode(entry.payload) try)
word label(entry: Entry) -> String
    match entry.payload
        case Json.Null
            entry.name + ": empty"
        case _
            entry.name + ": set"
print(label(entry))
`
	if got := runTree(t, source); got != "metrics\nnull\nmetrics: empty\n" {
		t.Fatalf("got %q", got)
	}
}

func TestJSONTreeDeclarationsStayHostOwned(t *testing.T) {
	for _, source := range []string{
		"type Json = record\n    value: Int\n",
		"word Json.Null() -> Int\n    0\n",
		"type Loop = record\n    next: Loop\n",
		"let doc = json.decode(\"1\", Json.Null) try\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted:\n%s", source)
		}
	}
	resolve := func(name string) (ModuleSource, bool) {
		if name == "Json" {
			return ModuleSource{Path: "Json.zhex", Source: "word Null() -> Int\n    0\n"}, true
		}
		return ModuleSource{}, false
	}
	if _, err := CompileModules("main.zhex", "use Json\n", resolve); err == nil {
		t.Error("accepted imported word colliding with Json.Null")
	}
}

func TestJSONTreePrintHint(t *testing.T) {
	_, err := Compile("test.zhex", "let doc = json.parse(\"1\") try\nprint(doc)\n")
	if err == nil {
		t.Fatal("accepted printing a Json value")
	}
	if !strings.Contains(err.Error(), "print does not support type Json") {
		t.Fatalf("message: %v", err)
	}
	if !strings.Contains(err.Error(), "encode it with json.encode first") {
		t.Fatalf("hint missing: %v", err)
	}
}

func TestJSONTreePartialsAndFlows(t *testing.T) {
	source := `word count-nodes(doc: Json) -> Int
    match doc
        case Json.List(values)
            var total = 1
            for v in values
                total = total + count-nodes(v)
            total
        case _
            1
let raw = "[1,{\"k\":\"v\"}]"
let doc = json.parse(raw) try
let encode-tree: (Json) -> Result<String, Error> = json.encode(_)
print(encode-tree(doc) try)
let count = flow json.parse(raw) try
    count-nodes
print(count)
`
	want := "[1,{\"k\":\"v\"}]\n3\n"
	if got := runTree(t, source); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
