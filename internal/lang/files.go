package lang

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// osWrite and osClose are the write-side host seam: production uses the real
// os.File methods; tests swap them to inject write failures, short writes,
// and close failures.
var osWrite = (*os.File).Write
var osClose = (*os.File).Close

// osStat, osReadDir, and osFreeSpace are the inspect-side host seam:
// production uses the real os functions; tests swap them to inject stat,
// listing, and free-space failures.
var (
	osStat      = os.Stat
	osReadDir   = os.ReadDir
	osFreeSpace = freeSpaceBytes
)

// joinPath follows filepath.Join except that an absolute child replaces the
// base (the Python PurePath rule), the safer composition for untrusted parts.
func joinPath(base, child string) string {
	if filepath.IsAbs(child) {
		return filepath.Clean(child)
	}
	return filepath.Join(base, child)
}

// osMkdir, osMkdirAll, osRemove, osRemoveAll, and osRename are the
// destructive-side host seam: production uses the real os functions; tests
// swap them to inject permission, cross-device, and partial-removal
// failures.
var (
	osMkdir     = os.Mkdir
	osMkdirAll  = os.MkdirAll
	osRemove    = os.Remove
	osRemoveAll = os.RemoveAll
	osRename    = os.Rename
)

// osFailureReason classifies an operating-system failure with the read-side
// taxonomy shared by file-open and file-write.
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

// inspectError reports a failed path inspection with the operation's code;
// the details carry the path and the reason taxonomy.
func inspectError(pos position, code Code, format string, path string, cause error) error {
	d := errorAt(pos, code, format, path, cause).(*diagnostic)
	d.details = map[string]string{"path": path, "reason": osFailureReason(cause)}
	return d
}

// writeTextFile writes contents to path in one internal open-write-close:
// create-or-truncate by default, or append (creating a missing file) when
// appendMode is set. No writable File escapes, a failure may leave a partial
// or truncated file, created files use mode 0644 before umask, and the write
// path follows symlinks.
func writeTextFile(pos position, contents, path string, appendMode bool) value {
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return value{result: &inputResult{err: fileWriteError(pos, path, "open", err)}}
	}
	n, err := osWrite(f, []byte(contents))
	if err == nil && n != len(contents) {
		err = io.ErrShortWrite
	}
	if err != nil {
		_ = osClose(f)
		return value{result: &inputResult{err: fileWriteError(pos, path, "write", err)}}
	}
	if err := osClose(f); err != nil {
		return value{result: &inputResult{err: fileWriteError(pos, path, "close", err)}}
	}
	return value{result: &inputResult{}}
}

// fileWriteError maps a failed write phase to file-write with the read-side
// reason taxonomy (not-found, permission-denied, other) and the phase.
func fileWriteError(pos position, path, phase string, cause error) error {
	reason := "other"
	if os.IsNotExist(cause) {
		reason = "not-found"
	} else if os.IsPermission(cause) {
		reason = "permission-denied"
	}
	var message string
	switch phase {
	case "open":
		message = fmt.Sprintf("cannot open %q for writing: %v", path, cause)
	case "close":
		message = fmt.Sprintf("closing %q failed: %v", path, cause)
	default:
		message = fmt.Sprintf("writing %q failed: %v", path, cause)
	}
	d := errorAt(pos, CodeFileWrite, "%s", message).(*diagnostic)
	d.details = map[string]string{"path": path, "reason": reason, "phase": phase}
	return d
}

// createFolder creates one folder level, or every missing ancestor when
// parents is set; os.MkdirAll tolerates an already-existing folder while a
// plain os.Mkdir fails on any existing target.
func createFolder(pos position, path string, parents bool) value {
	var err error
	if parents {
		err = osMkdirAll(path, 0o755)
	} else {
		err = osMkdir(path, 0o755)
	}
	if err != nil {
		return value{result: &inputResult{err: inspectError(pos, CodeFileMakeFolder, "cannot create folder %q: %v", path, err)}}
	}
	return value{result: &inputResult{}}
}

// deletePath removes a file or an empty folder, os.Remove semantics: a
// nonempty folder fails and nothing is removed.
func deletePath(pos position, path string) value {
	if err := osRemove(path); err != nil {
		return value{result: &inputResult{err: inspectError(pos, CodeFileDelete, "cannot delete %q: %v", path, err)}}
	}
	return value{result: &inputResult{}}
}

// deleteFolder removes an empty folder, or the whole tree best-effort when
// recursive is set: os.RemoveAll may delete part of the tree before the
// first failure, and earlier effects stand — no transaction promise.
func deleteFolder(pos position, path string, recursive bool) value {
	var err error
	if recursive {
		err = osRemoveAll(path)
	} else {
		err = osRemove(path)
	}
	if err != nil {
		return value{result: &inputResult{err: inspectError(pos, CodeFileDeleteFolder, "cannot remove folder %q: %v", path, err)}}
	}
	return value{result: &inputResult{}}
}

// moveError reports a failed move naming both paths, with the shared reason
// taxonomy in its details.
func moveError(pos position, source, target, reason, format string, args ...any) error {
	d := errorAt(pos, CodeFileMove, format, args...).(*diagnostic)
	d.details = map[string]string{"path": source, "target": target, "reason": reason}
	return d
}

// movePath renames source to target, covering files and folders. overwrite
// replaces an existing file only; a folder target is refused in both modes
// so every platform reports one contract. Cross-device moves fail with the
// platform's error — no copy fallback — and the gap between the target
// check and the rename is a documented check-then-act window: a target that
// appears in between is left to rename itself.
func movePath(pos position, source, target string, overwrite bool) value {
	if info, err := osStat(target); err == nil {
		if info.IsDir() {
			return value{result: &inputResult{err: moveError(pos, source, target, "other", "cannot move %q to %q: the target is a folder", source, target)}}
		}
		if !overwrite {
			return value{result: &inputResult{err: moveError(pos, source, target, "other", "cannot move %q to %q: the target exists", source, target)}}
		}
	} else if !os.IsNotExist(err) {
		return value{result: &inputResult{err: moveError(pos, source, target, osFailureReason(err), "cannot move %q to %q: %v", source, target, err)}}
	}
	if err := osRename(source, target); err != nil {
		return value{result: &inputResult{err: moveError(pos, source, target, osFailureReason(err), "cannot move %q to %q: %v", source, target, err)}}
	}
	return value{result: &inputResult{}}
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
