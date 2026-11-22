package zhexrt

import (
	"math"
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

// The #196 text editing words. repeat and the pad pair validate their
// numeric arguments as bug signals (the replace-empty law); wrap, insert,
// and remove are total — wrap adopts pdf-doc's _wrap semantics verbatim
// and insert/remove clamp like TextSlice.

func TextRepeat(pos Pos, s string, n Int) string {
	if n.Negative() {
		Fail(pos, "text-repeat-count", "text.repeat count is negative")
	}
	// A count whose result no string could hold is out of range, not a build.
	if !n.Fits64() || len(s) > 0 && n.AsInt64() > int64(math.MaxInt)/int64(len(s)) {
		Fail(pos, "text-repeat-count", "text.repeat count %s is out of range", Str(n))
	}
	return strings.Repeat(s, int(n.AsInt64()))
}

// textPad validates the shared pad contract and returns the fill prefix:
// width at or below the text's code-point length pads nothing, and the
// fill is exactly one code point so any target width stays reachable.
func textPad(pos Pos, word, s string, width Int, fill string) string {
	if width.Negative() {
		Fail(pos, "text-pad-width", "%s width is negative", word)
	}
	if utf8.RuneCountInString(fill) != 1 {
		Fail(pos, "text-pad-fill", "%s fill must be exactly one code point", word)
	}
	length := int64(utf8.RuneCountInString(s))
	if !width.Fits64() {
		Fail(pos, "text-pad-width", "%s width %s is out of range", word, Str(width))
	}
	if w := width.AsInt64(); w > length {
		return strings.Repeat(fill, int(w-length))
	}
	return ""
}

func TextPadLeft(pos Pos, s string, width Int, fill string) string {
	return textPad(pos, "text.pad-left", s, width, fill) + s
}

func TextPadRight(pos Pos, s string, width Int, fill string) string {
	return s + textPad(pos, "text.pad-right", s, width, fill)
}

// TextWrap is the greedy word wrap pdf-doc's _wrap established: split on
// single spaces with empty fields skipped, a line breaks before the first
// word that no longer fits, an over-long word overflows alone, and text
// with no words wraps to one empty line. Width counts code points.
func TextWrap(pos Pos, s string, width Int) *List[string] {
	// Lines never outnumber words, so the space count bounds them before
	// anything materializes — the text.split limit.
	if int64(strings.Count(s, " "))+1 > MaxElements {
		Fail(pos, "collection-limit", "text.wrap exceeds element limit")
	}
	lines := []string{}
	cur := ""
	any := false
	for _, w := range strings.Split(s, " ") {
		if w == "" {
			continue
		}
		if !any {
			cur, any = w, true
			continue
		}
		if Cmp(I64(int64(utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w))), width) <= 0 {
			cur = cur + " " + w
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if any {
		lines = append(lines, cur)
	} else {
		lines = append(lines, "")
	}
	return &List[string]{Items: lines}
}

// TextInsert places inserted at the code-point index, 0 through the length
// inclusive (the length appends); an index outside clamps into range, the
// TextSlice law.
func TextInsert(s string, index Int, inserted string) string {
	runes := []rune(s)
	if index.Negative() {
		index = I64(0)
	}
	if length := I64(int64(len(runes))); Cmp(index, length) > 0 {
		index = length
	}
	at := index.AsInt64()
	return string(runes[:at]) + inserted + string(runes[at:])
}

// TextRemove is TextSlice's mirror image: the same clamped half-open
// range, removed instead of kept — an empty or inverted range is a no-op.
func TextRemove(s string, start, end Int) string {
	runes := []rune(s)
	length := I64(int64(len(runes)))
	if start.Negative() {
		start = I64(0)
	}
	if Cmp(end, length) > 0 {
		end = length
	}
	if Cmp(start, end) >= 0 {
		return s
	}
	return string(runes[:start.AsInt64()]) + string(runes[end.AsInt64():])
}
