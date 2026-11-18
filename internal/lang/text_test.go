package lang

import (
	"strings"
	"testing"
)

func TestTextWords(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"length counts code points", "print(text.length(\"héllo\"))\n", "5\n"},
		{"line-count", "print(text.line-count(\"\"))\nprint(text.line-count(\"a\"))\nprint(text.line-count(\"a\\n\"))\nprint(text.line-count(\"a\\nb\"))\n", "0\n1\n1\n2\n"},
		{"word-count splits on Unicode whitespace", "print(text.word-count(\"  a\\u00a0b\\tc \"))\n", "3\n"},
		{"upper maps Unicode", "print(text.upper(\"éà\"))\n", "ÉÀ\n"},
		{"lower maps Unicode", "print(text.lower(\"ÉÀ\"))\n", "éà\n"},
		{"trim cuts Unicode whitespace on both ends", "print(\"[\" + text.trim(\"\\u00a0 \\t hi \\n\\u00a0\") + \"]\")\n", "[hi]\n"},
		{"contains", "print(text.contains(\"hello\", \"ell\"))\nprint(text.contains(\"hello\", \"xyz\"))\n", "true\nfalse\n"},
		{"index-of reports the first occurrence in code points", "print(text.index-of(\"héllo\", \"l\"))\n", "2\n"},
		{"index-of absent is -1", "print(text.index-of(\"héllo\", \"z\"))\n", "-1\n"},
		{"slice is empty when start is at or past end", "print(text.slice(\"hello\", 3, 2))\nprint(text.slice(\"hello\", 2, 2))\n", "\n\n"},
		{"slice clamps below zero and past the length", "print(text.slice(\"hello\", -2, 99))\n", "hello\n"},
		{"slice is code-point indexed", "print(text.slice(\"héllo\", 1, 3))\n", "él\n"},
		{"replace rewrites every occurrence", "print(text.replace(\"a-b-c\", \"-\", \"+\"))\n", "a+b+c\n"},
		{"replace is literal, not a pattern", "print(text.replace(\"a.b.c\", \".\", \"!\"))\n", "a!b!c\n"},
		{"split preserves leading, repeated, and trailing empty fields", "let parts = text.split(\",a,,b,\", \",\")\nprint(list.length(parts))\nprint(parts[0] + \"|\" + parts[1] + \"|\" + parts[2] + \"|\" + parts[3] + \"|\" + parts[4])\n", "5\n|a||b|\n"},
		{"split keeps surrounding spaces", "let parts = text.split(\" a , b \", \",\")\nprint(\"[\" + parts[0] + \"][\" + parts[1] + \"]\")\n", "[ a ][ b ]\n"},
		{"split without the separator keeps the whole text", "let parts = text.split(\"abc\", \",\")\nprint(list.length(parts))\nprint(parts[0])\n", "1\nabc\n"},
		{"split on an empty separator yields code points", "let parts = text.split(\"héllo\", \"\")\nprint(list.length(parts))\nprint(parts[1])\n", "5\né\n"},
		{"split of empty text with an empty separator is an empty list", "print(list.length(text.split(\"\", \"\")))\n", "0\n"},
		{"split of empty text with a separator keeps one empty field", "print(list.length(text.split(\"\", \",\")))\n", "1\n"},
		{"split on a multibyte separator", "let parts = text.split(\"a😀b\", \"😀\")\nprint(list.length(parts))\nprint(parts[0] + parts[1])\n", "2\nab\n"},
		{"split keeps combining marks as code points", "let marks = text.split(\"e\\u0301|e\\u0302\", \"|\")\nprint(text.length(marks[0]))\nprint(marks[1])\n", "2\ne\u0302\n"},
		{"split returns fresh lists", "let a = text.split(\"x,y\", \",\")\nlet b = text.split(\"x,y\", \",\")\nlist.append(a, \"z\")\nprint(list.length(a))\nprint(list.length(b))\n", "3\n2\n"},
		{"join of an empty list is empty text", "print(\"[\" + text.join([], \"-\") + \"]\")\n", "[]\n"},
		{"join of one element has no separator", "print(text.join(text.split(\"solo\", \",\"), \"-\"))\n", "solo\n"},
		{"join places the separator between all elements", "print(text.join(text.split(\"a,b,c\", \",\"), \"-\"))\n", "a-b-c\n"},
		{"join keeps multibyte content", "print(text.join(text.split(\"1😀2\", \"😀\"), \"😀\"))\n", "1😀2\n"},
		{"starts-with matches a literal prefix", "print(text.starts-with(\"hello\", \"he\"))\nprint(text.starts-with(\"hello\", \"el\"))\nprint(text.starts-with(\"hello\", \"hello\"))\nprint(text.starts-with(\"hello\", \"hello!\"))\n", "true\nfalse\ntrue\nfalse\n"},
		{"starts-with accepts an empty prefix", "print(text.starts-with(\"hello\", \"\"))\nprint(text.starts-with(\"\", \"\"))\nprint(text.starts-with(\"\", \"x\"))\n", "true\ntrue\nfalse\n"},
		{"starts-with is code-point safe", "print(text.starts-with(\"héllo\", \"hé\"))\nprint(text.starts-with(\"héllo\", \"llo\"))\n", "true\nfalse\n"},
		{"ends-with matches a literal suffix", "print(text.ends-with(\"hello\", \"lo\"))\nprint(text.ends-with(\"hello\", \"el\"))\nprint(text.ends-with(\"hello\", \"hello\"))\nprint(text.ends-with(\"hello\", \"!hello\"))\n", "true\nfalse\ntrue\nfalse\n"},
		{"ends-with accepts an empty suffix", "print(text.ends-with(\"hello\", \"\"))\nprint(text.ends-with(\"\", \"\"))\nprint(text.ends-with(\"\", \"x\"))\n", "true\ntrue\nfalse\n"},
		{"ends-with is code-point safe", "print(text.ends-with(\"héllo\", \"llo\"))\nprint(text.ends-with(\"héllo\", \"hé\"))\n", "true\nfalse\n"},
		{"flow splits and rejoins", "flow \"a,b,c\"\n    text.split(\",\")\n    text.join(\"-\")\n    print\n", "a-b-c\n"},
		{"flow checks a suffix", "flow \"report.csv\"\n    text.ends-with(\".csv\")\n    print\n", "true\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runText(t, tc.source)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReplaceEmptyIsABugSignal pins the hard error: an empty old text has no
// sane result, so the run exits with replace-empty — stable first line, no
// hint, because there is nothing to suggest but a nonempty old text.
func TestReplaceEmptyIsABugSignal(t *testing.T) {
	err := Run("bad.zhex", "print(text.replace(\"abc\", \"\", \"x\"))\n", strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.HasPrefix(err.Error(), "bad.zhex:1:7: text.replace old text is empty") {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if strings.Contains(err.Error(), "hint:") {
		t.Fatalf("replace-empty must carry no hint: %q", err.Error())
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"replace-empty"`) {
		t.Fatalf("wanted code replace-empty, got %s", report)
	}
}

// TestSplitElementLimit pins the hard stop before materializing: a split that
// would produce more than the runtime element limit fails with the shared
// collection-limit code, the same limit as list.append and array.new.
func TestSplitElementLimit(t *testing.T) {
	for name, source := range map[string]string{
		"empty separator":    "var big = \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\nfor i in range(1, 16)\n    big = big + big\nlet parts = text.split(big, \"\")\nprint(list.length(parts))\n",
		"nonempty separator": "var big = \",,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,,\"\nfor i in range(1, 16)\n    big = big + big\nprint(list.length(text.split(big, \",\")))\n",
	} {
		t.Run(name, func(t *testing.T) {
			var out strings.Builder
			err := Run("split.zhex", source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), "text.split exceeds element limit") {
				t.Fatalf("wanted element limit diagnostic, got %v", err)
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"collection-limit"`) {
				t.Fatalf("wanted code collection-limit, got %s", report)
			}
			if out.Len() != 0 {
				t.Fatalf("limit must stop the run before output, got %q", out.String())
			}
		})
	}
}

// TestTextSplitJoinPrefixTyping pins the argument diagnostics of the new
// words: join takes a List<String>, the others take Strings.
func TestTextSplitJoinPrefixTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"join wants a list", "print(text.join(\"abc\", \"-\"))\n", "argument 1 of text.join requires type List<String>, found String"},
		{"join element type", "print(text.join([1, 2], \"-\"))\n", "list element has type Int; expected String"},
		{"split value type", "print(text.split(1, \",\"))\n", "argument 1 of text.split requires type String, found Int"},
		{"split separator type", "print(text.split(\"a\", 1))\n", "argument 2 of text.split requires type String, found Int"},
		{"split argument count", "print(text.split(\"a\"))\n", "text.split requires 2 arguments, found 1"},
		{"starts-with value type", "print(text.starts-with(1, \"a\"))\n", "argument 1 of text.starts-with requires type String, found Int"},
		{"starts-with prefix type", "print(text.starts-with(\"a\", 1))\n", "argument 2 of text.starts-with requires type String, found Int"},
		{"ends-with argument count", "print(text.ends-with(\"a\"))\n", "text.ends-with requires 2 arguments, found 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			err := Run("typing.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
	var out strings.Builder
	if err := Run("typing.zhex", "print(text.join([], \"-\") == \"\")\n", strings.NewReader(""), &out); err != nil {
		t.Fatalf("empty list literal must check against List<String>: %v", err)
	}
	if out.String() != "true\n" {
		t.Fatalf("got %q", out.String())
	}
}

// TestStringContextIntException pins the string-context Int exception: print
// already accepts Int, and "+" concatenates when exactly one operand is Int
// and the other is String. Int + Int stays numeric, and no other position
// converts.
func TestStringContextIntException(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"print accepts Int directly", "print 5\nprint(-7)\n", "5\n-7\n"},
		{"print keeps Bool text", "print true\n", "true\n"},
		{"text plus int", "print \"a\" + 1\n", "a1\n"},
		{"int plus text", "print 1 + \"a\"\n", "1a\n"},
		{"int plus int stays numeric", "print 1 + 2\n", "3\n"},
		{"mixed chains associate left to right", "print 1 + 2 + \"a\"\nprint 1 + \"a\" + 2\nprint \"a\" + 1 + \"b\"\n", "3a\n1a2\na1b\n"},
		{"negative int renders like int.to-string", "print \"value: \" + (2 - 5)\n", "value: -3\n"},
		{"bindings convert on either side", "let n = 4\nprint \"a\" + n\nprint n + \"a\"\n", "a4\n4a\n"},
		{"word body concatenates an annotated Int parameter", "word label(prefix: String, n: Int) -> String\n    prefix + n\nprint label(\"n=\", 5)\n", "n=5\n"},
		{"inferred return of a mixed concat is String", "word tag(n: Int)\n    \"n=\" + n\nprint tag(5)\nprint tag(5) + \"!\"\n", "n=5\nn=5!\n"},
		{"flow seed may mix", "flow \"a\" + 1\n    print\n", "a1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runText(t, tc.source)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTextCodePointWords pins the code-point conversion pair: a string's
// code points (the unit text.length already counts) as a fresh List<Int>,
// and the rebuild from valid code points only.
func TestTextCodePointWords(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"to-code-points of ASCII", "let cps = text.to-code-points(\"Hi\")\nprint(cps[0])\nprint(cps[1])\n", "72\n105\n"},
		{"to-code-points counts code points like length", "let cps = text.to-code-points(\"héllo\")\nprint(list.length(cps))\nprint(cps[1])\n", "5\n233\n"},
		{"to-code-points of an emoji", "print(text.to-code-points(\"😀\")[0])\n", "128512\n"},
		{"to-code-points of empty text is an empty list", "print(list.length(text.to-code-points(\"\")))\n", "0\n"},
		{"to-code-points returns fresh lists", "let a = text.to-code-points(\"ab\")\nlet b = text.to-code-points(\"ab\")\nlist.append(a, 0)\nprint(list.length(a))\nprint(list.length(b))\n", "3\n2\n"},
		{"from-code-points builds text", "print(text.from-code-points([72, 105]))\n", "Hi\n"},
		{"from-code-points of an empty list is empty text", "print(\"[\" + text.from-code-points([]) + \"]\")\n", "[]\n"},
		{"from-code-points keeps multibyte content", "print(text.from-code-points([233, 128512]))\n", "é😀\n"},
		{"round trip through a flow", "flow \"héllo😀\"\n    text.to-code-points\n    text.from-code-points\n    print\n", "héllo😀\n"},
		{"digits become ordinary Int arithmetic", "word digit(cp: Int) -> Int\n    cp - 48\nlet digits = list.map(text.to-code-points(\"12\"), digit(_))\nprint(digits[0] + digits[1])\n", "3\n"},
		{"from-code-points partial", "let encode = text.from-code-points(_)\nprint encode([72, 105])\n", "Hi\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runText(t, tc.source)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTextCodePointTyping pins the argument diagnostics: to-code-points takes
// one String, from-code-points one List<Int>.
func TestTextCodePointTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"to-code-points argument count", "print(text.to-code-points())\n", "text.to-code-points requires 1 arguments, found 0"},
		{"to-code-points value type", "print(text.to-code-points(1))\n", "argument 1 of text.to-code-points requires type String, found Int"},
		{"from-code-points wants a list", "print(text.from-code-points(\"ab\"))\n", "argument 1 of text.from-code-points requires type List<Int>, found String"},
		{"from-code-points element type", "print(text.from-code-points([\"a\"]))\n", "list element has type String; expected Int"},
		{"from-code-points argument count", "print(text.from-code-points([65], [66]))\n", "text.from-code-points requires 1 arguments, found 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			err := Run("typing.zhex", tc.source, strings.NewReader(""), &out)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}

// TestFromCodePointsInvalidIsABugSignal pins the hard error: negative values,
// surrogate halves, and anything above U+10FFFF have no UTF-8 encoding, so
// the run exits with text-code-point — stable first line, no hint, because
// there is nothing to suggest but a valid code point.
func TestFromCodePointsInvalidIsABugSignal(t *testing.T) {
	for name, literal := range map[string]string{
		"negative":  "-1",
		"surrogate": "55296",
		"above max": "1114112",
	} {
		t.Run(name, func(t *testing.T) {
			err := Run("bad.zhex", "print(text.from-code-points(["+literal+"]))\n", strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), "bad.zhex:1:7: text.from-code-points code point "+literal+" is invalid") {
				t.Fatalf("first line changed: %q", err.Error())
			}
			if strings.Contains(err.Error(), "hint:") {
				t.Fatalf("text-code-point must carry no hint: %q", err.Error())
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"text-code-point"`) {
				t.Fatalf("wanted code text-code-point, got %s", report)
			}
		})
	}
}

// TestToCodePointsElementLimit pins the hard stop before materializing: a
// string longer than the runtime element limit fails with the shared
// collection-limit code, the same limit as text.split.
func TestToCodePointsElementLimit(t *testing.T) {
	var out strings.Builder
	source := "var big = \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\nfor i in range(1, 16)\n    big = big + big\nprint(list.length(text.to-code-points(big)))\n"
	err := Run("cps.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "text.to-code-points exceeds element limit") {
		t.Fatalf("wanted element limit diagnostic, got %v", err)
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"collection-limit"`) {
		t.Fatalf("wanted code collection-limit, got %s", report)
	}
	if out.Len() != 0 {
		t.Fatalf("limit must stop the run before output, got %q", out.String())
	}
}

func runText(t *testing.T, source string) string {
	t.Helper()
	var out strings.Builder
	if err := Run("text.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
