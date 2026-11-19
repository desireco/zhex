// Print stdin lines containing "ERROR", streaming until end of input.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "ERROR") {
			fmt.Fprintln(out, line)
		}
	}
}
