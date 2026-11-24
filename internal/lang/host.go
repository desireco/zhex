package lang

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"zhex/internal/zhexrt"
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

// builtin dispatches the built-in words. The pure text/int words call the
// zhexrt implementations the native target shares (the M3 extraction);
// operations over dynamic values (collections, buffers, files) or with
// injectable failure seams stay here, byte-equal by the differential
// gates.
func (r *runtime) builtin(x *expression, args []value) (value, error) {
	if err := wordBoundary(x); err != nil {
		return value{result: &inputResult{err: err}}, nil
	}
	switch x.text {
	case "date.new", "date.parse", "date.to-string", "date.add-days", "time.new", "time.parse", "time.to-string", "duration.new", "duration.parse", "duration.to-string":
		return temporalBuiltin(x, args)
	case "instant.now", "instant.parse", "instant.to-string", "instant.add-duration", "timezone.load", "timezone.name", "zoned.new", "zoned.to-string", "zoned.instant", "zoned.in-zone", "zoned.add-days", "zoned.add-duration":
		return temporalBuiltin(x, args)
	case "json.decode":
		return decodeJSON(x.pos, args[0].text, args[1], x.jsonSchema), nil
	case "json.encode":
		if x.jsonSchema == nil {
			return encodeJSONTree(x.pos, args[0]), nil
		}
		return encodeJSON(x.pos, args[0], x.jsonSchema), nil
	case "json.parse":
		return parseJSONTree(x.pos, args[0].text), nil
	case "xml.decode":
		return decodeXML(x.pos, args[0].text, args[1], x.jsonSchema), nil
	case "map.length":
		return value{number: zhexrt.I64(int64(len(args[0].mapping.entries)))}, nil
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
		return args[0], nil
	case "map.into":
		if err := args[1].mapping.guard(x.pos, "map.into"); err != nil {
			return value{}, err
		}
		args[1].mapping.merge(args[0].mapping)
		return args[1], nil
	case "map.keys", "map.values":
		// A read, so the traversal guard does not apply: extraction completes
		// before the fresh list escapes, the file.list shape.
		m := args[0].mapping
		if int64(len(m.entries)) > maxCollectionElements {
			return value{}, errorAt(x.pos, CodeCollectionLimit, "%s exceeds element limit", x.text)
		}
		items := make([]value, len(m.entries))
		for i, entry := range m.entries {
			if x.text == "map.keys" {
				items[i] = m.keyValue(entry.key)
			} else {
				items[i] = entry.value
			}
		}
		return value{list: &listValue{items: items}}, nil
	case "array.new":
		return arrayNew(x.pos, args[0].list, args[1]), nil
	case "array.resize":
		return arrayResize(x.pos, args[0].array, args[1].list, args[2]), nil
	case "array.shape":
		items := make([]value, len(args[0].array.shape))
		for i, n := range args[0].array.shape {
			items[i] = value{number: zhexrt.I64(n)}
		}
		return value{list: &listValue{items: items}}, nil
	case "array.rank":
		return value{number: zhexrt.I64(int64(len(args[0].array.shape)))}, nil
	case "array.length":
		return value{number: zhexrt.I64(int64(len(args[0].array.items)))}, nil
	case "array.select":
		source := args[0].array
		if len(source.shape) != 2 {
			return value{}, errorAt(x.pos, CodeArrayRank, "array.select requires a rank-2 array, found rank %d", len(source.shape))
		}
		source.active++
		defer func() { source.active-- }()
		width := source.shape[1]
		kept := make([]value, 0, len(source.items))
		rows := 0
		for row := int64(0); row < source.shape[0]; row++ {
			// Each row is a fresh list; sharing the backing store would let an
			// append inside the predicate overwrite the source array.
			items := append([]value(nil), source.items[row*width:(row+1)*width]...)
			keep, err := r.callFunction(args[1].function, x.pos, value{list: &listValue{items: items}})
			if err != nil {
				return value{}, err
			}
			if keep.boolean {
				kept = append(kept, items...)
				rows++
			}
		}
		return value{array: &arrayValue{shape: []int64{int64(rows), width}, items: kept}}, nil
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
		return value{result: &inputResult{value: args[0]}}, nil
	case "list.insert":
		if args[0].list.active > 0 {
			return failedCollection(x.pos, CodeCollectionIterating, "list.insert", "collection is being iterated", nil), nil
		}
		if len(args[0].list.items) >= maxCollectionElements {
			return failedCollection(x.pos, CodeCollectionLimit, "list.insert", "list exceeds element limit", nil), nil
		}
		index := args[1].number
		if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() > int64(len(args[0].list.items)) {
			return failedCollection(x.pos, CodeListRange, "list.insert", "list insert index is out of range", map[string]string{"index": zhexrt.Str(index), "length": strconv.Itoa(len(args[0].list.items))}), nil
		}
		at := index.AsInt64()
		rest := append([]value{args[2]}, args[0].list.items[at:]...)
		args[0].list.items = append(args[0].list.items[:at], rest...)
		return value{result: &inputResult{value: args[0]}}, nil
	case "list.pop":
		// the sanctioned exception: the removed element comes back, not the
		// list. The empty failure keeps the family's diagnostic — the same
		// shape xs[list.length(xs) - 1] produced, index -1 against length 0.
		if args[0].list.active > 0 {
			return failedCollection(x.pos, CodeCollectionIterating, "list.pop", "collection is being iterated", nil), nil
		}
		if len(args[0].list.items) == 0 {
			return failedCollection(x.pos, CodeListRange, "list.pop", "list index is out of range", map[string]string{"index": "-1", "length": "0"}), nil
		}
		last := args[0].list.items[len(args[0].list.items)-1]
		args[0].list.items = args[0].list.items[:len(args[0].list.items)-1]
		return value{result: &inputResult{value: last}}, nil
	case "list.first":
		if len(args[0].list.items) == 0 {
			return failedCollection(x.pos, CodeListRange, "list.first", "list index is out of range", map[string]string{"index": "0", "length": "0"}), nil
		}
		return value{result: &inputResult{value: args[0].list.items[0]}}, nil
	case "list.last":
		if len(args[0].list.items) == 0 {
			return failedCollection(x.pos, CodeListRange, "list.last", "list index is out of range", map[string]string{"index": "-1", "length": "0"}), nil
		}
		return value{result: &inputResult{value: args[0].list.items[len(args[0].list.items)-1]}}, nil
	case "list.length":
		return value{number: zhexrt.I64(int64(len(args[0].list.items)))}, nil
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
	case "list.filter", "list.select":
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
	case "list.contains":
		source := args[0].list
		source.active++
		defer func() { source.active-- }()
		_, inner, _ := collectionInner(x.inputType)
		for _, item := range source.items {
			if inner == stringType && item.text == args[1].text || inner != stringType && item.number == args[1].number {
				return value{boolean: true}, nil
			}
		}
		return value{boolean: false}, nil
	case "list.sort":
		_, inner, _ := collectionInner(x.inputType)
		items := args[0].list.items
		return value{list: &listValue{items: sortValuesStable(items, inner == stringType)}}, nil
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
		return value{number: zhexrt.I64(int64(args[0].fault.(*diagnostic).pos.line))}, nil
	case "error.column":
		return value{number: zhexrt.I64(int64(args[0].fault.(*diagnostic).pos.column))}, nil
	case "error.detail":
		return value{text: args[0].fault.(*diagnostic).details[args[1].text]}, nil
	case "print":
		text := formatValue(args[0], x.inputType)
		return value{}, r.write(x.pos, text+"\n")
	case "int.to-string":
		return value{text: zhexrt.Str(args[0].number)}, nil
	case "int.mod-floor":
		n, modulus := args[0].number, args[1].number
		if modulus.IsZero() {
			return value{}, errorAt(x.pos, CodeDivisionZero, "int.mod-floor modulus is zero")
		}
		_, wrapped := zhexrt.DivMod(n, modulus)
		// Floored semantics: a nonzero result takes the modulus's sign, so
		// /'s truncation needs one adjustment; |wrapped| < |modulus| keeps
		// the sum in Int range.
		if !wrapped.IsZero() && wrapped.Negative() != modulus.Negative() {
			wrapped, _ = zhexrt.AddOvf(wrapped, modulus)
		}
		return value{number: wrapped}, nil
	case "text.reverse":
		return value{text: zhexrt.TextReverse(args[0].text)}, nil
	case "read-line":
		if err := r.write(x.pos, args[0].text); err != nil {
			return value{}, err
		}
		line, err := r.input.ReadString('\n')
		if err == io.EOF && len(line) == 0 {
			e := errorAt(x.pos, CodeConsoleInput, "end of input").(*diagnostic)
			e.details = map[string]string{"reason": "end-of-file"}
			return value{result: &inputResult{err: e}}, nil
		}
		if err != nil && err != io.EOF {
			e := errorAt(x.pos, CodeConsoleInput, "console input failed: %v", err).(*diagnostic)
			e.details = map[string]string{"reason": "read-failed"}
			return value{result: &inputResult{err: e}}, nil
		}
		line = zhexrt.Chomp(line)
		if !utf8.ValidString(line) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeConsoleUTF8, "console input is not valid UTF-8")}}, nil
		}
		return value{result: &inputResult{value: value{text: line}}}, nil
	case "text.to-int":
		number, ok := zhexrt.Parse(args[0].text)
		if !ok {
			return value{result: &inputResult{err: errorAt(x.pos, CodeTextToInt, "%q is not a signed 128-bit decimal integer", args[0].text)}}, nil
		}
		return value{result: &inputResult{value: value{number: number}}}, nil
	case "text.length":
		return value{number: zhexrt.TextLength(args[0].text)}, nil
	case "text.line-count":
		return value{number: zhexrt.TextLineCount(args[0].text)}, nil
	case "text.word-count":
		return value{number: zhexrt.TextWordCount(args[0].text)}, nil
	case "text.upper":
		return value{text: zhexrt.TextUpper(args[0].text)}, nil
	case "text.lower":
		return value{text: zhexrt.TextLower(args[0].text)}, nil
	case "text.trim":
		return value{text: zhexrt.TextTrim(args[0].text)}, nil
	case "text.contains":
		return value{boolean: zhexrt.TextContains(args[0].text, args[1].text)}, nil
	case "text.index-of":
		return value{number: zhexrt.TextIndexOf(args[0].text, args[1].text)}, nil
	case "text.slice":
		return value{text: zhexrt.TextSlice(rtPos(x.pos), args[0].text, args[1].number, args[2].number)}, nil
	case "text.replace":
		replaced, err := rtFail(func() string { return zhexrt.TextReplace(rtPos(x.pos), args[0].text, args[1].text, args[2].text) })
		if err != nil {
			return value{}, err
		}
		return value{text: replaced}, nil
	case "text.split":
		parts, err := rtFail(func() *zhexrt.List[string] { return zhexrt.TextSplit(rtPos(x.pos), args[0].text, args[1].text) })
		if err != nil {
			return value{}, err
		}
		items := make([]value, len(parts.Items))
		for i, part := range parts.Items {
			items[i] = value{text: part}
		}
		return value{list: &listValue{items: items}}, nil
	case "text.join":
		parts := make([]string, len(args[0].list.items))
		for i, item := range args[0].list.items {
			parts[i] = item.text
		}
		return value{text: zhexrt.TextJoin(&zhexrt.List[string]{Items: parts}, args[1].text)}, nil
	case "text.to-code-points":
		points, err := rtFail(func() *zhexrt.List[zhexrt.Int] { return zhexrt.TextToCodePoints(rtPos(x.pos), args[0].text) })
		if err != nil {
			return value{}, err
		}
		items := make([]value, len(points.Items))
		for i, cp := range points.Items {
			items[i] = value{number: cp}
		}
		return value{list: &listValue{items: items}}, nil
	case "text.from-code-points":
		codes := make([]zhexrt.Int, len(args[0].list.items))
		for i, item := range args[0].list.items {
			codes[i] = item.number
		}
		text, err := rtFail(func() string { return zhexrt.TextFromCodePoints(rtPos(x.pos), &zhexrt.List[zhexrt.Int]{Items: codes}) })
		if err != nil {
			return value{}, err
		}
		return value{text: text}, nil
	case "text.starts-with":
		return value{boolean: zhexrt.TextStartsWith(args[0].text, args[1].text)}, nil
	case "text.ends-with":
		return value{boolean: zhexrt.TextEndsWith(args[0].text, args[1].text)}, nil
	case "text.repeat":
		text, err := rtFail(func() string { return zhexrt.TextRepeat(rtPos(x.pos), args[0].text, args[1].number) })
		if err != nil {
			return value{}, err
		}
		return value{text: text}, nil
	case "text.pad-left", "text.pad-right":
		pad := func(fn func(zhexrt.Pos, string, zhexrt.Int, string) string) (string, error) {
			return rtFail(func() string { return fn(rtPos(x.pos), args[0].text, args[1].number, args[2].text) })
		}
		var padded string
		var err error
		if x.text == "text.pad-left" {
			padded, err = pad(zhexrt.TextPadLeft)
		} else {
			padded, err = pad(zhexrt.TextPadRight)
		}
		if err != nil {
			return value{}, err
		}
		return value{text: padded}, nil
	case "text.wrap":
		// The pre-check mirrors zhexrt.TextWrap's so the shared body
		// cannot panic here; past it, the greedy law lives in one place.
		if int64(strings.Count(args[0].text, " "))+1 > maxCollectionElements {
			return value{}, errorAt(x.pos, CodeCollectionLimit, "text.wrap exceeds element limit")
		}
		wrapped := zhexrt.TextWrap(rtPos(x.pos), args[0].text, args[1].number)
		items := make([]value, len(wrapped.Items))
		for i, line := range wrapped.Items {
			items[i] = value{text: line}
		}
		return value{list: &listValue{items: items}}, nil
	case "text.insert":
		return value{text: zhexrt.TextInsert(args[0].text, args[1].number, args[2].text)}, nil
	case "text.remove":
		return value{text: zhexrt.TextRemove(args[0].text, args[1].number, args[2].number)}, nil
	case "text.sentences":
		// The pre-check mirrors zhexrt.TextSentences' so the shared body
		// cannot panic here; past it, the boundary law lives in one place.
		if int64(zhexrt.SentenceCount(args[0].text)) > maxCollectionElements {
			return value{}, errorAt(x.pos, CodeCollectionLimit, "text.sentences exceeds element limit")
		}
		sentences := zhexrt.TextSentences(rtPos(x.pos), args[0].text)
		items := make([]value, len(sentences.Items))
		for i, sentence := range sentences.Items {
			items[i] = value{text: sentence}
		}
		return value{list: &listValue{items: items}}, nil
	case "buffer.new":
		return value{buffer: &bufferValue{}}, nil
	case "buffer.add-line":
		args[0].buffer.lines = append(args[0].buffer.lines, args[1].text)
		return args[0], nil
	case "buffer.line-count":
		return value{number: zhexrt.I64(int64(len(args[0].buffer.lines)))}, nil
	case "buffer.line":
		i := args[1].number
		if !i.Fits64() || i.AsInt64() < 0 || i.AsInt64() >= int64(len(args[0].buffer.lines)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeBufferRange, "buffer index %s is outside 0..%d", zhexrt.Str(i), len(args[0].buffer.lines)-1)}}, nil
		}
		return value{result: &inputResult{value: value{text: args[0].buffer.lines[i.AsInt64()]}}}, nil
	case "buffer.set-line":
		i := args[1].number
		if !i.Fits64() || i.AsInt64() < 0 || i.AsInt64() >= int64(len(args[0].buffer.lines)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeBufferRange, "buffer index %s is outside 0..%d", zhexrt.Str(i), len(args[0].buffer.lines)-1)}}, nil
		}
		args[0].buffer.lines[i.AsInt64()] = args[2].text
		return value{result: &inputResult{value: args[0]}}, nil
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
	case "file.write-bytes":
		return writeBytesFile(x.pos, args[0].text, args[1].list.items), nil
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
		return value{result: &inputResult{value: value{number: zhexrt.I64(size)}}}, nil
	case "file.free-space":
		path := args[0].text
		avail, err := osFreeSpace(path)
		if err != nil {
			return value{result: &inputResult{err: inspectError(x.pos, CodeFileFreeSpace, "cannot get the free space for %q: %v", path, err)}}, nil
		}
		return value{result: &inputResult{value: value{number: zhexrt.I64(avail)}}}, nil
	case "file.join":
		return value{text: joinPath(args[0].text, args[1].text)}, nil
	case "file.base":
		return value{text: filepath.Base(args[0].text)}, nil
	case "file.extension":
		return value{text: filepath.Ext(args[0].text)}, nil
	case "file.directory-of":
		return value{text: filepath.Dir(args[0].text)}, nil
	case "file.make-folder":
		return createFolder(x.pos, args[0].text, args[1].boolean), nil
	case "file.delete":
		return deletePath(x.pos, args[0].text), nil
	case "file.delete-folder":
		return deleteFolder(x.pos, args[0].text, args[1].boolean), nil
	case "file.move":
		return movePath(x.pos, args[0].text, args[1].text, args[2].boolean), nil
	case "console.fail":
		_, _ = io.WriteString(r.diagnostics, args[0].text+"\n")
		return value{}, &ExitError{Status: 1}
	case "console.exit":
		status := args[0].number
		if status.Negative() || zhexrt.Cmp(status, zhexrt.I64(255)) > 0 {
			return value{}, errorAt(x.pos, CodeExitStatus, "console.exit status must be between 0 and 255, found %s", zhexrt.Str(status))
		}
		return value{}, &ExitError{Status: int(status.AsInt64())}
	case "console.arg":
		i := args[0].number
		if !i.Fits64() || i.AsInt64() < 0 || i.AsInt64() >= int64(len(r.args)) {
			return value{result: &inputResult{err: errorAt(x.pos, CodeArgumentMissing, "argument index %s is out of range; the program received %d", zhexrt.Str(i), len(r.args))}}, nil
		}
		return value{result: &inputResult{value: value{text: r.args[i.AsInt64()]}}}, nil
	case "console.args-count":
		return value{number: zhexrt.I64(int64(len(r.args)))}, nil
	case "env.get":
		if text, ok := os.LookupEnv(args[0].text); ok {
			return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{{text: text}}}}, nil
		}
		return value{choice: &choiceValue{name: "Option.None", variant: 0}}, nil
	case "env.load":
		loaded := zhexrt.EnvLoad(rtPos(x.pos), args[0].text)
		if loaded.Err != nil {
			return value{result: &inputResult{err: fromRT(loaded.Err)}}, nil
		}
		return value{result: &inputResult{}}, nil
	}
	return value{}, errorAt(x.pos, CodeUnsupportedCode, "unknown built-in operation %q", x.text)
}

func formatValue(v value, typ valueType) string {
	if v.function != nil {
		return fmt.Sprintf("function(%s, %d remaining)", v.function.target.text, len(v.function.holes))
	}
	switch typ {
	case intType:
		return zhexrt.Str(v.number)
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
		return time.Duration(v.number.AsInt64()).String()
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
