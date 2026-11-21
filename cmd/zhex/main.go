package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"zhex/internal/lang"
	"zhex/internal/native"
	"zhex/internal/wasm"
	"zhex/internal/zhexrt"
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

const usage = "usage: zhex run <file.zhex> [--report=text|json] [--trace] [args...] | zhex compile <file.zhex> [-o <out>] [--target=browser|wasi|native] | zhex repl [--trace] | zhex doctests <file.zhex> [--report=text|json] | zhex test [path] [--report=text|json] [--gaps] | zhex errors [code] [--json]"

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostics io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		fmt.Fprintln(output, usage)
		return 0
	}
	if len(args) >= 1 && args[0] == "repl" {
		return repl(args[1:], input, output, diagnostics)
	}
	if len(args) >= 1 && args[0] == "compile" {
		return compileCmd(args[1:], diagnostics)
	}
	if len(args) >= 1 && args[0] == "doctests" {
		return doctests(args[1:], input, output, diagnostics)
	}
	if len(args) >= 1 && args[0] == "test" {
		return testCommand(args[1:], input, output, diagnostics)
	}
	if len(args) >= 1 && args[0] == "errors" {
		return errorsCmd(args[1:], output, diagnostics)
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

// compileCmd implements `zhex compile <file.zhex> [-o <out>]
// [--target=browser|wasi|native]`: the wasm targets check, emit WAT, and
// assemble with wasm-tools into one artifact; the native target emits a
// Go program directory (main.go, go.mod, zhexrt) and builds it with the
// Go toolchain, or writes the sources with a build instruction when go is
// absent. Checker errors render exactly as run's; a missing wasm-tools
// prints the pinned missing-tool diagnostic; either way no artifact is
// written.
func compileCmd(args []string, diagnostics io.Writer) int {
	file, outPath := "", ""
	skin := lang.WatSkinBrowser
	target := "browser"
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-o":
			if i+1 >= len(args) || outPath != "" {
				fmt.Fprintln(diagnostics, usage)
				return 2
			}
			i++
			outPath = args[i]
		case arg == "--target=browser":
			target, skin = "browser", lang.WatSkinBrowser
		case arg == "--target=wasi":
			target, skin = "wasi", lang.WatSkinWASI
		case arg == "--target=native":
			target = "native"
		case strings.HasPrefix(arg, "-"):
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
	if outPath == "" {
		if target == "native" {
			outPath = strings.TrimSuffix(file, ".zhex")
		} else if strings.HasSuffix(file, ".zhex") {
			outPath = strings.TrimSuffix(file, ".zhex") + ".wasm"
		} else {
			outPath = file + ".wasm"
		}
	}
	source, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	dir := filepath.Dir(file)
	if target == "native" {
		if err := native.Compile(file, string(source), outPath, func(name string) (lang.ModuleSource, bool) {
			return readModule(dir, name)
		}); err != nil {
			var missing *native.MissingToolError
			if errors.As(err, &missing) {
				// the sources are written; the toolchain is the only thing missing
				fmt.Fprintln(diagnostics, err)
				return 0
			}
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		return 0
	}
	if err := wasm.Compile(file, string(source), outPath, func(name string) (lang.ModuleSource, bool) {
		return readModule(dir, name)
	}, skin); err != nil {
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
	failures := lang.RunDoctests(file, string(source), input, io.Discard, func(name string) (lang.ModuleSource, bool) {
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

// errorDoc is one catalog entry in `zhex errors --json` output.
type errorDoc struct {
	Code string `json:"code"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

// errorsCmd implements `zhex errors [code] [--json]`: the repair guide
// shipped inside the binary. It renders no content of its own — the
// entries come from the same go:embed catalog the diagnostics raise
// codes from, so the guide cannot drift from the tool. An unknown code
// exits 1 suggesting the closest catalog code; malformed invocations
// are usage errors.
func errorsCmd(args []string, output, diagnostics io.Writer) int {
	code, asJSON := "", false
	for _, arg := range args {
		switch {
		case arg == "--json":
			asJSON = true
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case code == "":
			code = arg
		default:
			fmt.Fprintln(diagnostics, usage)
			return 2
		}
	}
	entries := lang.CatalogEntries()
	if asJSON {
		if code != "" {
			fmt.Fprintln(diagnostics, usage)
			return 2
		}
		docs := make([]errorDoc, len(entries))
		for i, entry := range entries {
			docs[i] = errorDoc{Code: entry.Code, Text: entry.Text, URL: lang.Code(entry.Code).DocsURL()}
		}
		out, err := json.Marshal(docs)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		fmt.Fprintln(output, string(out))
		return 0
	}
	if code == "" {
		fmt.Fprintf(output, "%d diagnostic codes — full entry: zhex errors <code> — %s\n", len(entries), zhexrt.DocsBaseURL)
		for _, entry := range entries {
			fmt.Fprintf(output, "%s  %s\n", entry.Code, firstSentence(entry.Text))
		}
		return 0
	}
	for _, entry := range entries {
		if entry.Code != code {
			continue
		}
		fmt.Fprintln(output, entry.Code)
		fmt.Fprintln(output, entry.Text)
		fmt.Fprintln(output, "learn: "+lang.Code(entry.Code).DocsURL())
		return 0
	}
	fmt.Fprintf(diagnostics, "unknown code %q\n", code)
	if hint := lang.CodeHint(code); hint != "" {
		fmt.Fprintln(diagnostics, "hint: "+hint)
	}
	return 1
}

// firstSentence flattens a catalog entry to its first sentence — the
// one-line listing form. Section prose can wrap lines, so join before
// cutting at the sentence boundary.
func firstSentence(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	if i := strings.Index(flat, ". "); i >= 0 {
		return flat[:i+1]
	}
	return flat
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
