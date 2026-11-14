package main

import (
	"bufio"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
	"zhex/internal/lang"
)

// continuationIndent follows the current block depth and adds a level only
// when the parser is waiting for a nested body.
func continuationIndent(source string) int {
	lines := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	last := lines[len(lines)-1]
	indent := len(last) - len(strings.TrimLeft(last, " "))
	if _, err := lang.Compile("repl", source); lang.IsIncomplete(err) {
		indent += 4
	}
	return indent
}

// terminalInput is the sole editor consumer of the shared REPL reader. It
// feeds the terminal editor one byte at a time so it never retains program
// input that a subsequent read-line operation needs.
type terminalInput struct {
	reader       *bufio.Reader
	output       io.Writer
	queued       []byte
	inPaste      bool
	touched      bool
	content      bool
	indent       int
	deferred     int
	cancelled    bool
	readErr      error
	lastCR       bool
	programInput bool
}

func (in *terminalInput) Write(p []byte) (int, error) { return in.output.Write(p) }
func (in *terminalInput) prepare(indent int) {
	in.touched, in.content, in.cancelled = false, false, false
	in.indent, in.deferred = indent, 0
	if in.inPaste {
		in.deferred = indent
	} else {
		in.queued = append(in.queued, strings.Repeat(" ", indent)...)
	}
}
func (in *terminalInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(in.queued) == 0 {
		if in.readErr != nil {
			err := in.readErr
			in.readErr = nil
			return 0, err
		}
		b, err := in.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == '\n' {
			if in.inPaste && in.lastCR {
				in.lastCR = false
				return in.Read(p)
			}
			in.lastCR = false
			b = '\r'
		} else {
			in.lastCR = b == '\r'
		}
		switch b {
		case '\x1b':
			seq := []byte{b}
			const start, end = "\x1b[200~", "\x1b[201~"
			for (strings.HasPrefix(start, string(seq)) || strings.HasPrefix(end, string(seq))) && len(seq) < len(start) {
				next, err := in.reader.ReadByte()
				if err != nil {
					in.readErr = err
					break
				}
				seq = append(seq, next)
			}
			switch string(seq) {
			case start:
				if !in.touched && in.indent > 0 && !in.inPaste {
					in.queued = append(in.queued, '\x15')
				}
				in.inPaste = true
				in.queued = append(in.queued, seq...)
			case end:
				in.inPaste = false
				in.queued = append(in.queued, seq...)
				if !in.content && in.deferred > 0 {
					in.queued = append(in.queued, strings.Repeat(" ", in.deferred)...)
					in.indent = in.deferred
				}
				in.deferred = 0
			default:
				in.touched = true
				in.queued = append(in.queued, seq...)
			}
		case '\t':
			in.touched = true
			if in.inPaste || in.programInput {
				in.queued = append(in.queued, b)
			} else {
				in.queued = append(in.queued, "    "...)
			}
		case '\x03':
			if in.inPaste {
				in.queued = append(in.queued, b)
			} else {
				in.cancelled = true
				in.queued = append(in.queued, '\x15', '\r')
			}
		case '\x04':
			if !in.inPaste && !in.touched && in.indent > 0 {
				in.queued = append(in.queued, '\x15')
			}
			in.queued = append(in.queued, b)
		default:
			if b != '\r' && b != '\n' {
				in.touched = true
				in.content = true
			}
			in.queued = append(in.queued, b)
		}
	}
	p[0] = in.queued[0]
	in.queued = in.queued[1:]
	return 1, nil
}

type replEditor struct {
	fd       int
	input    *terminalInput
	terminal *term.Terminal
}

func newReplEditor(input io.Reader, reader *bufio.Reader, output io.Writer) *replEditor {
	file, ok := input.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return nil
	}
	out, ok := output.(*os.File)
	if !ok || !term.IsTerminal(int(out.Fd())) {
		return nil
	}
	in := &terminalInput{reader: reader, output: output}
	editor := &replEditor{fd: int(file.Fd()), input: in, terminal: term.NewTerminal(in, "")}
	if width, height, err := term.GetSize(editor.fd); err == nil && width > 0 && height > 0 {
		_ = editor.terminal.SetSize(width, height)
	}
	return editor
}

func (e *replEditor) readLine(prompt string, indent int, programInput bool) (string, bool, error) {
	state, err := term.MakeRaw(e.fd)
	if err != nil {
		return "", false, err
	}
	defer term.Restore(e.fd, state)
	e.terminal.SetPrompt(prompt)
	e.input.programInput = programInput
	e.terminal.AutoCompleteCallback = func(line string, pos int, key rune) (string, int, bool) {
		if programInput && key == '\t' {
			return line[:pos] + "\t" + line[pos:], pos + 1, true
		}
		// x/term v0.30 otherwise discards typed runes at exactly 4096;
		// retain long source lines rather than silently changing them.
		if len(line) >= 4096 && utf8.RuneCountInString(line) == 4096 && key >= 32 {
			value := string(key)
			return line[:pos] + value + line[pos:], pos + len(value), true
		}
		return "", 0, false
	}
	e.terminal.SetBracketedPasteMode(true)
	defer e.terminal.SetBracketedPasteMode(false)
	e.input.prepare(indent)
	line, err := e.terminal.ReadLine()
	if err == term.ErrPasteIndicator {
		err = nil
	}
	return line, e.input.cancelled, err
}

// Program input uses the same synchronous terminal reader, so bytes queued
// while the editor is in raw mode cannot be stranded across a mode change.
type replProgramInput struct {
	editor    *replEditor
	remaining []byte
}

func (in *replProgramInput) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(in.remaining) == 0 {
		line, cancelled, err := in.editor.readLine("", 0, true)
		if cancelled {
			return 0, io.EOF
		}
		if err != nil {
			return 0, err
		}
		in.remaining = append([]byte(line), '\n')
	}
	n := copy(p, in.remaining)
	in.remaining = in.remaining[n:]
	return n, nil
}
