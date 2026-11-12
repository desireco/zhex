package lang

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExamples(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{"composition", "", "32\n"},
		{"hello", "Ada\n", "Your name: Hello, Ada!\n"},
		{"hello", "\n", "Your name: Hello, !\n"},
		{"hello", " Ada \r\n", "Your name: Hello,  Ada !\n"},
		{"hello", "Ada", "Your name: Hello, Ada!\n"},
		{"fizzbuzz", "15\n", "FizzBuzz up to (1..10000): 1\n2\nFizz\n4\nBuzz\nFizz\n7\n8\nFizz\nBuzz\n11\nFizz\n13\n14\nFizzBuzz\n"},
		{"fizzbuzz", "1\n", "FizzBuzz up to (1..10000): 1\n"},
		{"fizzbuzz", "0\n", "FizzBuzz up to (1..10000): Enter a number from 1 to 10000.\n"},
		{"fibonacci", "0\n", "Fibonacci index (0..92): 0\n"},
		{"fibonacci", "1\n", "Fibonacci index (0..92): 1\n"},
		{"fibonacci", "2\n", "Fibonacci index (0..92): 1\n"},
		{"fibonacci", "10\n", "Fibonacci index (0..92): 55\n"},
		{"fibonacci", "92\n", "Fibonacci index (0..92): 7540113804746346429\n"},
		{"fibonacci", "93\n", "Fibonacci index (0..92): Enter an index from 0 to 92.\n"},
		{"factorial", "0\n", "Factorial of (0..20): 1\n"},
		{"factorial", "1\n", "Factorial of (0..20): 1\n"},
		{"factorial", "5\n", "Factorial of (0..20): 120\n"},
		{"factorial", "20\n", "Factorial of (0..20): 2432902008176640000\n"},
		{"factorial", "-1\n", "Factorial of (0..20): Enter a number from 0 to 20.\n"},
		{"factorial", "21\n", "Factorial of (0..20): Enter a number from 0 to 20.\n"},
		{"palindrome", "racecar\n", "Text to check: Palindrome\n"},
		{"palindrome", "abba\n", "Text to check: Palindrome\n"},
		{"palindrome", "été\n", "Text to check: Palindrome\n"},
		{"palindrome", "\n", "Text to check: Palindrome\n"},
		{"palindrome", "hello\n", "Text to check: Not a palindrome\n"},
		{"palindrome", "Racecar\n", "Text to check: Not a palindrome\n"},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s/%d", tc.name, i), func(t *testing.T) {
			path := "../../examples/" + tc.name + ".zhex"
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := Run(path, string(src), strings.NewReader(tc.input), &out); err != nil {
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
			err := Run("input.zhex", "let n = console.read-int(\"Number: \") try\nprint(n)\n", strings.NewReader(input), &out)
			if err == nil || !strings.Contains(err.Error(), "input.zhex:1:") {
				t.Fatalf("expected input diagnostic, got %v", err)
			}
			if out.String() != "Number: " {
				t.Fatalf("unexpected output %q", out.String())
			}
		})
	}
}
