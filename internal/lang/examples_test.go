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
		wantStderr        string // console.fail output, checked when non-empty
		tempFixture       string // written to a temp file appended to args
	}{
		{name: "partials", want: "12\n12\n12\n1\n1\n12\naDa\n7\n1\n8\ntext-to-int\n3\n"},
		{name: "calls", want: "Hello, Ada\nHello, Grace\n3\n3\n7\n2\n1\n3\n"},
		{name: "word-defaults", want: "Hello Grace\ndefault ran\nHello Ada\n1\n1\n"},
		{name: "line-filter", want: "shipped: inv-1042, inv-1044\n"},
		{name: "score-report", want: "Ada: 36 / Grace: 9 / Lin: 41 / Nancy: 23\npassing: 3 of 4\ntotal: 100\nfirst above 40: 41\nsomeone is above 40, but not everyone\n"},
		{name: "sorting", want: "Grace: 23\nNancy: 23\nAda: 41\nLin: 41\nsource unchanged: Ada: 41\nsorted: apple, fig, pear\n"},
		{name: "file-write", want: "Save failed: file-write\nNothing was written; the report stays in memory.\nIn-memory report:\ninvoices: 2\ncredits: 7\n\n"},
		{name: "paths", want: "/tmp/zhex-out/june/invoices.csv\n/tmp/zhex-out/june\ninvoices.csv\n.csv\n/var/run/state.json\n/tmp/archive/old.csv\n"},
		{name: "collatz", want: "1: 0\n2: 1\n3: 7\n4: 2\n5: 5\n6: 8\n7: 16\n8: 3\n9: 19\n"},
		// file-work takes a base folder as its first argument and removes the
		// zhex-work tree it creates, so the output is deterministic.
		{name: "file-work", args: []string{filepath.Join(t.TempDir(), "base")}, want: "work folder: credits.txt, invoices.txt\ninvoices.txt: 12 bytes\nvolume still has space\nwork folder removed\n"},
		{name: "file-work", wantErr: "argument-missing"},
		// env-config writes its own .env fixture into the base folder and
		// removes it, so the output is deterministic.
		{name: "env-config", args: []string{filepath.Join(t.TempDir(), "base")}, want: "title: Weekly status\nmode: set but empty\nlimit: not in the file\nreport configured\n"},
		{name: "temporal", want: "2024-03-10T12:00:00-04:00[America/New_York]\n2024-03-10T13:00:00-04:00[America/New_York]\n"},
		{name: "json-config", want: "3\ntrue\n{\"retries\":3,\"verbose\":true}\n"},
		{name: "json-tree", want: "nodes: 12\nservice = ingest\nretries = 1.5e1\nendpoints = list of 4\nflags = object of 1\n{\"service\":\"ingest\",\"retries\":1.5e1,\"endpoints\":[\"primary\",443,null,{\"host\":\"db\",\"port\":5432}],\"flags\":{\"dry-run\":true}}\n"},
		{name: "xml-config", want: "ingest\n3\ntrue\n2\ndb:5432\nowner unset\n"},
		{name: "maps", want: "1\n3\nAda: 11\nMorgan has no score\nAda: 11\nGrace: 9\nNancy: 23\n4\n4\n"},
		{name: "life", want: "Generation 0\n.......\n.......\n...#...\n....#..\n..###..\n.......\n.......\nGeneration 1\n.......\n.......\n.......\n..#.#..\n...##..\n...#...\n.......\nGeneration 2\n.......\n.......\n.......\n....#..\n..#.#..\n...##..\n.......\nGeneration 3\n.......\n.......\n.......\n...#...\n....##.\n...##..\n.......\nGeneration 4\n.......\n.......\n.......\n....#..\n.....#.\n...###.\n.......\n"},
		{name: "edit-distance", want: "kitten → sitting: 3\nempty → abc: 3\ncafé → cafe: 1\n"},
		{name: "choices", want: "running\nCould not start: network unavailable\n7\n0\n"},
		{name: "records", want: "Ada, age 36\n"},
		{name: "collections", want: "3\nGrace\n3\n0: Ada\ncollection-iterating\n1: Grace\ncollection-iterating\n2: Lin\ncollection-iterating\n2\nLin\n1\n5\n0,0: 1\n0,1: 2\n0,2: 3\n1,0: 4\n1,1: 5\n1,2: 6\n6\n7\n12\n"},
		{name: "composition", want: "32\n"},
		{name: "with-blocks", want: "he\n3\n"},
		{name: "exit-status", input: "0\n", want: "Exit status (0..255): Exiting with status 0\n", exited: true, exitCode: 0},
		{name: "exit-status", input: "oops\n", want: "Exit status (0..255): Invalid status: text-to-int\n", exited: true, exitCode: 2},
		{name: "exit-status", input: "256\n", want: "Exit status (0..255): Status must be from 0 to 255.\n", exited: true, exitCode: 2},
		{name: "recovery", want: "That is not a whole number. I'll use 3 retries.\nRetries: 3\n"},
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
		// filter is the line-filter tool: literal case-sensitive substring
		// match, matching lines verbatim, usage on wrong arg count, and the
		// natural FileError for a missing file.
		{name: "filter", tempFixture: "alpha one\nBeta two\ngamma alpha\nplain\nALPHA three\n", args: []string{"alpha"}, want: "alpha one\ngamma alpha\n"},
		{name: "filter", tempFixture: "alpha one\nBeta two\ngamma alpha\nplain\nALPHA three\n", args: []string{"ALPHA"}, want: "ALPHA three\n"},
		{name: "filter", tempFixture: "alpha one\n", args: []string{"zzz"}, want: ""},
		{name: "filter", args: []string{"alpha"}, exited: true, exitCode: 1, wantStderr: "usage: zhex run filter.zhex <pattern> <file>\n"},
		{name: "filter", args: []string{"alpha", "../../examples/no-such-file.txt"}, wantErr: "file-open"},
		{name: "modules/main", input: "48\n18\n", want: "First number: Second number: 30\n6\n144\n"},
		{name: "modules/main", input: "0\n5\n", want: "First number: Second number: Enter two positive integers.\n"},
		// math.test is the zhex test seed: the .test.zhex file a plain
		// `zhex run` executes and the `zhex test` runner discovers.
		{name: "modules/math.test", want: "math module tests passed\n"},
		{name: "modules/md-report/main", want: "## Weekly status \\& latency\n\nThree services reported\\; nothing is down\\.\n\n| service            | region   | latency |\n| ------------------ | -------- | ------- |\n| edge\\-cache        | eu\\-west | 12 ms   |\n| search\\-api \\(v2\\) | eu\\-west | 41 ms   |\n| auth               | us\\-east | 87 ms   |\n\n- no renderer\n- no template engine\\, just words\n\n[CommonMark](https://commonmark.org/help)\n\n```zhex\nlet title = md.heading(2, \"Weekly\")\nflow `inline code`\n    text.upper\n```\n"},
		// html-page composes the html sibling module: escaped text by
		// default, markup through element and one deliberate html.raw,
		// and a rejected javascript: link target.
		{name: "modules/html-page/main", want: "<!DOCTYPE html>\n<html>\n<head>\n<title>Zhex services — café &amp; &lt;status&gt;</title>\n</head>\n<body>\n<p>Everything below is escaped unless it arrives through html.raw.</p>\n<ul class=\"status &#34;live&#34; &amp; &lt;fresh&gt;\">\n<li>api: 2 ms</li>\n<li>web: 9 ms</li>\n<li>db: 41 ms &amp; &lt;degraded&gt;</li>\n</ul>\n<hr>\n<a href=\"https://example.io/zhex\">project home</a> / (link rejected: javascript: is not an allowed scheme)\n</body>\n</html>\n"},
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
			args := tc.args
			if tc.tempFixture != "" {
				fixture := filepath.Join(t.TempDir(), "fixture.txt")
				if err := os.WriteFile(fixture, []byte(tc.tempFixture), 0o644); err != nil {
					t.Fatal(err)
				}
				args = append(args, fixture)
			}
			var stderr bytes.Buffer
			err = RunWithModulesOptions(path, string(src), strings.NewReader(tc.input), &out, RunOptions{Args: args, Diagnostics: &stderr}, resolve)
			if tc.exited {
				exit, ok := err.(*ExitError)
				if !ok || exit.Status != tc.exitCode || out.String() != tc.want {
					t.Fatalf("exit %v, output %q", err, out.String())
				}
				if tc.wantStderr != "" && stderr.String() != tc.wantStderr {
					t.Fatalf("stderr %q, want %q", stderr.String(), tc.wantStderr)
				}
				return
			}
			if tc.wantStderr != "" && stderr.String() != tc.wantStderr {
				t.Fatalf("stderr %q, want %q", stderr.String(), tc.wantStderr)
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
	for _, input := range []string{"", "\n", "  \n", "abc\n", "1.5\n", "170141183460469231731687303715884105728\n", "-170141183460469231731687303715884105729\n", "\xff\n"} {
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
