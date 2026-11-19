// Word frequencies over the generated corpus, printing the top ten by
// count descending with word-ascending ties. The distinct-word slice keeps
// first-seen order so map iteration order never reaches the output.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type entry struct {
	word  string
	count int
}

func main() {
	data, err := os.ReadFile("inputs/wordcount.txt")
	if err != nil {
		panic(err)
	}
	counts := make(map[string]int)
	var distinct []string
	for _, line := range strings.Split(string(data), "\n") {
		for _, w := range strings.Split(line, " ") {
			if len(w) == 0 {
				continue
			}
			if _, seen := counts[w]; !seen {
				distinct = append(distinct, w)
			}
			counts[w]++
		}
	}
	entries := make([]entry, 0, len(distinct))
	for _, w := range distinct {
		entries = append(entries, entry{w, counts[w]})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].word < entries[j].word
	})
	for _, e := range entries[:10] {
		fmt.Println(e.word, e.count)
	}
}
