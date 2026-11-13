package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"zhex/internal/lang"
)

const usage = "usage: zhex run <file.zhex> [--report=text|json] [--trace] [args...] | zhex repl [--trace]"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(output, usage)
		return 0
	}
	if len(args) >= 1 && args[0] == "repl" {
		return repl(args[1:], input, output, diagnostics)
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
		err = lang.RunWithArgs(file, string(source), input, output, trace, programArgs)
	}
	if err != nil {
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
	session := lang.NewSession(reader, output, trace)
	fmt.Fprintln(output, "Zhex REPL — end multi-line entries with a blank line; Ctrl-D exits.")
	buffer, pending := "", false
	listWords := func() {
		if words := session.Words(); len(words) == 0 {
			fmt.Fprintln(output, "no words defined")
		} else {
			for _, word := range words {
				fmt.Fprintln(output, word)
			}
		}
	}
	enter := func() {
		echo, err := session.Enter(buffer)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return
		}
		if echo != "" {
			fmt.Fprintln(output, echo)
		}
	}
	for {
		fmt.Fprint(output, map[bool]string{true: "....> ", false: "zhex> "}[pending])
		line, err := reader.ReadString('\n')
		if err != nil {
			// A final line without its newline still runs; anything that
			// would keep collecting lines is discarded at EOF.
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			_, cerr := lang.Compile("repl", line+"\n")
			switch {
			case pending || line == "":
			case line == "words":
				listWords()
			case lang.IsIncomplete(cerr):
			default:
				buffer = line + "\n"
				enter()
			}
			return 0
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if pending {
			if line != "" {
				buffer += line + "\n"
				continue
			}
			pending = false
		} else {
			if line == "" {
				continue
			}
			if line == "words" {
				listWords()
				continue
			}
			buffer = line + "\n"
			if _, cerr := lang.Compile("repl", buffer); lang.IsIncomplete(cerr) {
				pending = true
				continue
			}
		}
		enter()
	}
}
