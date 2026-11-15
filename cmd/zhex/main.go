package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zhex/internal/lang"
)

// readModule loads a sibling module file — the resolution rule for use.
func readModule(dir, name string) (lang.ModuleSource, bool) {
	path := filepath.Join(dir, name+".zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		return lang.ModuleSource{}, false
	}
	return lang.ModuleSource{Path: path, Source: string(source)}, true
}

const usage = "usage: zhex run <file.zhex> [--report=text|json] [--trace] [args...] | zhex repl [--trace] | zhex doctests <file.zhex> [--report=text|json] | zhex test [path] [--report=text|json] [--gaps]"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(output, usage)
		return 0
	}
	if len(args) >= 1 && args[0] == "repl" {
		return repl(args[1:], input, output, diagnostics)
	}
	if len(args) >= 1 && args[0] == "doctests" {
		return doctests(args[1:], input, output, diagnostics)
	}
	if len(args) >= 1 && args[0] == "test" {
		return testCommand(args[1:], input, output, diagnostics)
	}
	if len(args) < 2 || args[0] != "run" {
		fmt.Fprintln(diagnostics, usage)
		return 2
	}
	file, report := "", "text"
	var trace io.Writer
	var programArgs []string
	for _, arg := range args[1:] {
		switch {
		case arg == "--report=text" || arg == "--report=json":
			report = strings.TrimPrefix(arg, "--report=")
		case strings.HasPrefix(arg, "--report="):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case arg == "--trace":
			trace = diagnostics
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case file == "":
			file = arg
		default:
			programArgs = append(programArgs, arg)
		}
	}
	if file == "" {
		fmt.Fprintln(diagnostics, usage)
		return 2
	}
	source, err := os.ReadFile(file)
	if err == nil {
		dir := filepath.Dir(file)
		err = lang.RunWithModulesOptions(file, string(source), input, output, lang.RunOptions{Trace: trace, Args: programArgs, Diagnostics: diagnostics}, func(name string) (lang.ModuleSource, bool) {
			return readModule(dir, name)
		})
	}
	if err != nil {
		var exit *lang.ExitError
		if errors.As(err, &exit) {
			return exit.Status
		}
		if report == "json" {
			if jsonReport, ok := lang.Report(err); ok {
				fmt.Fprintln(diagnostics, jsonReport)
				return 1
			}
		}
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	return 0
}

// doctests runs the ## > examples in a file's word docs. Each failing
// doctest prints its diagnostic — one JSON line each with --report=json —
// and any failure exits 1; passing doctests print nothing.
func doctests(args []string, input io.Reader, output, diagnostics io.Writer) int {
	file, report := "", "text"
	for _, arg := range args {
		switch {
		case arg == "--report=text" || arg == "--report=json":
			report = strings.TrimPrefix(arg, "--report=")
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case file == "":
			file = arg
		default:
			fmt.Fprintln(diagnostics, usage)
			return 2
		}
	}
	if file == "" {
		fmt.Fprintln(diagnostics, usage)
		return 2
	}
	source, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	dir := filepath.Dir(file)
	failures := lang.RunDoctests(file, string(source), input, output, func(name string) (lang.ModuleSource, bool) {
		return readModule(dir, name)
	})
	for _, failure := range failures {
		if report == "json" {
			if jsonReport, ok := lang.Report(failure); ok {
				fmt.Fprintln(diagnostics, jsonReport)
				continue
			}
		}
		fmt.Fprintln(diagnostics, failure)
	}
	if len(failures) != 0 {
		return 1
	}
	return 0
}

// repl reads entries from input until EOF. A first line whose compile is
// incomplete (a block header without its indented body) collects lines until
// a blank one; the whole buffer is then one entry.
func repl(args []string, input io.Reader, output, diagnostics io.Writer) int {
	var trace io.Writer
	for _, arg := range args {
		if arg != "--trace" {
			fmt.Fprintln(diagnostics, usage)
			return 2
		}
		trace = diagnostics
	}
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	editor := newReplEditor(input, reader, output)
	var programInput io.Reader = reader
	if editor != nil {
		programInput = &replProgramInput{editor: editor}
	}
	session := lang.NewSession(programInput, output, trace, func(name string) (lang.ModuleSource, bool) {
		return readModule(".", name)
	})
	session.SetDiagnostics(diagnostics)
	if _, err := session.Enter("word bye() -> Unit\n    console.exit(0)\n"); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	fmt.Fprintln(output, "Zhex REPL — end multi-line entries with a blank line; bye() or Ctrl-D exits.")
	buffer, pending := "", false
	blockPending, openDelimiter := false, false
	listWords := func() {
		if words := session.Words(); len(words) == 0 {
			fmt.Fprintln(output, "no words defined")
		} else {
			for _, word := range words {
				fmt.Fprintln(output, word)
			}
		}
	}
	enter := func() (int, bool) {
		echo, err := session.Enter(buffer)
		if err != nil {
			var exit *lang.ExitError
			if errors.As(err, &exit) {
				return exit.Status, true
			}
			fmt.Fprintln(diagnostics, err)
			return 0, false
		}
		if echo != "" {
			fmt.Fprintln(output, echo)
		}
		return 0, false
	}
	for {
		prompt := map[bool]string{true: "....> ", false: "zhex> "}[pending]
		var line string
		var err error
		if editor != nil {
			indent := 0
			if pending {
				indent = continuationIndent(buffer)
			}
			var cancelled bool
			line, cancelled, err = editor.readLine(prompt, indent, false)
			if cancelled {
				fmt.Fprintln(output, "^C")
				buffer, pending, blockPending, openDelimiter = "", false, false, false
				continue
			}
		} else {
			fmt.Fprint(output, prompt)
			line, err = reader.ReadString('\n')
		}
		if err != nil {
			// Preserve the existing block-EOF behavior. Piped unclosed
			// delimiters receive a diagnostic, including a final partial line.
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			candidate := line + "\n"
			if pending {
				candidate = buffer + candidate
			}
			_, cerr := lang.Compile("repl", candidate)
			if editor == nil {
				if _, open := lang.DelimiterIndent(candidate, cerr); open {
					fmt.Fprintln(diagnostics, cerr)
					return 0
				}
				if pending && openDelimiter && !blockPending && !lang.IsIncomplete(cerr) {
					buffer = candidate
					if status, exited := enter(); exited {
						return status
					}
					return 0
				}
			}
			switch {
			case pending || line == "":
			case line == "words":
				listWords()
			case lang.IsIncomplete(cerr):
			default:
				buffer = line + "\n"
				if status, exited := enter(); exited {
					return status
				}
			}
			return 0
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if pending {
			blank := line == "" || (editor != nil && strings.Trim(line, " ") == "")
			if !blank && line == "words" && commentOnly(buffer) {
				// A held comment line must not swallow the words command.
				buffer, pending, blockPending = "", false, false
				listWords()
				continue
			}
			if !blank || openDelimiter {
				buffer += line + "\n"
				if commentOnly(buffer) {
					continue
				}
				_, cerr := lang.Compile("repl", buffer)
				_, openDelimiter = lang.DelimiterIndent(buffer, cerr)
				if openDelimiter || blockPending {
					continue
				}
				if lang.IsIncomplete(cerr) {
					blockPending = true
					continue
				}
				pending = false
			} else {
				pending, blockPending = false, false
			}
		} else {
			if line == "" {
				continue
			}
			if line == "words" {
				listWords()
				continue
			}
			buffer = line + "\n"
			if commentOnly(buffer) {
				// A comment line cannot complete an entry; hold it so a ##
				// doc block shares its entry with the word it documents.
				pending, openDelimiter = true, false
				continue
			}
			if _, cerr := lang.Compile("repl", buffer); lang.IsIncomplete(cerr) {
				_, openDelimiter = lang.DelimiterIndent(buffer, cerr)
				blockPending = !openDelimiter
				pending = true
				continue
			}
		}
		if status, exited := enter(); exited {
			return status
		}
	}
}

// commentOnly reports whether source holds only comment lines; such an entry
// cannot stand alone, so the loop holds it pending for the line that follows.
func commentOnly(source string) bool {
	comments := false
	for _, line := range strings.Split(source, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(strings.TrimLeft(line, " "), "#") {
			return false
		}
		comments = true
	}
	return comments
}
