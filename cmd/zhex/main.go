package main

import (
	"fmt"
	"io"
	"os"

	"zhex/internal/lang"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(output, "usage: zhex run <file.zhex>")
		return 0
	}
	if len(args) != 2 || args[0] != "run" {
		fmt.Fprintln(diagnostics, "usage: zhex run <file.zhex>")
		return 2
	}
	source, err := os.ReadFile(args[1])
	if err == nil {
		err = lang.Run(args[1], string(source), input, output)
	}
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	return 0
}
