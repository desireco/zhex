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
	case "ok":
		return value{result: &inputResult{value: args[0]}}, nil
	case "err":
		return value{result: &inputResult{err: args[0].fault}}, nil
	case "error.new":
		d := errorAt(x.pos, Code(args[0].text), "%s", args[1].text).(*diagnostic)
		d.operation = "error.new"
		return value{fault: d}, nil
	case "error.report":
		r.report(args[0].fault, "handled", x.pos)
		return value{}, nil
	case "error.wrap":
		cause := args[2].fault.(*diagnostic)
		d := errorAt(cause.pos, Code(args[0].text), "%s", args[1].text).(*diagnostic)
		d.operation, d.cause = cause.operation, cause
		d.occurrence = ensureOccurrence(cause)
		return value{fault: d}, nil
	case "error.has-cause":
		return value{boolean: args[0].fault.(*diagnostic).cause != nil}, nil
	case "error.cause":
		cause := args[0].fault.(*diagnostic).cause
		if cause == nil {
			return value{result: &inputResult{err: errorAt(x.pos, CodeErrorNoCause, "error has no cause")}}, nil
		}
		return value{result: &inputResult{value: value{fault: cause}}}, nil
	case "error.code":
		return value{text: string(args[0].fault.(*diagnostic).code)}, nil
	case "error.message":
		return value{text: args[0].fault.(*diagnostic).message}, nil
	case "error.operation":
		return value{text: args[0].fault.(*diagnostic).operation}, nil
	case "error.file":
		return value{text: args[0].fault.(*diagnostic).pos.file}, nil
	case "error.line":
		return value{number: int64(args[0].fault.(*diagnostic).pos.line)}, nil
	case "error.column":
		return value{number: int64(args[0].fault.(*diagnostic).pos.column)}, nil
	case "error.detail":
		return value{text: args[0].fault.(*diagnostic).details[args[1].text]}, nil
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
		line = chomp(line)
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
	case "text.length":
		return value{number: int64(utf8.RuneCountInString(args[0].text))}, nil
	case "text.line-count":
		if args[0].text == "" {
			return value{}, nil
		}
		count := int64(strings.Count(args[0].text, "\n"))
		if !strings.HasSuffix(args[0].text, "\n") {
			count++
		}
		return value{number: count}, nil
	case "text.word-count":
		return value{number: int64(len(strings.Fields(args[0].text)))}, nil
	case "text.upper":
		return value{text: strings.ToUpper(args[0].text)}, nil
	case "text.lower":
		return value{text: strings.ToLower(args[0].text)}, nil
	case "text.trim":
		return value{text: strings.TrimSpace(args[0].text)}, nil
	case "text.contains":
		return value{boolean: strings.Contains(args[0].text, args[1].text)}, nil
	case "text.index-of":
		at := strings.Index(args[0].text, args[1].text)
		if at < 0 {
			return value{number: -1}, nil
		}
		return value{number: int64(utf8.RuneCountInString(args[0].text[:at]))}, nil
	case "text.slice":
		runes := []rune(args[0].text)
		start, end := args[1].number, args[2].number
		if start < 0 {
			start = 0
		}
		if end > int64(len(runes)) {
			end = int64(len(runes))
		}
		if start >= end {
			return value{}, nil
		}
		return value{text: string(runes[start:end])}, nil
	case "text.replace":
		if args[1].text == "" {
			return value{}, errorAt(x.pos, CodeReplaceEmpty, "text.replace old text is empty")
		}
		return value{text: strings.ReplaceAll(args[0].text, args[1].text, args[2].text)}, nil
	case "buffer.new":
		return value{buffer: &bufferValue{}}, nil
	case "buffer.add-line":
		args[0].buffer.lines = append(args[0].buffer.lines, args[1].text)
		return value{}, nil
	case "buffer.line-count":
		return value{number: int64(len(args[0].buffer.lines))}, nil
	case "buffer.line":
		i := args[1].number
		if i < 0 || i >= int64(len(args[0].buffer.lines)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeBufferRange, "buffer index %d is outside 0..%d", i, len(args[0].buffer.lines)-1)}}, nil
		}
		return value{result: &inputResult{value: value{text: args[0].buffer.lines[i]}}}, nil
	case "buffer.set-line":
		i := args[1].number
		if i < 0 || i >= int64(len(args[0].buffer.lines)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeBufferRange, "buffer index %d is outside 0..%d", i, len(args[0].buffer.lines)-1)}}, nil
		}
		args[0].buffer.lines[i] = args[2].text
		return value{result: &inputResult{}}, nil
	case "buffer.to-text":
		return value{text: strings.Join(args[0].buffer.lines, "\n")}, nil
	case "file.read-all":
		text, err := readFile(x.pos, args[0].text)
		if err != nil {
			return value{result: &inputResult{err: err}}, nil
		}
		return value{result: &inputResult{value: value{text: text}}}, nil
	case "file.read-lines":
		text, err := readFile(x.pos, args[0].text)
		if err != nil {
			return value{result: &inputResult{err: err}}, nil
		}
		return value{result: &inputResult{value: value{buffer: &bufferValue{lines: chompedLines(text)}}}}, nil
	case "file.open":
		f, err := openFile(x.pos, args[0].text)
		if err != nil {
			return value{result: &inputResult{err: err}}, nil
		}
		return value{result: &inputResult{value: value{file: f}}}, nil
	case "file.read-line":
		return args[0].file.readLine(x.pos), nil
	case "file.close":
		args[0].file.close()
		return value{}, nil
	case "console.exit":
		status := args[0].number
		if status < 0 || status > 255 {
			return value{}, errorAt(x.pos, CodeExitStatus, "console.exit status must be between 0 and 255, found %d", status)
		}
		return value{}, &ExitError{Status: int(status)}
	case "console.arg":
		i := args[0].number
		if i < 0 || i >= int64(len(r.args)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeArgumentMissing, "argument index %d is out of range; the program received %d", i, len(r.args))}}, nil
		}
		return value{result: &inputResult{value: value{text: r.args[i]}}}, nil
	case "console.args-count":
		return value{number: int64(len(r.args))}, nil
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
	case unitType:
		return ""
	case errorType:
		return v.fault.Error()
	case bufferType:
		return fmt.Sprintf("buffer(%d lines)", len(v.buffer.lines))
	case fileType:
		return fmt.Sprintf("file(%q)", v.file.path)
	}
	panic(fmt.Sprintf("unchecked print type %s", typ))
}
