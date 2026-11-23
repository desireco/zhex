package lang

import (
	"strconv"
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

// TestTextEditingWords pins the #196 editing vocabulary: repeat, the pad
// pair with the space-fill default, wrap over pdf-doc's _wrap law, and the
// insert/remove pair clamping like text.slice.
func TestTextEditingWords(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"repeat concatenates whole copies", "print(text.repeat(\"ab\", 3))\nprint(text.repeat(\"ab\", 1))\n", "ababab\nab\n"},
		{"repeat count zero is empty text", "print(\"[\" + text.repeat(\"ab\", 0) + \"]\")\n", "[]\n"},
		{"repeat of empty text stays empty", "print(\"[\" + text.repeat(\"\", 4) + \"]\")\n", "[]\n"},
		{"repeat is code-point safe", "print(text.repeat(\"é\", 3))\n", "ééé\n"},
		{"repeat flows", "flow \"-\"\n    text.repeat(3)\n    print\n", "---\n"},
		{"repeat partial", "let dash = text.repeat(_, 3)\nprint dash(\"*\")\n", "***\n"},
		{"pad-left defaults to spaces", "print(\"[\" + text.pad-left(\"7\", 3) + \"]\")\n", "[  7]\n"},
		{"pad-right defaults to spaces", "print(\"[\" + text.pad-right(\"7\", 3) + \"]\")\n", "[7  ]\n"},
		{"pad-left takes a one-code-point fill", "print(text.pad-left(\"7\", 3, \".\"))\nprint(text.pad-left(\"7\", 3, \"·\"))\n", "..7\n··7\n"},
		{"pad-right takes a one-code-point fill", "print(text.pad-right(\"7\", 3, \".\"))\n", "7..\n"},
		{"pad to the exact length returns the text", "print(\"[\" + text.pad-left(\"abc\", 3, \"0\") + \"]\")\nprint(\"[\" + text.pad-right(\"abc\", 3, \"0\") + \"]\")\n", "[abc]\n[abc]\n"},
		{"pad never truncates", "print(text.pad-left(\"abc\", 2))\nprint(text.pad-right(\"abc\", 0, \".\"))\n", "abc\nabc\n"},
		{"pad counts code points", "print(\"[\" + text.pad-left(\"é\", 2) + \"]\")\n", "[ é]\n"},
		{"pad of empty text fills to width", "print(\"[\" + text.pad-left(\"\", 2, \"0\") + \"]\")\n", "[00]\n"},
		{"pad flows with the fill supplied", "flow \"1\"\n    text.pad-left(3, \"0\")\n    print\n", "001\n"},
		{"pad partial keeps the fill deferred", "let pad3 = text.pad-left(_, 3)\nprint(\"[\" + pad3(\"a\") + \"]\")\n", "[  a]\n"},
		{"wrap fills greedily", "let lines = text.wrap(\"aa bb cc\", 5)\nprint(list.length(lines))\nprint(lines[0] + \"|\" + lines[1])\n", "2\naa bb|cc\n"},
		{"wrap keeps a fitting line whole", "print(text.join(text.wrap(\"aaa bbb\", 7), \"|\"))\n", "aaa bbb\n"},
		{"wrap lets an over-long word overflow alone", "print(text.join(text.wrap(\"aaaaaaaa bb\", 5), \"|\"))\n", "aaaaaaaa|bb\n"},
		{"wrap of empty text is one empty line", "let lines = text.wrap(\"\", 4)\nprint(list.length(lines))\nprint(\"[\" + lines[0] + \"]\")\n", "1\n[]\n"},
		{"wrap of spaces-only text is one empty line", "print(list.length(text.wrap(\"  \", 4)))\n", "1\n"},
		{"wrap collapses space runs between words", "print(text.join(text.wrap(\"aa  bb\", 5), \"|\"))\n", "aa bb\n"},
		{"wrap at width one puts every word alone", "print(text.join(text.wrap(\"a b\", 1), \"|\"))\n", "a|b\n"},
		{"wrap counts code points", "print(text.join(text.wrap(\"éé x\", 3), \"|\"))\nprint(text.join(text.wrap(\"é x\", 3), \"|\"))\n", "éé|x\né x\n"},
		{"wrap flows", "flow \"aa bb cc\"\n    text.wrap(5)\n    list.length\n    print\n", "2\n"},
		{"insert places text at the index", "print(text.insert(\"ac\", 1, \"b\"))\nprint(text.insert(\"ac\", 0, \"b\"))\n", "abc\nbac\n"},
		{"insert at the length appends", "print(text.insert(\"ac\", 2, \"-\"))\n", "ac-\n"},
		{"insert clamps out-of-range indexes like slice", "print(text.insert(\"ac\", -5, \"x\"))\nprint(text.insert(\"ac\", 99, \"!\"))\n", "xac\nac!\n"},
		{"insert is code-point indexed", "print(text.insert(\"hélo\", 1, \"e\"))\n", "heélo\n"},
		{"insert into empty text at zero", "print(text.insert(\"\", 0, \"hi\"))\n", "hi\n"},
		{"insert of empty text changes nothing", "print(text.insert(\"ab\", 1, \"\"))\n", "ab\n"},
		{"insert may place multicharacter text", "print(text.insert(\"ac\", 1, \"XYZ\"))\n", "aXYZc\n"},
		{"insert flows", "flow \"ac\"\n    text.insert(1, \"b\")\n    print\n", "abc\n"},
		{"remove cuts the half-open range", "print(text.remove(\"hello\", 1, 3))\n", "hlo\n"},
		{"remove of the whole text is empty", "print(\"[\" + text.remove(\"hello\", 0, 5) + \"]\")\n", "[]\n"},
		{"remove of an empty or inverted range changes nothing", "print(text.remove(\"hello\", 2, 2))\nprint(text.remove(\"hello\", 3, 2))\n", "hello\nhello\n"},
		{"remove clamps like slice", "print(\"[\" + text.remove(\"hello\", -2, 99) + \"]\")\nprint(text.remove(\"hello\", -2, 2))\n", "[]\nllo\n"},
		{"remove is code-point indexed", "print(text.remove(\"héélo\", 1, 3))\n", "hlo\n"},
		{"remove of empty text is empty", "print(\"[\" + text.remove(\"\", 0, 5) + \"]\")\n", "[]\n"},
		{"remove flows", "flow \"hello\"\n    text.remove(1, 3)\n    print\n", "hlo\n"},
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

// TestTextEditingTyping pins the argument diagnostics of the editing words:
// the pad pair accepts 2 to 3 arguments (the fill default), the others their
// exact arities.
func TestTextEditingTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"repeat argument count", "print(text.repeat(\"a\"))\n", "text.repeat requires 2 arguments, found 1"},
		{"repeat count type", "print(text.repeat(\"a\", \"b\"))\n", "argument 2 of text.repeat requires type Int, found String"},
		{"repeat value type", "print(text.repeat(1, 2))\n", "argument 1 of text.repeat requires type String, found Int"},
		{"pad-left short arity", "print(text.pad-left(\"a\"))\n", "text.pad-left requires 2 to 3 arguments, found 1"},
		{"pad-left long arity", "print(text.pad-left(\"a\", 1, \"b\", \"c\"))\n", "text.pad-left requires 2 to 3 arguments, found 4"},
		{"pad-left fill type", "print(text.pad-left(\"a\", 1, 2))\n", "argument 3 of text.pad-left requires type String, found Int"},
		{"pad-right width type", "print(text.pad-right(\"a\", \"b\"))\n", "argument 2 of text.pad-right requires type Int, found String"},
		{"pad-right long arity", "print(text.pad-right(\"a\", 1, \"b\", true))\n", "text.pad-right requires 2 to 3 arguments, found 4"},
		{"wrap value type", "print(text.wrap(1, 2))\n", "argument 1 of text.wrap requires type String, found Int"},
		{"wrap width type", "print(text.wrap(\"a\", \"b\"))\n", "argument 2 of text.wrap requires type Int, found String"},
		{"insert index type", "print(text.insert(\"a\", \"b\", \"c\"))\n", "argument 2 of text.insert requires type Int, found String"},
		{"remove argument count", "print(text.remove(\"a\", 1))\n", "text.remove requires 3 arguments, found 2"},
		{"remove end type", "print(text.remove(\"a\", 1, \"b\"))\n", "argument 3 of text.remove requires type Int, found String"},
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

// TestPadAndRepeatValidationIsABugSignal pins the hard errors: a negative pad
// width, a fill that is not exactly one code point, and a negative repeat
// count have no sane result, so the run exits with their stable codes — no
// hint, because there is nothing to suggest but a valid argument.
func TestPadAndRepeatValidationIsABugSignal(t *testing.T) {
	for _, tc := range []struct{ code, source, first string }{
		{"text-pad-width", "print(text.pad-left(\"a\", -1))\n", "bad.zhex:1:7: text.pad-left width is negative"},
		{"text-pad-width", "print(text.pad-right(\"a\", -1))\n", "bad.zhex:1:7: text.pad-right width is negative"},
		{"text-pad-fill", "print(text.pad-left(\"a\", 2, \"\"))\n", "bad.zhex:1:7: text.pad-left fill must be exactly one code point"},
		{"text-pad-fill", "print(text.pad-right(\"a\", 2, \"ab\"))\n", "bad.zhex:1:7: text.pad-right fill must be exactly one code point"},
		{"text-repeat-count", "print(text.repeat(\"a\", -1))\n", "bad.zhex:1:7: text.repeat count is negative"},
		{"text-repeat-count", "print(text.repeat(\"x\", 4611686018427387904))\n", "bad.zhex:1:7: text.repeat count 4611686018427387904 is out of range"},
		{"text-pad-width", "print(text.pad-left(\"x\", 4611686018427387904, \"0\"))\n", "bad.zhex:1:7: text.pad-left width 4611686018427387904 is out of range"},
		{"text-pad-width", "print(text.pad-right(\"x\", 4611686018427387904, \"0\"))\n", "bad.zhex:1:7: text.pad-right width 4611686018427387904 is out of range"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			err := Run("bad.zhex", tc.source, strings.NewReader(""), &strings.Builder{})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), tc.first) {
				t.Fatalf("first line changed: %q", err.Error())
			}
			if strings.Contains(err.Error(), "hint:") {
				t.Fatalf("%s must carry no hint: %q", tc.code, err.Error())
			}
			report, ok := Report(err)
			if !ok || !strings.Contains(report, `"code":"`+tc.code+`"`) {
				t.Fatalf("wanted code %s, got %s", tc.code, report)
			}
		})
	}
}

// TestWrapElementLimit pins the hard stop before materializing: a wrap that
// would produce more than the runtime element limit fails with the shared
// collection-limit code, the same limit as text.split.
func TestWrapElementLimit(t *testing.T) {
	var out strings.Builder
	source := "var big = \"a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a\"\nfor i in range(1, 16)\n    big = big + \" \" + big\nprint(list.length(text.wrap(big, 1)))\n"
	err := Run("wrap.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "text.wrap exceeds element limit") {
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

// TestTextSentences pins the #218 sentence layer: the written-down boundary
// law — a run of the terminators . ! ?, optional closing quotes or
// brackets, then whitespace or the end — with every sentence keeping its
// punctuation and the whitespace between sentences dropped.
func TestTextSentences(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"each terminator ends a sentence", "print(text.join(text.sentences(\"One. Two! Three?\"), \"|\"))\n", "One.|Two!|Three?\n"},
		{"closing quotes attach to the sentence", "print(text.join(text.sentences(\"He said \\\"Stop.\\\" Then left.\"), \"|\"))\n", "He said \"Stop.\"|Then left.\n"},
		{"single quotes attach", "print(text.join(text.sentences(\"He called it 'fine.' Then left.\"), \"|\"))\n", "He called it 'fine.'|Then left.\n"},
		{"typographic quotes attach", "print(text.join(text.sentences(\"“Done.” Next. ‘Done.’ Next.\"), \"|\"))\n", "“Done.”|Next.|‘Done.’|Next.\n"},
		{"brackets attach", "print(text.join(text.sentences(\"(Really?) Yes. [Why?] Because.\"), \"|\"))\n", "(Really?)|Yes.|[Why?]|Because.\n"},
		{"a terminator after the closers still ends the sentence", "print(text.join(text.sentences(\"(What?)! Next.\"), \"|\"))\n", "(What?)!|Next.\n"},
		{"consecutive terminators stay together", "print(text.join(text.sentences(\"Wait... Really? What?! Fine.\"), \"|\"))\n", "Wait...|Really?|What?!|Fine.\n"},
		{"mixed consecutive terminators stay together", "print(text.join(text.sentences(\"What!? Really.\"), \"|\"))\n", "What!?|Really.\n"},
		{"a terminator needs whitespace or the end after it", "print(text.join(text.sentences(\"3.14 is pi. a.b.c\"), \"|\"))\n", "3.14 is pi.|a.b.c\n"},
		{"abbreviations split", "print(text.join(text.sentences(\"Dr. Smith arrived, e.g. today.\"), \"|\"))\n", "Dr.|Smith arrived, e.g.|today.\n"},
		{"interior whitespace stays inside the sentence", "print(text.join(text.sentences(\"One two.  Three   four.\"), \"|\"))\n", "One two.|Three   four.\n"},
		{"a newline inside a sentence stays", "print(text.join(text.sentences(\"One\\ntwo. Three.\"), \"|\"))\n", "One\ntwo.|Three.\n"},
		{"newlines separate sentences", "print(text.join(text.sentences(\"One.\\nTwo.\\n\\nThree.\"), \"|\"))\n", "One.|Two.|Three.\n"},
		{"whitespace around sentences is dropped", "print(\"[\" + text.join(text.sentences(\"  One.  Two!  \"), \"|\") + \"]\")\n", "[One.|Two!]\n"},
		{"empty text is an empty list", "print(list.length(text.sentences(\"\")))\n", "0\n"},
		{"whitespace-only text is an empty list", "print(list.length(text.sentences(\" \\n\\t\")))\n", "0\n"},
		{"Unicode whitespace-only text is an empty list", "print(list.length(text.sentences(\"\\u00a0\\u2028 \")))\n", "0\n"},
		{"a final stretch without a terminator is a sentence", "print(text.join(text.sentences(\"One. Two\"), \"|\"))\n", "One.|Two\n"},
		{"text without a terminator is one sentence", "print(text.join(text.sentences(\"hello world\"), \"|\"))\n", "hello world\n"},
		{"terminators alone are sentences", "print(text.join(text.sentences(\"... ?!\"), \"|\"))\n", "...|?!\n"},
		{"apostrophes inside words are ordinary", "print(text.join(text.sentences(\"It's John's. Right?\"), \"|\"))\n", "It's John's.|Right?\n"},
		{"a bracket without a terminator does not end a sentence", "print(text.join(text.sentences(\"(a b) c.\"), \"|\"))\n", "(a b) c.\n"},
		{"non-ASCII text splits on the same terminators", "print(text.join(text.sentences(\"Été. À bientôt 😀. 🙂?\"), \"|\"))\n", "Été.|À bientôt 😀.|🙂?\n"},
		{"returns fresh lists", "let a = text.sentences(\"A. B.\")\nlet b = text.sentences(\"A. B.\")\nlist.append(a, \"C.\")\nprint(list.length(a))\nprint(list.length(b))\n", "3\n2\n"},
		{"flows", "flow \"One. Two!\"\n    text.sentences\n    list.length\n    print\n", "2\n"},
		{"partial", "let sentences = text.sentences(_)\nprint(list.length(sentences(\"A. B.\")))\n", "2\n"},
		{"a word returning the sentences infers List<String>", "word split(text: String)\n    text.sentences(text)\nprint(list.length(split(\"A. B.\")))\n", "2\n"},
		{"an editing pass finds the long sentence", "word long(sentence: String) -> Bool\n    text.word-count(sentence) > 3\nlet long-ones = list.filter(text.sentences(\"A short one. This sentence is far too long. Tiny.\"), long(_))\nprint(text.join(long-ones, \"|\"))\n", "This sentence is far too long.\n"},
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

// TestTextSentencesJoinRoundTrip pins the rejoin law: every sentence keeps
// its punctuation and closers, so joining with a single space restores a
// text exactly, and joining with the separator a text used restores that
// one too — the round trip loses only the whitespace between sentences.
func TestTextSentencesJoinRoundTrip(t *testing.T) {
	for name, text := range map[string]string{
		"terminators":     `One. Two! Three?`,
		"closers":         `He said "Stop!" and left. (Really?) Yes.`,
		"groups":          `Wait... Really? What?!`,
		"unterminated":    `One. Two`,
		"interior breaks": "One\ntwo. Three   four.",
		"abbreviations":   `Dr. Smith. e.g. this.`,
		"empty":           ``,
	} {
		t.Run(name, func(t *testing.T) {
			source := "print(text.join(text.sentences(" + strconv.Quote(text) + "), \" \") == " + strconv.Quote(text) + ")\n"
			if got := runText(t, source); got != "true\n" {
				t.Fatalf("joining with a space must restore %q, got %q", text, got)
			}
		})
	}
	if got := runText(t, "print(text.join(text.sentences("+strconv.Quote("A.\n\nB.")+"), "+strconv.Quote("\n\n")+"))\n"); got != "A.\n\nB.\n" {
		t.Fatalf("joining with the paragraph separator must restore the text, got %q", got)
	}
	if got := runText(t, "print(\"[\" + text.join(text.sentences("+strconv.Quote("  A.\n\nB. ")+"), \" \") + \"]\")\n"); got != "[A. B.]\n" {
		t.Fatalf("the round trip is exact only modulo the joining space, got %q", got)
	}
}

// TestTextSentencesTyping pins the argument diagnostics and the
// List<String> result of text.sentences.
func TestTextSentencesTyping(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"argument count zero", "print(list.length(text.sentences()))\n", "text.sentences requires 1 arguments, found 0"},
		{"argument count two", "print(list.length(text.sentences(\"a\", \"b\")))\n", "text.sentences requires 1 arguments, found 2"},
		{"value type", "print(list.length(text.sentences(1)))\n", "argument 1 of text.sentences requires type String, found Int"},
		{"result type", "let counts: List<Int> = text.sentences(\"A.\")\n", "binding counts has type List<String>; declared List<Int>"},
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

// TestSentencesElementLimit pins the hard stop before materializing: more
// sentences than the runtime element limit fails with the shared
// collection-limit code, the same limit as text.split — the limit itself
// passes, and the limit counts sentences, not terminators.
func TestSentencesElementLimit(t *testing.T) {
	var out strings.Builder
	source := "print(list.length(text.sentences(text.repeat(\"a. \", 1000001))))\n"
	err := Run("sentences.zhex", source, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "text.sentences exceeds element limit") {
		t.Fatalf("wanted element limit diagnostic, got %v", err)
	}
	report, ok := Report(err)
	if !ok || !strings.Contains(report, `"code":"collection-limit"`) {
		t.Fatalf("wanted code collection-limit, got %s", report)
	}
	if out.Len() != 0 {
		t.Fatalf("limit must stop the run before output, got %q", out.String())
	}
	if got := runText(t, "print(list.length(text.sentences(text.repeat(\"a. \", 1000000))))\n"); got != "1000000\n" {
		t.Fatalf("the limit itself must pass, got %q", got)
	}
	if got := runText(t, "print(list.length(text.sentences(text.repeat(\"a.\", 1000001) + \" x\")))\n"); got != "2\n" {
		t.Fatalf("a million terminators in two sentences must pass, got %q", got)
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
