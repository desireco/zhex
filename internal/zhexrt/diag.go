package zhexrt

import (
	"fmt"
	"os"
	"strings"
)

// Pos is a source position inside the compiled program; the generated
// code carries one at every operation that can fail at runtime so errors
// render exactly as zhex run's do.
type Pos struct {
	File string
	Line int
	Col  int
}

// At constructs a Pos concisely at generated call sites.
func At(file string, line, col int) Pos { return Pos{File: file, Line: line, Col: col} }

// Sources holds every compiled file's text, keyed by path, so Error can
// print the excerpt and caret the interpreter's diagnostics print. The
// generated program assigns it in init.
var Sources = map[string]string{}

// Error is a Zhex diagnostic at runtime: the same first line, excerpt,
// caret, and hint shape zhex run prints, rendered by Run at the boundary.
type Error struct {
	Pos       Pos
	Code      string
	Message   string
	Hint      string
	Operation string
	Details   map[string]string
	Cause     *Error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.Pos.File, e.Pos.Line, e.Pos.Col, e.Message)
}

// RenderDiag formats the stable diagnostic text — first line, optional
// excerpt with caret line, optional hint — shared by the interpreter's
// checker errors and this runtime's failures so the two never diverge.
func RenderDiag(file string, line, column int, message, hint, sourceLine string, spanStart, spanEnd int) string {
	out := fmt.Sprintf("%s:%d:%d: %s", file, line, column, message)
	if sourceLine != "" {
		out += "\n    " + sourceLine + "\n    "
		indent, width := column-1, 1
		if spanStart > 0 && spanEnd > spanStart+1 {
			indent, width = spanStart-1, spanEnd-spanStart
		}
		if indent < 0 {
			indent = 0
		}
		if width < 1 {
			width = 1
		}
		out += strings.Repeat(" ", indent) + strings.Repeat("^", width)
	}
	if hint != "" {
		out += "\nhint: " + hint
	}
	return out
}

func (e *Error) render() string {
	line := ""
	if src, ok := Sources[e.Pos.File]; ok {
		if lines := strings.Split(src, "\n"); e.Pos.Line >= 1 && e.Pos.Line <= len(lines) {
			line = strings.TrimSuffix(lines[e.Pos.Line-1], "\r")
		}
	}
	return RenderDiag(e.Pos.File, e.Pos.Line, e.Pos.Col, e.Message, e.Hint, line, 0, 0)
}

// Fail raises a runtime diagnostic as a panic; Run recovers it, renders
// it, and exits 1. Panics keep hard errors unwinding past every Zhex
// construct (they are never catchable), while Go still runs deferred
// cleanup along the way.
func Fail(pos Pos, code, format string, args ...any) {
	panic(&Error{Pos: pos, Code: code, Message: fmt.Sprintf(format, args...)})
}

// FailOp is Fail with the operation a diagnostic narrates (the word the
// error passed through, e.g. "/" or "list.append").
func FailOp(pos Pos, code, operation, format string, args ...any) {
	panic(&Error{Pos: pos, Code: code, Message: fmt.Sprintf(format, args...), Operation: operation})
}

// Exit requests a deliberate program exit (console.exit / console.fail).
type Exit struct{ Status int }

// Do exits with status, mirroring console.exit.
func Do(status int) { panic(&Exit{Status: status}) }

// Run executes a compiled program's entry and turns its outcome into the
// process contract zhex run honors: a nil error exits 0, a diagnostic
// renders to stderr and exits 1, a deliberate exit takes its status, and
// no Go panic ever leaks raw (it becomes a diagnostic naming the code).
func Run(entry func() *Error) {
	var result *Error
	func() {
		defer func() {
			if r := recover(); r != nil {
				switch v := r.(type) {
				case *Error:
					result = v
				case *Exit:
					os.Exit(v.Status)
				default:
					panic(r)
				}
			}
		}()
		result = entry()
	}()
	if result != nil {
		fmt.Fprintln(os.Stderr, result.render())
		os.Exit(1)
	}
}
