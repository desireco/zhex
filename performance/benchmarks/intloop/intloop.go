// Euclid's gcd by remainder over 100k pairs from an in-program Park-Miller
// generator (state * 48271 % 2147483647, seeded 20260119); prints the gcd sum.
package main

import "fmt"

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func main() {
	state := int64(20260119)
	total := int64(0)
	for i := 0; i < 100000; i++ {
		state = state * 48271 % 2147483647
		a := state%999983 + 1
		state = state * 48271 % 2147483647
		b := state%999979 + 1
		total += gcd(a, b)
	}
	fmt.Println(total)
}
