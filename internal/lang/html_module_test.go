package lang

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runHTML runs a main program that imports the html sibling module, the
// same resolution rule the CLI applies next to examples/modules/html-page.
func runHTML(t *testing.T, source string) string {
	t.Helper()
	modulePath := filepath.Join("..", "..", "examples", "modules", "html-page", "html.zhex")
	src, err := os.ReadFile(modulePath)
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(name string) (ModuleSource, bool) {
		if name != "html" {
			return ModuleSource{}, false
		}
		return ModuleSource{Path: modulePath, Source: string(src)}, true
	}
	var out strings.Builder
	if err := RunWithModulesOptions("main.zhex", "use html\n"+source, strings.NewReader(""), &out, RunOptions{}, resolve); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return out.String()
}

// TestHTMLEscapeTextPinsGoSemantics pins escape-text to Go's
// html.EscapeString over the ticket's text fixtures: Unicode passes
// through, the five entities escape, and no unescaped < survives.
func TestHTMLEscapeTextPinsGoSemantics(t *testing.T) {
	fixtures := []string{
		"",
		"plain text",
		"café — ünïcode ✓",
		"5 < 6 & 7 > 4",
		"she said \"hi\" and 'bye'",
		"<script>alert(1)</script>",
		"&amp; already escaped",
		"mixed ' \" < > & tail",
	}
	var b strings.Builder
	for i, fixture := range fixtures {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("print(\"[\" + html.escape-text(\"")
		b.WriteString(strings.NewReplacer("\\", `\\`, "\"", `\"`).Replace(fixture))
		b.WriteString("\") + \"]\")\n")
	}
	out := runHTML(t, b.String())
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != len(fixtures) {
		t.Fatalf("got %d lines, want %d", len(lines), len(fixtures))
	}
	for i, fixture := range fixtures {
		want := "[" + html.EscapeString(fixture) + "]"
		if lines[i] != want {
			t.Errorf("fixture %q: got %q, want %q", fixture, lines[i], want)
		}
		if strings.ContainsAny(html.EscapeString(fixture), "<>'\"") {
			t.Errorf("fixture %q: escape output still carries a raw delimiter", fixture)
		}
	}
}

// TestHTMLRawBypassesEveryEscapingProperty pins the unsafe contract: raw
// is the identity, documented bypass; it never escapes, and nothing
// escaped becomes markup by passing through it.
func TestHTMLRawBypassesEveryEscapingProperty(t *testing.T) {
	out := runHTML(t, `print(html.raw("<script>alert(\"x\")</script>"))
print("[" + html.raw("") + "]")
print(html.escape-text(html.raw("<b>bold</b> & 'more'")))
`)
	want := "<script>alert(\"x\")</script>\n[]\n&lt;b&gt;bold&lt;/b&gt; &amp; &#39;more&#39;\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// TestHTMLElementAssembly pins element and attr: pre-escaped attributes
// joined with spaces, attribute values escaped including quotes, empty
// content, and nested composition through raw.
func TestHTMLElementAssembly(t *testing.T) {
	out := runHTML(t, `print(html.element("p", [], "plain"))
print(html.element("a", [html.attr("href", "https://example.io"), html.attr("class", "btn")], "label"))
print(html.attr("title", "say \"hi\" & <bye>"))
print(html.element("br", [], ""))
print(html.element("div", [], html.raw(html.element("ul", [], html.raw(html.element("li", [], html.escape-text("a < b & 'c'")))))))
`)
	want := "<p>plain</p>\n" +
		"<a href=\"https://example.io\" class=\"btn\">label</a>\n" +
		"title=\"say &#34;hi&#34; &amp; &lt;bye&gt;\"\n" +
		"<br></br>\n" +
		"<div><ul><li>a &lt; b &amp; &#39;c&#39;</li></ul></div>\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// TestHTMLPageAssembly pins page: doctype, escaped title, verbatim body.
func TestHTMLPageAssembly(t *testing.T) {
	out := runHTML(t, `print(html.page("Ti&t <e> \"q\"", "<p>body & markup</p>"))
`)
	want := "<!DOCTYPE html>\n<html>\n<head>\n<title>Ti&amp;t &lt;e&gt; &#34;q&#34;</title>\n</head>\n<body>\n<p>body & markup</p>\n</body>\n</html>\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// TestHTMLLinkURLPolicy pins the allowlist: relative, http, https, and
// mailto targets pass; javascript:, data:, any other scheme, and
// case-trick variants are rejected with the html-url-scheme code.
func TestHTMLLinkURLPolicy(t *testing.T) {
	allowed := []string{
		"docs/getting-started.html",
		"/root-relative",
		"#fragment",
		"//example.io/shared",
		"https://example.io/zhex",
		"http://example.io",
		"mailto:ada@example.io",
	}
	var b strings.Builder
	for _, href := range allowed {
		b.WriteString("print(html.link(\"go\", \"" + href + "\") try)\n")
	}
	b.WriteString(`let a = html.link("run it", "javascript:alert(1)") catch problem
    print(error.code(problem))
    ""
let c = html.link("run it", "data:text/html,x") catch problem
    print(error.code(problem))
    ""
let d = html.link("run it", "JaVaScRiPt:alert(1)") catch problem
    print(error.code(problem))
    ""
let e = html.link("run it", "vbscript:msgbox") catch problem
    print(error.code(problem))
    ""
let f = html.link("run it", "ftp://files.example.io") catch problem
    print(error.code(problem))
    ""
`)
	out := runHTML(t, b.String())
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	wantAllowed := []string{
		"<a href=\"docs/getting-started.html\">go</a>",
		"<a href=\"/root-relative\">go</a>",
		"<a href=\"#fragment\">go</a>",
		"<a href=\"//example.io/shared\">go</a>",
		"<a href=\"https://example.io/zhex\">go</a>",
		"<a href=\"http://example.io\">go</a>",
		"<a href=\"mailto:ada@example.io\">go</a>",
	}
	if len(lines) != len(wantAllowed)+5 {
		t.Fatalf("got %d lines, want %d", len(lines), len(wantAllowed)+5)
	}
	for i, want := range wantAllowed {
		if lines[i] != want {
			t.Errorf("allowed %d: got %q, want %q", i, lines[i], want)
		}
	}
	for i, line := range lines[len(wantAllowed):] {
		if line != "html-url-scheme" {
			t.Errorf("rejected %d: got %q, want html-url-scheme", i, line)
		}
	}
}

// TestHTMLLinkEscapesLabelAndTarget pins that both link positions escape.
func TestHTMLLinkEscapesLabelAndTarget(t *testing.T) {
	out := runHTML(t, `print(html.link("a <b> & \"c\"", "https://example.io/?q=1&z=2") try)
`)
	want := "<a href=\"https://example.io/?q=1&amp;z=2\">a &lt;b&gt; &amp; &#34;c&#34;</a>\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// TestHTMLSharedTextAlias pins that module words hold no state: one
// binding passed through several words keeps producing the same output.
// (The module takes no Buffer parameters, so buffer aliasing cannot arise.)
func TestHTMLSharedTextAlias(t *testing.T) {
	out := runHTML(t, `let shared = "a & b <c> 'd' \"e\""
print(html.escape-text(shared))
print(html.attr("title", shared))
print(html.escape-text(shared))
`)
	want := "a &amp; b &lt;c&gt; &#39;d&#39; &#34;e&#34;\n" +
		"title=\"a &amp; b &lt;c&gt; &#39;d&#39; &#34;e&#34;\"\n" +
		"a &amp; b &lt;c&gt; &#39;d&#39; &#34;e&#34;\n"
	if out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
