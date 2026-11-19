package zhexrt

import (
	"strings"
	"unicode/utf8"
)

// The text words, moved from the interpreter's host functions: the same
// rune semantics, the same failure codes and messages.

func TextReverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func TextLength(s string) Int { return I64(int64(utf8.RuneCountInString(s))) }

func TextLineCount(s string) Int {
	if s == "" {
		return I64(0)
	}
	count := int64(strings.Count(s, "\n"))
	if !strings.HasSuffix(s, "\n") {
		count++
	}
	return I64(count)
}

func TextWordCount(s string) Int { return I64(int64(len(strings.Fields(s)))) }
func TextUpper(s string) string  { return strings.ToUpper(s) }
func TextLower(s string) string  { return strings.ToLower(s) }
func TextTrim(s string) string   { return strings.TrimSpace(s) }
func TextContains(s, sub string) bool {
	return strings.Contains(s, sub)
}
func TextStartsWith(s, prefix string) bool { return strings.HasPrefix(s, prefix) }
func TextEndsWith(s, suffix string) bool   { return strings.HasSuffix(s, suffix) }

func TextIndexOf(s, sub string) Int {
	at := strings.Index(s, sub)
	if at < 0 {
		return I64(-1)
	}
	return I64(int64(utf8.RuneCountInString(s[:at])))
}

func TextSlice(pos Pos, s string, start, end Int) string {
	runes := []rune(s)
	length := I64(int64(len(runes)))
	if start.Negative() {
		start = I64(0)
	}
	if Cmp(end, length) > 0 {
		end = length
	}
	if Cmp(start, end) >= 0 {
		return ""
	}
	return string(runes[start.AsInt64():end.AsInt64()])
}

func TextReplace(pos Pos, s, old, new string) string {
	if old == "" {
		Fail(pos, "replace-empty", "text.replace old text is empty")
	}
	return strings.ReplaceAll(s, old, new)
}

func TextSplit(pos Pos, s, sep string) *List[string] {
	count := int64(1)
	if sep == "" {
		count = int64(utf8.RuneCountInString(s))
	} else {
		count = int64(strings.Count(s, sep)) + 1
	}
	if count > MaxElements {
		Fail(pos, "collection-limit", "text.split exceeds element limit")
	}
	parts := strings.Split(s, sep)
	return &List[string]{Items: parts}
}

func TextJoin(l *List[string], sep string) string {
	return strings.Join(l.Items, sep)
}

func TextToCodePoints(pos Pos, s string) *List[Int] {
	count := int64(utf8.RuneCountInString(s))
	if count > MaxElements {
		Fail(pos, "collection-limit", "text.to-code-points exceeds element limit")
	}
	items := make([]Int, 0, count)
	for _, r := range s {
		items = append(items, I64(int64(r)))
	}
	return &List[Int]{Items: items}
}

func TextFromCodePoints(pos Pos, l *List[Int]) string {
	var b strings.Builder
	for _, item := range l.Items {
		cp := item
		// Surrogate halves have no UTF-8 encoding; rune(cp) is safe past
		// these guards because cp already fits a valid code point.
		if cp.Negative() || !cp.Fits64() || cp.AsInt64() > 0x10FFFF || cp.AsInt64() >= 0xD800 && cp.AsInt64() <= 0xDFFF {
			Fail(pos, "text-code-point", "text.from-code-points code point %s is invalid", Str(cp))
		}
		b.WriteRune(rune(cp.AsInt64()))
	}
	return b.String()
}
