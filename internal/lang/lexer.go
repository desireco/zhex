package lang

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type token struct {
	kind, text string
	pos        position
}

func lex(file, source string) ([]token, error) {
	var tokens []token
	indent := 0
	lines := strings.Split(source, "\n")
	for lineIndex, line := range lines {
		pos := position{file, lineIndex + 1, 1}
		if !utf8.ValidString(line) {
			return nil, errorAt(pos, "source must be valid UTF-8")
		}
		line = strings.TrimSuffix(line, "\r")
		runes := []rune(line)
		i := 0
		for i < len(runes) && runes[i] == ' ' {
			i++
		}
		if i < len(runes) && runes[i] == '\t' {
			return nil, errorAt(position{file, lineIndex + 1, i + 1}, "tabs are not allowed outside strings")
		}
		if i == len(runes) || runes[i] == '#' {
			continue
		}
		if i%4 != 0 || i > indent+4 {
			return nil, errorAt(pos, "indentation must use four spaces per level")
		}
		if i > indent {
			tokens = append(tokens, token{"indent", "", pos})
		}
		for level := indent; level > i; level -= 4 {
			tokens = append(tokens, token{"dedent", "", pos})
		}
		indent = i
		for i < len(runes) {
			pos = position{file, lineIndex + 1, i + 1}
			c := runes[i]
			if c == ' ' {
				i++
				continue
			}
			if c == '#' {
				break
			}
			if c == '\t' {
				return nil, errorAt(pos, "tabs are not allowed outside strings")
			}
			start := i
			if nameStart(c) {
				i++
				for i < len(runes) {
					if namePart(runes[i]) {
						i++
						continue
					}
					if (runes[i] == '-' || runes[i] == '.') && i+1 < len(runes) && nameStart(runes[i+1]) {
						i += 2
						continue
					}
					break
				}
				tokens = append(tokens, token{"name", string(runes[start:i]), pos})
				continue
			}
			if c >= '0' && c <= '9' {
				i++
				for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
					i++
				}
				tokens = append(tokens, token{"number", string(runes[start:i]), pos})
				continue
			}
			if c == '"' {
				i++
				for i < len(runes) && runes[i] != '"' {
					if runes[i] == '\\' {
						i++
					}
					i++
				}
				if i >= len(runes) {
					return nil, errorAt(pos, "unterminated string")
				}
				i++
				decoded, err := strconv.Unquote(string(runes[start:i]))
				if err != nil || !utf8.ValidString(decoded) {
					return nil, errorAt(pos, "invalid string escape or UTF-8")
				}
				tokens = append(tokens, token{"string", decoded, pos})
				continue
			}
			if i+1 < len(runes) {
				pair := string(runes[i : i+2])
				if pair == "->" || pair == "==" || pair == "!=" || pair == "<=" || pair == ">=" {
					tokens = append(tokens, token{pair, pair, pos})
					i += 2
					continue
				}
			}
			if strings.ContainsRune("()+-*/%<>=:,", c) {
				tokens = append(tokens, token{string(c), string(c), pos})
				i++
				continue
			}
			return nil, errorAt(pos, "unexpected character %q", c)
		}
		tokens = append(tokens, token{"newline", "", position{file, lineIndex + 1, len(runes) + 1}})
	}
	end := position{file, len(lines), 1}
	for ; indent > 0; indent -= 4 {
		tokens = append(tokens, token{"dedent", "", end})
	}
	tokens = append(tokens, token{"eof", "", end})
	return tokens, nil
}

func nameStart(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func namePart(c rune) bool  { return nameStart(c) || c >= '0' && c <= '9' }
