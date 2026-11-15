package lang

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
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
	case "date.new", "date.parse", "date.to-string", "date.add-days", "time.new", "time.parse", "time.to-string", "duration.new", "duration.parse", "duration.to-string":
		return temporalBuiltin(x, args)
	case "instant.now", "instant.parse", "instant.to-string", "instant.add-duration", "timezone.load", "timezone.name", "zoned.new", "zoned.to-string", "zoned.instant", "zoned.in-zone", "zoned.add-days", "zoned.add-duration":
		return temporalBuiltin(x, args)
	case "json.decode":
		return decodeJSON(x.pos, args[0].text, args[1], x.jsonSchema), nil
	case "json.encode":
		return encodeJSON(x.pos, args[0], x.jsonSchema), nil
	case "map.length":
		return value{number: int64(len(args[0].mapping.entries))}, nil
	case "map.contains":
		_, ok := args[0].mapping.index[args[0].mapping.key(args[1])]
		return value{boolean: ok}, nil
	case "map.put":
		if err := args[0].mapping.guard(x.pos, "map.put"); err != nil {
			return value{}, err
		}
		args[0].mapping.put(args[1], args[2])
		return args[0], nil
	case "map.remove":
		if err := args[0].mapping.guard(x.pos, "map.remove"); err != nil {
			return value{}, err
		}
		args[0].mapping.remove(args[1])
		return value{}, nil
	case "map.into":
		if err := args[1].mapping.guard(x.pos, "map.into"); err != nil {
			return value{}, err
		}
		args[1].mapping.merge(args[0].mapping)
		return args[1], nil
	case "array.new":
		return arrayNew(x.pos, args[0].list, args[1]), nil
	case "array.resize":
		return arrayResize(x.pos, args[0].array, args[1].list, args[2]), nil
	case "array.shape":
		items := make([]value, len(args[0].array.shape))
		for i, n := range args[0].array.shape {
			items[i] = value{number: n}
		}
		return value{list: &listValue{items: items}}, nil
	case "array.rank":
		return value{number: int64(len(args[0].array.shape))}, nil
	case "array.length":
		return value{number: int64(len(args[0].array.items))}, nil
	case "list.new":
		return value{list: &listValue{}}, nil
	case "list.append":
		if args[0].list.active > 0 {
			return failedCollection(x.pos, CodeCollectionIterating, "list.append", "collection is being iterated", nil), nil
		}
		if len(args[0].list.items) >= maxCollectionElements {
			return failedCollection(x.pos, CodeCollectionLimit, "list.append", "list exceeds element limit", nil), nil
		}
		args[0].list.items = append(args[0].list.items, args[1])
		return value{result: &inputResult{}}, nil
	case "list.length":
		return value{number: int64(len(args[0].list.items))}, nil
	case "list.map":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		items := make([]value, 0, len(source.items))
		for _, item := range source.items {
			mapped, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			items = append(items, mapped)
		}
		return value{list: &listValue{items: items}}, nil
	case "list.filter":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		items := make([]value, 0, len(source.items))
		for _, item := range source.items {
			keep, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			if keep.boolean {
				items = append(items, item)
			}
		}
		return value{list: &listValue{items: items}}, nil
	case "list.fold":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		accumulator := args[1]
		for _, item := range source.items {
			next, err := r.callFunction(args[2].function, x.pos, accumulator, item)
			if err != nil {
				return value{}, err
			}
			accumulator = next
		}
		return accumulator, nil
	case "list.find":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		for _, item := range source.items {
			keep, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			if keep.boolean {
				return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{item}}}, nil
			}
		}
		return value{choice: &choiceValue{name: "Option.None", variant: 0}}, nil
	case "list.any":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		for _, item := range source.items {
			keep, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			if keep.boolean {
				return value{boolean: true}, nil
			}
		}
		return value{boolean: false}, nil
	case "list.all":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		for _, item := range source.items {
			keep, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			if !keep.boolean {
				return value{boolean: false}, nil
			}
		}
		return value{boolean: true}, nil
	case "list.sort":
		_, inner, _ := collectionInner(x.inputType)
		items := args[0].list.items
		return value{list: &listValue{items: sortListStable(items, items, inner == stringType)}}, nil
	case "list.sort-by":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		keys := make([]value, len(source.items))
		for i, item := range source.items {
			key, err := r.callFunction(args[1].function, x.pos, item)
			if err != nil {
				return value{}, err
			}
			keys[i] = key
		}
		_, keyResult, _ := functionTypeParts(x.args[1].typ)
		return value{list: &listValue{items: sortListStable(source.items, keys, keyResult == stringType)}}, nil
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
	case "text.split":
		count := int64(1)
		if args[1].text == "" {
			count = int64(utf8.RuneCountInString(args[0].text))
		} else {
			count = int64(strings.Count(args[0].text, args[1].text)) + 1
		}
		if count > maxCollectionElements {
			return value{}, errorAt(x.pos, CodeCollectionLimit, "text.split exceeds element limit")
		}
		parts := strings.Split(args[0].text, args[1].text)
		items := make([]value, len(parts))
		for i, part := range parts {
			items[i] = value{text: part}
		}
		return value{list: &listValue{items: items}}, nil
	case "text.join":
		parts := make([]string, len(args[0].list.items))
		for i, item := range args[0].list.items {
			parts[i] = item.text
		}
		return value{text: strings.Join(parts, args[1].text)}, nil
	case "text.starts-with":
		return value{boolean: strings.HasPrefix(args[0].text, args[1].text)}, nil
	case "text.ends-with":
		return value{boolean: strings.HasSuffix(args[0].text, args[1].text)}, nil
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
	case "file.read":
		text, err := readFile(x.pos, args[0].text)
		if err != nil {
			return value{result: &inputResult{err: err}}, nil
		}
		return value{result: &inputResult{value: value{text: text}}}, nil
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
	case "file.write":
		return writeTextFile(x.pos, args[0].text, args[1].text, args[2].boolean), nil
	case "file.list":
		folder := args[0].text
		entries, err := osReadDir(folder)
		if err != nil {
			return value{result: &inputResult{err: inspectError(x.pos, CodeFileList, "cannot list %q: %v", folder, err)}}, nil
		}
		if len(entries) > maxCollectionElements {
			return value{result: &inputResult{err: errorAt(x.pos, CodeCollectionLimit, "file.list exceeds element limit")}}, nil
		}
		items := make([]value, len(entries))
		for i, entry := range entries {
			items[i] = value{text: entry.Name()}
		}
		return value{result: &inputResult{value: value{list: &listValue{items: items}}}}, nil
	case "file.exists":
		if _, err := osStat(args[0].text); err != nil {
			if os.IsNotExist(err) {
				return value{result: &inputResult{value: value{boolean: false}}}, nil
			}
			return value{result: &inputResult{err: inspectError(x.pos, CodeFileExists, "cannot check %q: %v", args[0].text, err)}}, nil
		}
		return value{result: &inputResult{value: value{boolean: true}}}, nil
	case "file.size":
		path := args[0].text
		info, err := osStat(path)
		if err != nil {
			return value{result: &inputResult{err: inspectError(x.pos, CodeFileSize, "cannot get the size of %q: %v", path, err)}}, nil
		}
		var size int64
		if !info.IsDir() {
			size = int64(info.Size())
		}
		return value{result: &inputResult{value: value{number: size}}}, nil
	case "file.free-space":
		path := args[0].text
		avail, err := osFreeSpace(path)
		if err != nil {
			return value{result: &inputResult{err: inspectError(x.pos, CodeFileFreeSpace, "cannot get the free space for %q: %v", path, err)}}, nil
		}
		return value{result: &inputResult{value: value{number: avail}}}, nil
	case "console.fail":
		_, _ = io.WriteString(r.diagnostics, args[0].text+"\n")
		return value{}, &ExitError{Status: 1}
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
	if v.function != nil {
		return fmt.Sprintf("function(%s, %d remaining)", v.function.target.text, len(v.function.holes))
	}
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
	case dateType:
		return v.moment.Format("2006-01-02")
	case timeType:
		return v.moment.Format("15:04:05.999999999")
	case durationType:
		return time.Duration(v.number).String()
	case instantType:
		return v.moment.UTC().Format(time.RFC3339Nano)
	case timezoneType:
		return v.zone.String()
	case zonedType:
		return formatZoned(v.moment)
	}
	if v.mapping != nil {
		return fmt.Sprintf("map(%d entries)", len(v.mapping.entries))
	}
	if v.array != nil {
		return fmt.Sprintf("array(%d elements)", len(v.array.items))
	}
	if v.list != nil {
		return fmt.Sprintf("list(%d elements)", len(v.list.items))
	}
	if v.choice != nil {
		return fmt.Sprintf("choice(%s)", v.choice.name)
	}
	if v.record != nil {
		return fmt.Sprintf("record(%s)", v.record.name)
	}
	panic(fmt.Sprintf("unchecked print type %s", typ))
}
