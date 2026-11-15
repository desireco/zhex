package lang

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestXMLDecodeRecordDefaultsAttributesAndLists(t *testing.T) {
	source := "type Endpoint = record\n    host: String\n    port: Int\ntype Server = record\n    name: String\n    retries: Int\n    verbose: Bool\n    endpoints: List<Endpoint>\nlet server = xml.decode(\"<server retries=\\\"3\\\" verbose=\\\"true\\\"><name>ingest</name><endpoints><host>db</host><port>5432</port></endpoints><endpoints><host>web</host><port>443</port></endpoints></server>\", Server(\"backup\", 1, false, [])) try\nprint(server.name)\nprint(server.retries)\nprint(server.verbose)\nprint(list.length(server.endpoints))\nlet second = server.endpoints[1] try\nprint(second.host)\nprint(second.port)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ingest\n3\ntrue\n2\nweb\n443\n" {
		t.Fatal(out.String())
	}
}

func TestXMLRejectsUnknownCollisionDuplicateAndTypes(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"<server><ghost>1</ghost></server>", "xml-field"},
		{"<server ghost=\\\"1\\\"/>", "xml-field"},
		{"<server name=\\\"a\\\"><name>b</name></server>", "xml-field"},
		{"<server><name>a</name><name>b</name></server>", "xml-field"},
		{"<server><retries>soon</retries></server>", "xml-type"},
		{"<server retries=\\\"99999999999999999999\\\"/>", "xml-range"},
		{"<server verbose=\\\"yes\\\"/>", "xml-type"},
		{"<server>text<name>x</name></server>", "xml-type"},
		{"<server retries=\\\"1.5\\\"/>", "xml-type"},
	}
	for _, tc := range cases {
		source := "type Server = record\n    name: String\n    retries: Int\n    verbose: Bool\nlet result = xml.decode(\"" + tc.raw + "\", Server(\"backup\", 1, false)) catch problem\n    print(error.code(problem))\n    Server(\"backup\", 1, false)\n"
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != tc.want+"\n" {
			t.Fatalf("%s output %q", tc.raw, out.String())
		}
	}
}

func TestXMLOptionAbsentAndPresent(t *testing.T) {
	source := "type Server = record\n    name: String\n    port: Option<Int>\nlet absent = xml.decode(\"<server><name>a</name></server>\", Server(\"a\", Option.None)) try\nmatch absent.port\n    case Option.None\n        print(\"none\")\n    case Option.Some(value)\n        print(value)\nlet present = xml.decode(\"<server><name>a</name><port>80</port></server>\", Server(\"a\", Option.None)) try\nmatch present.port\n    case Option.None\n        print(\"none\")\n    case Option.Some(value)\n        print(value)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "none\n80\n" {
		t.Fatal(out.String())
	}
}

func TestXMLListAndMapDecode(t *testing.T) {
	source := "type Config = record\n    ports: List<Int>\n    tags: Map<String, Int>\nlet config = xml.decode(\"<config><ports>80</ports><ports>443</ports><tags><alpha>1</alpha><beta>2</beta></tags></config>\", Config([], {})) try\nprint(list.length(config.ports))\nprint(config.ports[1] try)\nprint(map.length(config.tags))\nprint(map.contains(config.tags, \"alpha\"))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n443\n2\ntrue\n" {
		t.Fatal(out.String())
	}
}

func TestXMLEntitiesDecodedAndDTDRejected(t *testing.T) {
	source := "type Doc = record\n    title: String\n    body: String\nlet doc = xml.decode(\"<doc title=\\\"a &amp; b\\\"><body>x &lt;y&gt; &#65;</body></doc>\", Doc(\"\", \"\")) try\nprint(doc.title)\nprint(doc.body)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "a & b\nx <y> A\n" {
		t.Fatal(out.String())
	}
	for _, raw := range []string{"<!DOCTYPE doc [<!ENTITY x \\\"y\\\">]><doc/>", "<!DOCTYPE doc><doc/>", "<doc>&x;</doc>"} {
		source = "type Doc = record\n    title: String\nlet result = xml.decode(\"" + raw + "\", Doc(\"\")) catch problem\n    print(error.code(problem))\n    Doc(\"\")\n"
		out.Reset()
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "xml-syntax\n" {
			t.Fatalf("%s output %q", raw, out.String())
		}
	}
}

func TestXMLMalformedReportsSyntaxWithPosition(t *testing.T) {
	for _, raw := range []string{"<doc>", "<doc><body></doc>", "<doc/><doc/>", "", "<doc><?pi x?></doc>"} {
		source := "type Doc = record\n    title: String\nlet result = xml.decode(\"" + raw + "\", Doc(\"\")) catch problem\n    print(error.code(problem))\n    Doc(\"\")\n"
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "xml-syntax\n" {
			t.Fatalf("%q output %q", raw, out.String())
		}
	}
	source := "type Doc = record\n    title: String\nlet result = xml.decode(\"<doc>\\n<body></doc>\", Doc(\"\")) catch problem\n    print(error.detail(problem, \"line\"))\n    Doc(\"\")\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n" {
		t.Fatal(out.String())
	}
	source = "type Doc = record\n    title: String\nlet result = xml.decode(\"<doc>&x;</doc>\", Doc(\"\")) catch problem\n    print(error.detail(problem, \"offset\"))\n    Doc(\"\")\n"
	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n" {
		t.Fatal(out.String())
	}
}

func TestXMLPrefixedNamesRejected(t *testing.T) {
	for _, raw := range []string{"<a:b/>", "<doc b:c=\\\"1\\\"/>", "<doc xmlns=\\\"http://x\\\"/>"} {
		source := "type Doc = record\n    title: String\nlet result = xml.decode(\"" + raw + "\", Doc(\"\")) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"reason\"))\n    Doc(\"\")\n"
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
			t.Fatal(err)
		}
		if out.String() != "xml-field\nprefixed\n" {
			t.Fatalf("%s output %q", raw, out.String())
		}
	}
}

func TestXMLDepthLimit(t *testing.T) {
	schema := &jsonSchema{typ: stringType, kind: "string"}
	for i := 0; i < 255; i++ {
		schema = &jsonSchema{typ: valueType(fmt.Sprintf("R%d", i)), kind: "record", fields: []jsonField{{name: "a", schema: schema}}}
	}
	pos := position{file: "test.zhex", line: 1, column: 1}
	raw := strings.Repeat("<a>", 256) + "x" + strings.Repeat("</a>", 256)
	if v := decodeXML(pos, raw, value{}, schema); v.result.err != nil {
		t.Fatalf("depth 256 rejected: %v", v.result.err)
	}
	raw = strings.Repeat("<a>", 257) + strings.Repeat("</a>", 257)
	v := decodeXML(pos, raw, value{}, schema)
	if v.result == nil || v.result.err == nil || v.result.err.(*diagnostic).code != CodeXMLDepth {
		t.Fatalf("depth 257 accepted: %v", v.result)
	}
}

func TestXMLUnsupportedSchemaRejectedBeforeExecution(t *testing.T) {
	for _, source := range []string{"let data = buffer.new()\nxml.decode(\"<a/>\", data) try\n", "let m: Map<Int, String> = {}\nxml.decode(\"<a/>\", m) try\n", "let grid = array.new([2, 2], 0) try\nxml.decode(\"<a/>\", grid) try\n", "type Status = choice\n    On\nxml.decode(\"<a/>\", Status.On) try\n"} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted unsupported schema:\n%s", source)
		}
	}
}

func TestXMLDefaultsAreClonedAndPresentListsReplace(t *testing.T) {
	source := "type Settings = record\n    left: List<Int>\n    right: List<Int>\nlet shared = [1]\nlet defaults = Settings(shared, shared)\nlet first = xml.decode(\"<settings><left>7</left></settings>\", defaults) try\nfirst.left[0] = 9\nprint(first.left[0] try)\nprint(first.right[0] try)\nprint(defaults.left[0] try)\nlet second = xml.decode(\"<settings/>\", defaults) try\nprint(second.left[0] try)\nsecond.left[0] = 5\nprint(defaults.left[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n1\n1\n1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestXMLScalarRootAndEmptyElements(t *testing.T) {
	source := "let n = xml.decode(\"<n>7</n>\", 0) try\nprint(n)\nlet s = xml.decode(\"<s></s>\", \"\") try\nprint(s == \"\")\nlet closed = xml.decode(\"<s/>\", \"x\") try\nprint(closed == \"\")\nlet result = xml.decode(\"<n></n>\", 0) catch problem\n    print(error.code(problem))\n    0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\ntrue\ntrue\nxml-type\n" {
		t.Fatal(out.String())
	}
}

func TestXMLWorksThroughInferredFlow(t *testing.T) {
	source := "type Settings = record\n    retries: Int\nword parse(raw: String) -> Settings\n    flow raw\n        xml.decode(Settings(3)) try\nlet settings = parse(\"<settings><retries>5</retries></settings>\") try\nprint(settings.retries)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n" {
		t.Fatal(out.String())
	}
}

func TestXMLReportsPathsAndReasons(t *testing.T) {
	source := "type Server = record\n    name: String\n    retries: Int\nlet result = xml.decode(\"<server><ghost>1</ghost></server>\", Server(\"x\", 1)) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"path\"))\n    print(error.detail(problem, \"reason\"))\n    Server(\"x\", 1)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "xml-field\n/ghost\nunknown\n" {
		t.Fatal(out.String())
	}
	source = "type Endpoint = record\n    host: String\n    port: Int\nlet defaults: List<Endpoint> = []\nlet result = xml.decode(\"<list><endpoint><host>db</host></endpoint></list>\", defaults) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"path\"))\n    print(error.detail(problem, \"reason\"))\n    defaults\n"
	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "xml-field\n/0/port\nmissing\n" {
		t.Fatal(out.String())
	}
	source = "type Server = record\n    name: String\n    retries: Int\nlet result = xml.decode(\"<server retries=\\\"soon\\\"/>\", Server(\"x\", 1)) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"expected\"))\n    Server(\"x\", 1)\n"

	out.Reset()
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "xml-type\nInt\n" {
		t.Fatal(out.String())
	}
}
