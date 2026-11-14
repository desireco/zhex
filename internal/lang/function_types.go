package lang

import "strings"

// splitTypeList splits only top-level commas, respecting generic and callable
// types. The greater-than sign in an arrow is not a generic closing bracket.
func splitTypeList(text string) ([]valueType, bool) {
	if strings.TrimSpace(text) == "" {
		return nil, true
	}
	var result []valueType
	angle, paren, start := 0, 0, 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<':
			angle++
		case '>':
			if i > 0 && text[i-1] == '-' {
				continue
			}
			angle--
			if angle < 0 {
				return nil, false
			}
		case '(':
			paren++
		case ')':
			paren--
			if paren < 0 {
				return nil, false
			}
		case ',':
			if angle == 0 && paren == 0 {
				part := strings.TrimSpace(text[start:i])
				if part == "" {
					return nil, false
				}
				result = append(result, valueType(part))
				start = i + 1
			}
		}
	}
	if angle != 0 || paren != 0 {
		return nil, false
	}
	part := strings.TrimSpace(text[start:])
	if part == "" {
		return nil, false
	}
	return append(result, valueType(part)), true
}

func functionTypeParts(typ valueType) ([]valueType, valueType, bool) {
	text := string(typ)
	if !strings.HasPrefix(text, "(") {
		return nil, "", false
	}
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, "", false
			}
			if depth == 0 {
				suffix := strings.TrimSpace(text[i+1:])
				if !strings.HasPrefix(suffix, "->") {
					return nil, "", false
				}
				result := strings.TrimSpace(suffix[2:])
				if result == "" {
					return nil, "", false
				}
				args, ok := splitTypeList(text[1:i])
				if !ok {
					return nil, "", false
				}
				return args, valueType(result), true
			}
		}
	}
	return nil, "", false
}

func functionType(args []valueType, result valueType) valueType {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = string(arg)
	}
	return valueType("(" + strings.Join(parts, ", ") + ") -> " + string(result))
}
