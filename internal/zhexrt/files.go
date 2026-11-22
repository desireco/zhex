package zhexrt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// The file words, moved from the interpreter's host functions and
// files.go: the same phases, codes, messages, and reason taxonomy.

// File is Zhex File: an open reader the program closes; close is
// idempotent and read-after-close fails file-read.
type File struct {
	path   string
	handle *os.File
	reader *bufio.Reader
	closed bool
}

func osFailureReason(cause error) string {
	switch {
	case os.IsNotExist(cause):
		return "not-found"
	case os.IsPermission(cause):
		return "permission-denied"
	default:
		return "other"
	}
}

func fileErr(pos Pos, code, path, reason, format string, args ...any) *Error {
	return &Error{Pos: pos, Code: code, Message: fmt.Sprintf(format, args...), Details: map[string]string{"path": path, "reason": reason}}
}

// FileRead reads the whole file, validating UTF-8 at the boundary.
func FileRead(pos Pos, path string) Result[string] {
	content, err := os.ReadFile(path)
	if err != nil {
		return Result[string]{Err: fileErr(pos, "file-open", path, osFailureReason(err), "cannot open %q: %v", path, err)}
	}
	if !utf8.Valid(content) {
		return Result[string]{Err: fileErr(pos, "file-utf8", path, "invalid-utf8", "file %q is not valid UTF-8", path)}
	}
	return Ok(string(content))
}

// FileOpen opens path for reading.
func FileOpen(pos Pos, path string) Result[*File] {
	handle, err := os.Open(path)
	if err != nil {
		return Result[*File]{Err: fileErr(pos, "file-open", path, osFailureReason(err), "cannot open %q: %v", path, err)}
	}
	return Ok(&File{path: path, handle: handle, reader: bufio.NewReader(handle)})
}

// nextLine returns the next chomped, UTF-8-validated line; ok is false at
// end of file.
func (f *File) nextLine(pos Pos) (string, bool, *Error) {
	if f.closed {
		return "", false, fileErr(pos, "file-read", f.path, "closed", "file %q is closed", f.path)
	}
	raw, rerr := f.reader.ReadString('\n')
	if rerr != nil && rerr != io.EOF {
		return "", false, fileErr(pos, "file-read", f.path, "read-failed", "reading %q failed: %v", f.path, rerr)
	}
	if rerr == io.EOF && len(raw) == 0 {
		return "", false, nil
	}
	line := Chomp(raw)
	if !utf8.ValidString(line) {
		return "", false, fileErr(pos, "file-utf8", f.path, "invalid-utf8", "file %q is not valid UTF-8", f.path)
	}
	return line, true, nil
}

// StreamLine is file.stream's loop body: one chomped line, ok false at
// end of file; failures are hard diagnostics (the stream always closes).
func StreamLine(pos Pos, f *File) (string, bool) {
	line, ok, err := f.nextLine(pos)
	if err != nil {
		panic(err)
	}
	return line, ok
}

// FileReadLineOf is file.read-line on an open file: end of file fails
// inside the Result.
func FileReadLineOf(pos Pos, f *File) Result[string] {
	line, ok, err := f.nextLine(pos)
	if err != nil {
		return Result[string]{Err: err}
	}
	if !ok {
		return Result[string]{Err: fileErr(pos, "file-read", f.path, "end-of-file", "end of file")}
	}
	return Ok(line)
}

// FileClose releases the handle; closing again is a no-op.
func FileClose(f *File) Unit {
	if f.closed {
		return Unit{}
	}
	f.closed = true
	f.handle.Close()
	return Unit{}
}

// StreamOpen is file.stream's opener: failures are hard diagnostics
// carrying the stream's operation name.
func StreamOpen(pos Pos, path string) *File {
	handle, err := os.Open(path)
	if err != nil {
		d := fileErr(pos, "file-open", path, osFailureReason(err), "cannot open %q: %v", path, err)
		d.Operation = "file.stream"
		panic(d)
	}
	return &File{path: path, handle: handle, reader: bufio.NewReader(handle)}
}

// FileWrite writes contents in one open-write-close: create-or-truncate
// by default, or append (creating a missing file) when appendMode is set.
// shortcut: overwrite moves aside, plain os.File calls only — no injected
// failure seam here; the interpreter's tests own that coverage.
func FileWrite(pos Pos, contents, path string, appendMode bool) Result[Unit] {
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return Result[Unit]{Err: writeErr(pos, path, "open", err)}
	}
	n, err := f.Write([]byte(contents))
	if err == nil && n != len(contents) {
		err = io.ErrShortWrite
	}
	if err != nil {
		_ = f.Close()
		return Result[Unit]{Err: writeErr(pos, path, "write", err)}
	}
	if err := f.Close(); err != nil {
		return Result[Unit]{Err: writeErr(pos, path, "close", err)}
	}
	return Ok(Unit{})
}

func writeErr(pos Pos, path, phase string, cause error) *Error {
	var message string
	switch phase {
	case "open":
		message = fmt.Sprintf("cannot open %q for writing: %v", path, cause)
	case "close":
		message = fmt.Sprintf("closing %q failed: %v", path, cause)
	default:
		message = fmt.Sprintf("writing %q failed: %v", path, cause)
	}
	return &Error{Pos: pos, Code: "file-write", Message: message, Details: map[string]string{"path": path, "reason": osFailureReason(cause), "phase": phase}}
}

// FileWriteBytes writes one byte per Int, 0..255, in one open-write-close
// that always truncates (the lean #211 surface: no append). A value outside
// 0..255 fails file-bytes-range inside the Result before the file is
// opened; the message matches the interpreter's byte-for-byte.
func FileWriteBytes(pos Pos, path string, bytes *List[Int]) Result[Unit] {
	data := make([]byte, len(bytes.Items))
	for i, item := range bytes.Items {
		if item.Negative() || !item.Fits64() || item.AsInt64() > 255 {
			return Result[Unit]{Err: &Error{Pos: pos, Code: "file-bytes-range", Message: fmt.Sprintf("file.write-bytes byte %s is outside 0..255", Str(item))}}
		}
		data[i] = byte(item.AsInt64())
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return Result[Unit]{Err: writeErr(pos, path, "open", err)}
	}
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		_ = f.Close()
		return Result[Unit]{Err: writeErr(pos, path, "write", err)}
	}
	if err := f.Close(); err != nil {
		return Result[Unit]{Err: writeErr(pos, path, "close", err)}
	}
	return Ok(Unit{})
}

// FileList lists bare entry names, sorted, dot entries included.
func FileList(pos Pos, folder string) Result[*List[string]] {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return Result[*List[string]]{Err: inspectErr(pos, "file-list", "cannot list %q: %v", folder, err)}
	}
	if len(entries) > MaxElements {
		return Result[*List[string]]{Err: &Error{Pos: pos, Code: "collection-limit", Message: "file.list exceeds element limit"}}
	}
	items := make([]string, len(entries))
	for i, entry := range entries {
		items[i] = entry.Name()
	}
	return Ok(&List[string]{Items: items})
}

func inspectErr(pos Pos, code, format string, path string, cause error) *Error {
	return &Error{Pos: pos, Code: code, Message: fmt.Sprintf(format, path, cause), Details: map[string]string{"path": path, "reason": osFailureReason(cause)}}
}

// FileExists is the checked form (the plain Bool form reads .Value with
// the same Result).
func FileExists(pos Pos, path string) Result[bool] {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Ok(false)
		}
		return Result[bool]{Err: inspectErr(pos, "file-exists", "cannot check %q: %v", path, err)}
	}
	return Ok(true)
}

// FileSize reports regular-file bytes; directories are 0.
func FileSize(pos Pos, path string) Result[Int] {
	info, err := os.Stat(path)
	if err != nil {
		return Result[Int]{Err: inspectErr(pos, "file-size", "cannot get the size of %q: %v", path, err)}
	}
	var size int64
	if !info.IsDir() {
		size = info.Size()
	}
	return Ok(I64(size))
}

// The pure lexical path words.
func FileJoin(base, child string) string {
	if filepath.IsAbs(child) {
		return filepath.Clean(child)
	}
	return filepath.Join(base, child)
}

func FileBase(path string) string        { return filepath.Base(path) }
func FileExtension(path string) string   { return filepath.Ext(path) }
func FileDirectoryOf(path string) string { return filepath.Dir(path) }

// The destructive words.
func FileMakeFolder(pos Pos, path string, parents bool) Result[Unit] {
	var err error
	if parents {
		err = os.MkdirAll(path, 0o755)
	} else {
		err = os.Mkdir(path, 0o755)
	}
	if err != nil {
		return Result[Unit]{Err: inspectErr(pos, "file-make-folder", "cannot create folder %q: %v", path, err)}
	}
	return Ok(Unit{})
}

func FileDelete(pos Pos, path string) Result[Unit] {
	if err := os.Remove(path); err != nil {
		return Result[Unit]{Err: inspectErr(pos, "file-delete", "cannot delete %q: %v", path, err)}
	}
	return Ok(Unit{})
}

func FileDeleteFolder(pos Pos, path string, recursive bool) Result[Unit] {
	var err error
	if recursive {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		return Result[Unit]{Err: inspectErr(pos, "file-delete-folder", "cannot remove folder %q: %v", path, err)}
	}
	return Ok(Unit{})
}

func moveErr(pos Pos, source, target, reason, format string, args ...any) *Error {
	return &Error{Pos: pos, Code: "file-move", Message: fmt.Sprintf(format, args...), Details: map[string]string{"path": source, "target": target, "reason": reason}}
}

// FileMove renames source to target, files and folders. overwrite
// replaces an existing file only; a folder target is refused.
// shortcut: overwrite uses plain rename, which refuses existing targets on
// Windows; add a platform-tagged replace file when Windows matters.
func FileMove(pos Pos, source, target string, overwrite bool) Result[Unit] {
	if info, err := os.Lstat(target); err == nil {
		if info.IsDir() {
			return Result[Unit]{Err: moveErr(pos, source, target, "other", "cannot move %q to %q: the target is a folder", source, target)}
		}
		if !overwrite {
			return Result[Unit]{Err: moveErr(pos, source, target, "other", "cannot move %q to %q: the target exists", source, target)}
		}
	} else if !os.IsNotExist(err) {
		return Result[Unit]{Err: moveErr(pos, source, target, osFailureReason(err), "cannot move %q to %q: %v", source, target, err)}
	}
	if err := os.Rename(source, target); err != nil {
		return Result[Unit]{Err: moveErr(pos, source, target, osFailureReason(err), "cannot move %q to %q: %v", source, target, err)}
	}
	return Ok(Unit{})
}
