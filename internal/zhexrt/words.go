package zhexrt

import (
	"fmt"
	"os"
	"strings"
)

// Program arguments (console.arg / console.args-count): the generated
// binary's os.Args minus the program name.
var Args = os.Args[1:]

// ArgsAt is console.arg: an out-of-range index fails inside the Result.
func ArgsAt(pos Pos, index Int) Result[string] {
	if !index.Fits64() || index.AsInt64() < 0 || index.AsInt64() >= int64(len(Args)) {
		return ErrAt[string](pos, "argument-missing", fmt.Sprintf("argument index %s is out of range; the program received %d", Str(index), len(Args)), map[string]string{"index": Str(index), "argument-count": fmt.Sprint(len(Args))})
	}
	return Ok(Args[index.AsInt64()])
}

func ArgsCount() Int { return I64(int64(len(Args))) }

// ExitWith is console.exit: an invalid status is a hard diagnostic.
func ExitWith(pos Pos, status Int) Unit {
	if status.Negative() || Cmp(status, I64(255)) > 0 {
		FailOp(pos, "console-exit-status", "console.exit", "console.exit status must be between 0 and 255, found %s", Str(status))
	}
	Do(int(status.AsInt64()))
	return Unit{}
}

// FailWith is console.fail: one stderr line, then exit status 1 — no
// diagnostic.
func FailWith(message string) Unit {
	_, _ = os.Stderr.WriteString(message + "\n")
	Do(1)
	return Unit{}
}

// EnvGet is env.get: total Option (unset is None, set-but-empty Some("")).
func EnvGet(name string) Option[string] {
	if text, ok := os.LookupEnv(name); ok {
		return Some(text)
	}
	return None[string]()
}

// EnvLoad reads KEY=VALUE lines into the process environment: blank lines
// and whole-line comments are skipped, one surrounding quote pair is
// stripped, and a key already present — in the environment or set by an
// earlier line — keeps its value, so the first assignment wins. Lines
// before a malformed one stay applied.
func EnvLoad(pos Pos, path string) Result[Unit] {
	content, err := os.ReadFile(path)
	if err != nil {
		return ErrAt[Unit](pos, "env-read", fmt.Sprintf("cannot read %q: %v", path, err), map[string]string{"path": path, "reason": osFailureReason(err)})
	}
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, text, found := strings.Cut(line, "=")
		key, text = strings.TrimSpace(key), strings.TrimSpace(text)
		if !found || key == "" || strings.ContainsAny(key, " \t") {
			return ErrAt[Unit](pos, "env-parse", fmt.Sprintf("line %d of %q is not a KEY=VALUE assignment: %q", i+1, path, line), map[string]string{"path": path, "line": fmt.Sprintf("%d", i+1)})
		}
		if len(text) >= 2 && (text[0] == '"' || text[0] == '\'') && text[len(text)-1] == text[0] {
			text = text[1 : len(text)-1]
		}
		if _, present := os.LookupEnv(key); !present {
			os.Setenv(key, text)
		}
	}
	return Ok(Unit{})
}

// The error words over *Error, moved from the interpreter's host
// functions. A caught error flows as a value here; error.new and
// error.wrap build fresh ones at the call site's position.

func ErrorNew(pos Pos, code, message string) *Error {
	return &Error{Pos: pos, Code: code, Message: message, Operation: "error.new", uncataloged: true}
}

func ErrorWrap(pos Pos, code, message string, cause *Error) *Error {
	return &Error{Pos: cause.Pos, Code: code, Message: message, Operation: cause.Operation, Details: nil, Cause: cause, uncataloged: true}
}

func ErrorHasCause(e *Error) bool { return e.Cause != nil }

func ErrorCause(pos Pos, e *Error) Result[*Error] {
	if e.Cause == nil {
		return ErrAt[*Error](pos, "error-no-cause", "error has no cause", nil)
	}
	return Ok(e.Cause)
}

func ErrorCode(e *Error) string      { return e.Code }
func ErrorMessage(e *Error) string   { return e.Message }
func ErrorOperation(e *Error) string { return e.Operation }
func ErrorFile(e *Error) string      { return e.Pos.File }
func ErrorLine(e *Error) Int         { return I64(int64(e.Pos.Line)) }
func ErrorColumn(e *Error) Int       { return I64(int64(e.Pos.Col)) }
func ErrorDetail(e *Error, name string) string {
	if e.Details == nil {
		return ""
	}
	return e.Details[name]
}
