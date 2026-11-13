package lang

import (
	"bufio"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// openFile opens path for reading, reporting failures as file-open.
func openFile(pos position, path string) (*fileValue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errorAt(pos, CodeFileOpen, "cannot open %q: %v", path, err)
	}
	return &fileValue{path: path, handle: f, reader: bufio.NewReader(f)}, nil
}

// readFile reads the whole file, validating UTF-8 at the boundary.
func readFile(pos position, path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errorAt(pos, CodeFileOpen, "cannot open %q: %v", path, err)
	}
	if !utf8.Valid(content) {
		return "", errorAt(pos, CodeFileUTF8, "file %q is not valid UTF-8", path)
	}
	return string(content), nil
}

// chomp drops the newline and a trailing carriage return, like read-line.
func chomp(line string) string {
	if strings.HasSuffix(line, "\n") {
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
	}
	return line
}

// chompedLines splits whole-file text into chomped lines for a Buffer.
func chompedLines(text string) []string {
	lines := strings.SplitAfter(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, line := range lines {
		lines[i] = chomp(line)
	}
	return lines
}

// nextLine returns the next chomped, UTF-8-validated line; ok is false at
// end of file, which the for-in loop treats as normal exhaustion.
func (f *fileValue) nextLine(pos position) (line string, ok bool, err error) {
	if f.closed {
		return "", false, errorAt(pos, CodeFileRead, "file %q is closed", f.path)
	}
	raw, rerr := f.reader.ReadString('\n')
	if rerr != nil && rerr != io.EOF {
		return "", false, errorAt(pos, CodeFileRead, "reading %q failed: %v", f.path, rerr)
	}
	if rerr == io.EOF && len(raw) == 0 {
		return "", false, nil
	}
	line = chomp(raw)
	if !utf8.ValidString(line) {
		return "", false, errorAt(pos, CodeFileUTF8, "file %q is not valid UTF-8", f.path)
	}
	return line, true, nil
}

// readLine reads one line as a Result; end of file fails inside the Result.
func (f *fileValue) readLine(pos position) value {
	line, ok, err := f.nextLine(pos)
	if err != nil {
		return value{result: &inputResult{err: err}}
	}
	if !ok {
		return value{result: &inputResult{err: errorAt(pos, CodeFileRead, "end of file")}}
	}
	return value{result: &inputResult{value: value{text: line}}}
}

// close releases the handle; closing again is a no-op.
func (f *fileValue) close() {
	if f.closed {
		return
	}
	f.closed = true
	f.handle.Close()
}
