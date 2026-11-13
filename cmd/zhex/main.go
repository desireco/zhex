package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"zhex/internal/lang"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	usage := "usage: zhex run <file.zhex> [--report=text|json] [--trace]"
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(output, usage)
		return 0
	}
	if len(args) < 2 || args[0] != "run" {
		fmt.Fprintln(diagnostics, usage)
		return 2
	}
	file, report := "", "text"
	var trace io.Writer
	for _, arg := range args[1:] {
		switch {
		case arg == "--report=text" || arg == "--report=json":
			report = strings.TrimPrefix(arg, "--report=")
		case strings.HasPrefix(arg, "--report="):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case arg == "--trace":
			trace = diagnostics
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
	if err == nil {
		err = lang.RunWithTrace(file, string(source), input, output, trace)
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
