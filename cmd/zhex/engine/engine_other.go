//go:build !js

package main

// The engine exists only as the js build (engine.go); this placeholder keeps
// plain `go build ./...` green on other builds instead of failing on a
// missing func main.
func main() {}
