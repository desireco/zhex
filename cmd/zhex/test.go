package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"zhex/internal/lang"
)

// testBinary supplies the executable the runner re-executes for each
// .test.zhex file; tests point it at a built binary.
var testBinary = os.Executable

// skippedFolders never hold tests: version control, dependencies, site output, editor syntax data, tools.
var skippedFolders = map[string]bool{".git": true, "node_modules": true, "_site": true, "editors": true, "tools": true}

// testCommand runs a project's tests under one scope: every *.test.zhex file
// as its own process — the exit code is the whole contract — and every other
// .zhex file's ## > doctests, then one summary line and exit 1 on any
// failure. --gaps replaces the run with the list of public words referenced
// by no doctest and no test file.
func testCommand(args []string, input io.Reader, output, diagnostics io.Writer) int {
	path, report, gaps := "", "text", false
	for _, arg := range args {
		switch {
		case arg == "--report=text" || arg == "--report=json":
			report = strings.TrimPrefix(arg, "--report=")
		case arg == "--gaps":
			gaps = true
		case strings.HasPrefix(arg, "--"):
			fmt.Fprintln(diagnostics, usage)
			return 2
		case path == "":
			path = arg
		default:
			fmt.Fprintln(diagnostics, usage)
			return 2
		}
	}
	if gaps && report == "json" {
		fmt.Fprintln(diagnostics, usage)
		return 2
	}
	root := "."
	if path != "" {
		root = path
	}
	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	var testFiles, doctestFiles []string
	if info.IsDir() {
		if testFiles, doctestFiles, err = discoverTestFiles(root); err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
	} else if strings.HasSuffix(path, ".test.zhex") {
		testFiles = []string{path}
	} else if strings.HasSuffix(path, ".zhex") {
		doctestFiles = []string{path}
	} else {
		fmt.Fprintf(diagnostics, "%s is not a .zhex file\n", path)
		return 1
	}
	if gaps {
		return testGaps(doctestFiles, testFiles, output, diagnostics)
	}
	passed, failed := 0, 0
	for _, file := range doctestFiles {
		if testDoctests(file, input, report, output, diagnostics) {
			passed++
		} else {
			failed++
		}
	}
	for _, file := range testFiles {
		if testFile(file, report, output, diagnostics) {
			passed++
		} else {
			failed++
		}
	}
	if report != "json" {
		fmt.Fprintf(output, "%d files: %d passed, %d failed\n", passed+failed, passed, failed)
	}
	if failed != 0 {
		return 1
	}
	return 0
}

// discoverTestFiles collects the *.test.zhex files under root and the plain
// .zhex files whose doctests join them, skipping the skippedFolders.
func discoverTestFiles(root string) (testFiles, doctestFiles []string, err error) {
	walk := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && skippedFolders[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, ".test.zhex"):
			testFiles = append(testFiles, path)
		case strings.HasSuffix(path, ".zhex"):
			doctestFiles = append(doctestFiles, path)
		}
		return nil
	})
	return testFiles, doctestFiles, walk
}

// testDoctests runs one file's ## > doctests; a file with none passes the
// same way a passing one does, and failures keep their full diagnostics.
func testDoctests(file string, input io.Reader, report string, output, diagnostics io.Writer) bool {
	source, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		reportFail(output, report, file, err.Error())
		return false
	}
	dir := filepath.Dir(file)
	failures := lang.RunDoctests(file, string(source), input, io.Discard, func(name string) (lang.ModuleSource, bool) {
		return readModule(dir, name)
	})
	if len(failures) == 0 {
		reportPass(output, report, file)
		return true
	}
	reportFail(output, report, file, firstLine(failures[0].Error()))
	for _, failure := range failures {
		if report == "json" {
			if jsonReport, ok := lang.Report(failure); ok {
				fmt.Fprintln(diagnostics, jsonReport)
				continue
			}
		}
		fmt.Fprintln(diagnostics, failure)
	}
	return false
}

// testFile runs one .test.zhex file as its own process: the file is a plain
// program, its exit code is the whole contract, and a crash cannot take the
// runner with it. The failing file's captured failure line joins the FAIL
// line; its stderr replays in full, or as its own JSON envelopes.
func testFile(file, report string, output, diagnostics io.Writer) bool {
	bin, err := testBinary()
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return false
	}
	args := []string{"run", file}
	if report == "json" {
		args = append(args, "--report=json")
	}
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	runErr := cmd.Run()
	var exit *exec.ExitError
	if runErr != nil {
		if !errors.As(runErr, &exit) {
			fmt.Fprintln(diagnostics, runErr)
			reportFail(output, report, file, runErr.Error())
			return false
		}
	} else {
		reportPass(output, report, file)
		return true
	}
	line := firstLine(errBuf.String())
	if line == "" {
		line = firstLine(outBuf.String())
	}
	if line == "" {
		line = fmt.Sprintf("exit status %d", exit.ExitCode())
	}
	reportFail(output, report, file, line)
	if report == "json" {
		if relayEnvelopes(errBuf.String(), diagnostics) {
			return false
		}
		failure := lang.TestFailure(file, fmt.Sprintf("test file failed with exit status %d: %s", exit.ExitCode(), line), "run zhex run "+file+" to see the failing check")
		if jsonReport, ok := lang.Report(failure); ok {
			fmt.Fprintln(diagnostics, jsonReport)
		}
		return false
	}
	fmt.Fprint(diagnostics, errBuf.String())
	return false
}

// relayEnvelopes passes the child's own JSON diagnostic lines through; a
// plain failed check prints none, so false means the runner synthesizes the
// test-failed envelope itself.
func relayEnvelopes(stderrText string, diagnostics io.Writer) bool {
	relayed := false
	for _, line := range strings.Split(stderrText, "\n") {
		if line != "" && json.Valid([]byte(line)) {
			fmt.Fprintln(diagnostics, line)
			relayed = true
		}
	}
	return relayed
}

// reportPass and reportFail print the per-file lines; --report=json keeps
// stdout empty and lets the failure envelopes on stderr carry the run.
func reportPass(output io.Writer, report, file string) {
	if report != "json" {
		fmt.Fprintf(output, "PASS %s\n", file)
	}
}

func reportFail(output io.Writer, report, file, detail string) {
	if report != "json" {
		fmt.Fprintf(output, "FAIL %s: %s\n", file, detail)
	}
}

// firstLine returns the first non-empty line — the stable first line of a
// diagnostic, or the label a failed check printed.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			return line
		}
	}
	return ""
}

// testGaps lists every public word referenced by no doctest and no test
// file: a word's own doc doctests it, and a test file references it by its
// qualified module name. The list is advisory, but a file that fails to
// compile is reported and fails the run.
func testGaps(doctestFiles, testFiles []string, output, diagnostics io.Writer) int {
	var references strings.Builder
	for _, file := range testFiles {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
		references.Write(source)
	}
	covered := references.String()
	failed := 0
	for _, file := range doctestFiles {
		source, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			failed++
			continue
		}
		dir := filepath.Dir(file)
		words, err := lang.PublicWords(file, string(source), func(name string) (lang.ModuleSource, bool) {
			return readModule(dir, name)
		})
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			failed++
			continue
		}
		module := strings.TrimSuffix(filepath.Base(file), ".zhex")
		for _, w := range words {
			// shortcut: a comment mentioning module.word counts as a reference; match on text when gaps lie
			if w.Doctested || containsWordReference(covered, module+"."+w.Name) {
				continue
			}
			fmt.Fprintf(output, "%s: word %s has no doctest and no test file\n", file, w.Name)
		}
	}
	if failed != 0 {
		return 1
	}
	return 0
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

// containsWordReference reports whether ref appears in text bounded by non-identifier runes.
func containsWordReference(text, ref string) bool {
	offset := 0
	for {
		idx := strings.Index(text[offset:], ref)
		if idx < 0 {
			return false
		}
		at := offset + idx
		after := at + len(ref)
		beforeOk := at == 0 || (!isIdentChar(text[at-1]) && text[at-1] != '.')
		afterOk := after >= len(text) || !isIdentChar(text[after])
		if beforeOk && afterOk {
			return true
		}
		offset = at + 1
	}
}
