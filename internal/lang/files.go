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
		return nil, fileOpenError(pos, path, err)
	}
	return &fileValue{path: path, handle: f, reader: bufio.NewReader(f)}, nil
}

func fileOpenError(pos position, path string, cause error) error {
	reason := "other"
	if os.IsNotExist(cause) {
		reason = "not-found"
	} else if os.IsPermission(cause) {
		reason = "permission-denied"
	}
	d := errorAt(pos, CodeFileOpen, "cannot open %q: %v", path, cause).(*diagnostic)
	d.details = map[string]string{"path": path, "reason": reason}
	return d
}

func fileReason(err error, path, reason string) error {
	if d, ok := err.(*diagnostic); ok {
		d.details = map[string]string{"path": path, "reason": reason}
	}
	return err
}

// readFile reads the whole file, validating UTF-8 at the boundary.
func readFile(pos position, path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fileOpenError(pos, path, err)
	}
	if !utf8.Valid(content) {
		return "", fileReason(errorAt(pos, CodeFileUTF8, "file %q is not valid UTF-8", path), path, "invalid-utf8")
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
		return "", false, fileReason(errorAt(pos, CodeFileRead, "file %q is closed", f.path), f.path, "closed")
	}
	raw, rerr := f.reader.ReadString('\n')
	if rerr != nil && rerr != io.EOF {
		return "", false, fileReason(errorAt(pos, CodeFileRead, "reading %q failed: %v", f.path, rerr), f.path, "read-failed")
	}
	if rerr == io.EOF && len(raw) == 0 {
		return "", false, nil
	}
	line = chomp(raw)
	if !utf8.ValidString(line) {
		return "", false, fileReason(errorAt(pos, CodeFileUTF8, "file %q is not valid UTF-8", f.path), f.path, "invalid-utf8")
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
		return value{result: &inputResult{err: fileReason(errorAt(pos, CodeFileRead, "end of file"), f.path, "end-of-file")}}
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
