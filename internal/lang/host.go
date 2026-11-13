package lang

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (r *runtime) write(pos position, text string) error {
	n, err := io.WriteString(r.output, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return errorAt(pos, CodeConsoleOutput, "console output failed: %v", err)
	}
	if flusher, ok := r.output.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return errorAt(pos, CodeConsoleOutput, "console output failed: %v", err)
		}
	}
	return nil
}

func (r *runtime) builtin(x *expression, args []value) (value, error) {
	switch x.text {
	case "print":
		text := formatValue(args[0], x.inputType)
		return value{}, r.write(x.pos, text+"\n")
	case "int.to-string":
		return value{text: strconv.FormatInt(args[0].number, 10)}, nil
	case "text.reverse":
		runes := []rune(args[0].text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return value{text: string(runes)}, nil
	case "read-line":
		if err := r.write(x.pos, args[0].text); err != nil {
			return value{}, err
		}
		line, err := r.input.ReadString('\n')
		if err != nil && (err != io.EOF || len(line) == 0) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeConsoleInput, "console input failed: %v", err)}}, nil
		}
		if strings.HasSuffix(line, "\n") {
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
		}
		if !utf8.ValidString(line) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeConsoleUTF8, "console input is not valid UTF-8")}}, nil
		}
		return value{result: &inputResult{value: value{text: line}}}, nil
	case "text.to-int":
		number, err := strconv.ParseInt(strings.TrimSpace(args[0].text), 10, 64)
		if err != nil {
			return value{result: &inputResult{err: errorAt(x.pos, CodeTextToInt, "%q is not a signed 64-bit decimal integer", args[0].text)}}, nil
		}
		return value{result: &inputResult{value: value{number: number}}}, nil
	}
	return value{}, errorAt(x.pos, CodeUnsupportedCode, "unknown built-in operation %q", x.text)
}

func formatValue(v value, typ valueType) string {
	switch typ {
	case intType:
		return strconv.FormatInt(v.number, 10)
	case boolType:
		return strconv.FormatBool(v.boolean)
	case stringType:
		return v.text
	}
	panic(fmt.Sprintf("unchecked print type %s", typ))
}
