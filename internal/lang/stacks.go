package lang

import (
	"strconv"
	"strings"
	"zhex/internal/zhexrt"
)

// stackValue is Zhex Stack<T>: a shared-mutable LIFO whose top is the end
// of the backing store. There are no traversal words in v1, so there is no
// iterating guard to keep.
type stackValue struct {
	items []value
}

// renderStack shows a stack bottom-to-top in square brackets — the list
// literal's shape — so the last element shown is the top. String members
// keep quotes so the element type stays visible, the set convention.
func renderStack(typ valueType, s *stackValue) string {
	parts := make([]string, len(s.items))
	for i, v := range s.items {
		switch typ {
		case "Stack<String>":
			parts[i] = strconv.Quote(v.text)
		case "Stack<Bool>":
			parts[i] = strconv.FormatBool(v.boolean)
		default:
			parts[i] = zhexrt.Str(v.number)
		}
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
