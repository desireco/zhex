// Heat-style double-buffer diffusion on a 400x400 grid for 10 generations.
// Cells start at (r*131 + c*137) % 100; borders stay fixed; interior cells
// become (4*cur + four neighbours) / 8. Prints the final cell sum.
package main

import "fmt"

const size = 400
const generations = 10

func main() {
	current := make([][]int64, size)
	following := make([][]int64, size)
	for r := 0; r < size; r++ {
		current[r] = make([]int64, size)
		following[r] = make([]int64, size)
		for c := 0; c < size; c++ {
			current[r][c] = int64((r*131 + c*137) % 100)
		}
	}
	last := size - 1
	for g := 0; g < generations; g++ {
		for r := 0; r < size; r++ {
			for c := 0; c < size; c++ {
				if r == 0 || r == last || c == 0 || c == last {
					following[r][c] = current[r][c]
				} else {
					following[r][c] = (current[r][c]*4 + current[r-1][c] + current[r+1][c] + current[r][c-1] + current[r][c+1]) / 8
				}
			}
		}
		current, following = following, current
	}
	var total int64
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			total += current[r][c]
		}
	}
	fmt.Println(total)
}
