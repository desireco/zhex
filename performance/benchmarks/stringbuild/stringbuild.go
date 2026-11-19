// Build 200k numbered lines, then print the joined text's length and how
// many times "42" occurs across the lines (non-overlapping, per line).
package main

import (
	"fmt"
	"strings"
)

func main() {
	var b strings.Builder
	hits := 0
	for i := 0; i < 200000; i++ {
		line := "Line " + fmt.Sprint(i) + ": the quick brown fox jumps over the lazy dog"
		hits += strings.Count(line, "42")
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	text := b.String()
	fmt.Println(len(text))
	fmt.Println(hits)
}
