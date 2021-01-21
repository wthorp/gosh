//+build ignore

package main

import "github.com/wthorp/gosh"

// The simplest gosh scripts only call gosh.Run().
func main() {
	gosh.Run(`
		# set and echo are pre-registered gosh calls,
		# they are actually just Go code.
		set yinz = gosh
		echo Hello ${yinz}
		
		# command line executables are supported as well
		git clone https://github.com/wthorp/gosh.git
	`)
}
