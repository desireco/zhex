package lang

import (
	"fmt"
	"strings"
)

// suggestHint returns a did-you-mean line for the closest candidate, in the
// spirit of Elixir's UndefinedFunctionError and Ruby's did_you_mean.
func suggestHint(name string, candidates []string) string {
	if best, ok := closestName(name, candidates); ok {
		return fmt.Sprintf("did you mean %q?", best)
	}
	return ""
}

// suggestWordHint is suggestHint for word calls, plus one more rule: a bare
// name may spell a used module's importable word — gcd suggests math.gcd.
func suggestWordHint(name string, candidates []string) string {
	if hint := suggestHint(name, candidates); hint != "" {
		return hint
	}
	if strings.Contains(name, ".") {
		return ""
	}
	for _, candidate := range candidates {
		if _, word, _ := strings.Cut(candidate, "."); word == name {
			return fmt.Sprintf("did you mean %q?", candidate)
		}
	}
	return ""
}

// nameHint suggests the likely fix for a failed name lookup: the closest
// known name, else the hyphenated-name gotcha, since hyphens join one
// identifier and subtraction needs spaces.
func nameHint(name string, candidates []string) string {
	if hint := suggestHint(name, candidates); hint != "" {
		return hint
	}
	if strings.Contains(name, "-") {
		return fmt.Sprintf("%q is one name; hyphens join a single identifier. For subtraction write %q with spaces.", name, strings.ReplaceAll(name, "-", " - "))
	}
	return ""
}

// closestName returns the closest candidate within a length-aware edit
// distance, using the thresholds Ruby's did_you_mean popularized: one edit
// for short names, two otherwise.
func closestName(name string, candidates []string) (string, bool) {
	limit := 2
	if len(name) <= 3 {
		limit = 1
	}
	best, bestDistance := "", limit+1
	for _, candidate := range candidates {
		if d := editDistance(name, candidate); d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	return best, best != ""
}

// editDistance is the Levenshtein distance between two ASCII names.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

// tryHint explains the most common type mistake: a Result reaching a
// position that needs a plain value. Any Result<_,_> earns the hint.
func tryHint(types ...valueType) string {
	for _, typ := range types {
		if isResult(typ) {
			return `this value is a Result; add postfix try to unwrap it, as in text.to-int("N: ") try`
		}
	}
	return ""
}

// printHint names the concrete conversion for a printable-but-opaque type.
func printHint(typ valueType) string {
	if typ == bufferType {
		return "a Buffer holds lines; convert it with buffer.to-text first, as in print(buffer.to-text(b))"
	}
	return tryHint(typ)
}
