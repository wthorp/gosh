//go:build ignore
// +build ignore

package main

import "github.com/wthorp/gosh"

// RunAgentic falls back to Codex for lines that are not Gosh commands
// or executable programs.
func main() {
	gosh.RunAgentic(`
		set yinz = World
		display hello ${yinz} on the screen
		git diff
	`)
}
