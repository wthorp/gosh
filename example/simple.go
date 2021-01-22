//+build ignore

package main

import "github.com/wthorp/gosh"

// The simplest gosh scripts only call gosh.Run().
func main() {
	gosh.Run(`
		# set and echo are pre-registered gosh commands; 
		# they're simple Go functions
		set name = gosh
		echo Hello from ${name}!

		echo 
		echo Here are your files:
		# command line executables are supported as well
		ls .
	`)
}
