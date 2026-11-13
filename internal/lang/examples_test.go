package lang

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	cases := []struct {
		name, input, want string
		args              []string
		wantErr           string
		exited            bool
		exitCode          int
	}{
		{name: "records", want: "Ada, age 36\n"},
		{name: "collections", want: "3\nGrace\n0: Ada\ncollection-iterating\n1: Grace\ncollection-iterating\n2: Lin\ncollection-iterating\n0,0: 1\n0,1: 2\n0,2: 3\n1,0: 4\n1,1: 5\n1,2: 6\n6\n7\n12\n"},
		{name: "composition", want: "32\n"},
		{name: "exit-status", input: "0\n", want: "Exit status (0..255): Exiting with status 0\n", exited: true, exitCode: 0},
		{name: "exit-status", input: "oops\n", want: "Exit status (0..255): Invalid status: text-to-int\n", exited: true, exitCode: 2},
		{name: "exit-status", input: "256\n", want: "Exit status (0..255): Status must be from 0 to 255.\n", exited: true, exitCode: 2},
		{name: "recovery", want: "Handled text-to-int\nReported text-to-int\nThe retry count must be a decimal integer; using 3.\n3\n"},
		{name: "hello", input: "Ada\n", want: "Your name: Hello, Ada!\n"},
		{name: "hello", input: "\n", want: "Your name: Hello, !\n"},
		{name: "hello", input: " Ada \r\n", want: "Your name: Hello,  Ada !\n"},
		{name: "hello", input: "Ada", want: "Your name: Hello, Ada!\n"},
		{name: "fizzbuzz", input: "15\n", want: "FizzBuzz up to (1..10000): 1\n2\nFizz\n4\nBuzz\nFizz\n7\n8\nFizz\nBuzz\n11\nFizz\n13\n14\nFizzBuzz\n"},
		{name: "fizzbuzz", input: "1\n", want: "FizzBuzz up to (1..10000): 1\n"},
		{name: "fizzbuzz", input: "0\n", want: "FizzBuzz up to (1..10000): Enter a number from 1 to 10000.\n"},
		{name: "fibonacci", input: "0\n", want: "Fibonacci index (0..92): 0\n"},
		{name: "fibonacci", input: "1\n", want: "Fibonacci index (0..92): 1\n"},
		{name: "fibonacci", input: "2\n", want: "Fibonacci index (0..92): 1\n"},
		{name: "fibonacci", input: "10\n", want: "Fibonacci index (0..92): 55\n"},
		{name: "fibonacci", input: "92\n", want: "Fibonacci index (0..92): 7540113804746346429\n"},
		{name: "fibonacci", input: "93\n", want: "Fibonacci index (0..92): Enter an index from 0 to 92.\n"},
		{name: "gcd", input: "48\n18\n", want: "First number: Second number: 6\n"},
		{name: "gcd", input: "0\n0\n", want: "First number: Second number: 0\n"},
		{name: "gcd", input: "-4\n18\n", want: "First number: Second number: Enter two nonnegative integers.\n"},
		{name: "times-table", input: "7\n", want: "Table for: 7 x 1 = 7\n7 x 2 = 14\n7 x 3 = 21\n7 x 4 = 28\n7 x 5 = 35\n7 x 6 = 42\n7 x 7 = 49\n7 x 8 = 56\n7 x 9 = 63\n7 x 10 = 70\n"},
		{name: "times-table", input: "12\n", want: "Table for: 12 x 1 = 12\n12 x 2 = 24\n12 x 3 = 36\n12 x 4 = 48\n12 x 5 = 60\n12 x 6 = 72\n12 x 7 = 84\n12 x 8 = 96\n12 x 9 = 108\n12 x 10 = 120\n"},
		{name: "times-table", input: "0\n", want: "Table for: Enter a number from 1 to 12.\n"},
		{name: "times-table", input: "13\n", want: "Table for: Enter a number from 1 to 12.\n"},
		{name: "factorial", input: "0\n", want: "Factorial of (0..20): 1\n"},
		{name: "factorial", input: "1\n", want: "Factorial of (0..20): 1\n"},
		{name: "factorial", input: "5\n", want: "Factorial of (0..20): 120\n"},
		{name: "factorial", input: "20\n", want: "Factorial of (0..20): 2432902008176640000\n"},
		{name: "factorial", input: "-1\n", want: "Factorial of (0..20): Enter a number from 0 to 20.\n"},
		{name: "factorial", input: "21\n", want: "Factorial of (0..20): Enter a number from 0 to 20.\n"},
		{name: "palindrome", input: "racecar\n", want: "Text to check: Palindrome\n"},
		{name: "palindrome", input: "abba\n", want: "Text to check: Palindrome\n"},
		{name: "palindrome", input: "été\n", want: "Text to check: Palindrome\n"},
		{name: "palindrome", input: "\n", want: "Text to check: Palindrome\n"},
		{name: "palindrome", input: "hello\n", want: "Text to check: Not a palindrome\n"},
		{name: "palindrome", input: "Racecar\n", want: "Text to check: Not a palindrome\n"},
		// text-stats fixture: 3 lines, 11 words, 64 runes (61 letters, spaces,
		// and punctuation plus 3 newlines; "café" shows rune-safe counting).
		{name: "text-stats", args: []string{"../../examples/text-stats-sample.txt"}, want: "3 lines  11 words  64 chars\n"},
		{name: "text-stats", args: []string{"../../examples/text-stats-sample.txt", "upper"}, want: "ZHEX COUNTS WORDS, LINES, AND CHARACTERS.\nTHE CAFÉ IS OPEN!\nBYE\n"},
		{name: "text-stats", wantErr: "argument-missing"},
		{name: "text-stats", args: []string{"../../examples/no-such-file.txt"}, wantErr: "file-open"},
		{name: "modules/main", input: "48\n18\n", want: "First number: Second number: 30\n6\n144\n"},
		{name: "modules/main", input: "0\n5\n", want: "First number: Second number: Enter two positive integers.\n"},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s/%d", tc.name, i), func(t *testing.T) {
			path := "../../examples/" + tc.name + ".zhex"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Modules resolve as sibling files next to the program, the
			// same rule the CLI applies; single-file examples never call it.
			dir := filepath.Dir(path)
			resolve := func(name string) (ModuleSource, bool) {
				source, err := os.ReadFile(filepath.Join(dir, name+".zhex"))
				if err != nil {
					return ModuleSource{}, false
				}
				return ModuleSource{Path: filepath.Join(dir, name+".zhex"), Source: string(source)}, true
			}
			var out bytes.Buffer
			err = RunWithModules(path, string(src), strings.NewReader(tc.input), &out, nil, tc.args, resolve)
			if tc.exited {
				exit, ok := err.(*ExitError)
				if !ok || exit.Status != tc.exitCode || out.String() != tc.want {
					t.Fatalf("exit %v, output %q", err, out.String())
				}
				return
			}
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("wanted %s diagnostic, program succeeded", tc.wantErr)
				}
				if code, message := codeOf(t, err); code != tc.wantErr {
					t.Fatalf("wanted %s, got %s: %q", tc.wantErr, code, message)
				}
				if out.String() != "" {
					t.Fatalf("partial output before diagnostic: %q", out.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Fatalf("got %q\nwant %q", out.String(), tc.want)
			}
		})
	}
}

func TestInputFailures(t *testing.T) {
	for _, input := range []string{"", "\n", "  \n", "abc\n", "1.5\n", "9223372036854775808\n", "-9223372036854775809\n", "\xff\n"} {
		t.Run(fmt.Sprintf("%q", input), func(t *testing.T) {
			var out bytes.Buffer
			err := Run("input.zhex", "let n = text.to-int(read-line(\"Number: \") try) try\nprint(n)\n", strings.NewReader(input), &out)
			if err == nil || !strings.Contains(err.Error(), "input.zhex:1:") {
				t.Fatalf("expected input diagnostic, got %v", err)
			}
			if out.String() != "Number: " {
				t.Fatalf("unexpected output %q", out.String())
			}
		})
	}
}
