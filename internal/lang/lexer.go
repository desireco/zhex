package lang

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type token struct {
	kind, text string
	pos        position
	end        position // exclusive, one past the last rune; unset for indent/dedent/newline/eof
}

func lex(file, source string) ([]token, error) {
	var tokens []token
	type delimiter struct {
		open rune
		pos  position
	}
	var delimiters []delimiter
	indent := 0
	lines := strings.Split(source, "\n")
	for lineIndex, line := range lines {
		pos := position{file: file, line: lineIndex + 1, column: 1}
		if !utf8.ValidString(line) {
			return nil, errorAt(pos, CodeSourceUTF8, "source must be valid UTF-8")
		}
		line = strings.TrimSuffix(line, "\r")
		runes := []rune(line)
		i := 0
		for i < len(runes) && runes[i] == ' ' {
			i++
		}
		if i < len(runes) && runes[i] == '\t' {
			return nil, errorAt(position{file: file, line: lineIndex + 1, column: i + 1}, CodeTabCharacter, "tabs are not allowed outside strings")
		}
		if i == len(runes) || runes[i] == '#' {
			continue
		}
		if len(delimiters) == 0 {
			if i%4 != 0 || i > indent+4 {
				return nil, errorAt(pos, CodeIndentation, "indentation must use four spaces per level")
			}
			if i > indent {
				tokens = append(tokens, token{kind: "indent", pos: pos})
			}
			for level := indent; level > i; level -= 4 {
				tokens = append(tokens, token{kind: "dedent", pos: pos})
			}
			indent = i
		}
		for i < len(runes) {
			pos = position{file: file, line: lineIndex + 1, column: i + 1}
			c := runes[i]
			if c == ' ' {
				i++
				continue
			}
			if c == '#' {
				break
			}
			if c == '\t' {
				return nil, errorAt(pos, CodeTabCharacter, "tabs are not allowed outside strings")
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
				tokens = append(tokens, token{"name", string(runes[start:i]), pos, position{file: file, line: lineIndex + 1, column: i + 1}})
				continue
			}
			if c >= '0' && c <= '9' {
				i++
				for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
					i++
				}
				tokens = append(tokens, token{"number", string(runes[start:i]), pos, position{file: file, line: lineIndex + 1, column: i + 1}})
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
					return nil, errorAt(pos, CodeUnterminatedString, "unterminated string")
				}
				i++
				decoded, err := strconv.Unquote(string(runes[start:i]))
				if err != nil || !utf8.ValidString(decoded) {
					return nil, errorAt(pos, CodeStringEscape, "invalid string escape or UTF-8")
				}
				tokens = append(tokens, token{"string", decoded, pos, position{file: file, line: lineIndex + 1, column: i + 1}})
				continue
			}
			if i+1 < len(runes) {
				pair := string(runes[i : i+2])
				if pair == "->" || pair == "==" || pair == "!=" || pair == "<=" || pair == ">=" {
					i += 2
					end := position{file: file, line: lineIndex + 1, column: i + 1}
					tokens = append(tokens, token{kind: pair, text: pair, pos: pos, end: end})
					continue
				}
			}
			if strings.ContainsRune("()[]{}+-*/%<>=:,", c) {
				if strings.ContainsRune("([{", c) {
					if len(delimiters) >= 256 {
						return nil, errorAt(pos, CodeNestingLimit, "delimiter nesting limit exceeded")
					}
					delimiters = append(delimiters, delimiter{open: c, pos: pos})
				} else if strings.ContainsRune(")]}", c) && len(delimiters) > 0 {
					last := delimiters[len(delimiters)-1]
					expected := map[rune]rune{'(': ')', '[': ']', '{': '}'}[last.open]
					if c != expected {
						return nil, errorAt(pos, CodeExpectedToken, "expected %s, found %q", string(expected), string(c))
					}
					delimiters = delimiters[:len(delimiters)-1]
				}
				i++
				end := position{file: file, line: lineIndex + 1, column: i + 1}
				tokens = append(tokens, token{kind: string(c), text: string(c), pos: pos, end: end})
				continue
			}
			return nil, errorAt(pos, CodeUnexpectedCharacter, "unexpected character %q", c)
		}
		if len(delimiters) == 0 {
			tokens = append(tokens, token{kind: "newline", pos: position{file: file, line: lineIndex + 1, column: len(runes) + 1}})
		}
	}
	if len(delimiters) > 0 {
		last := delimiters[len(delimiters)-1]
		return nil, errorAt(last.pos, CodeUnclosedDelimiter, "unclosed delimiter %q", string(last.open))
	}
	end := position{file: file, line: len(lines), column: 1}
	for ; indent > 0; indent -= 4 {
		tokens = append(tokens, token{kind: "dedent", pos: end})
	}
	tokens = append(tokens, token{kind: "eof", pos: end})
	return tokens, nil
}

func nameStart(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
func namePart(c rune) bool  { return nameStart(c) || c >= '0' && c <= '9' }
