//+build ignore

package main

import (
	"fmt"

	"github.com/wthorp/gosh"
)

func main() {
	// gosh.Menu() is display usage information if this
	// program is run without parameters.
	gosh.Menu()
}

// Use gosh.Register with an exported function to
// allow it to be called from the command line.
var _ = gosh.Register(HelloGoshAndGo)

// HelloGoshAndGo is meant to be called from the CLI.
func HelloGoshAndGo(name string) {
	gosh.Run(`
		echo Hello ${name} from Gosh!
		HelloGo ${name}
	`)
}

// gosh.Func() can also register anonymous functions for
// use via command line.  Simple capitalize the name.
var _ = gosh.Func("HelloGo", func(name string) {
	fmt.Println("Welcome!")
	fmt.Printf("Hello %s directly from Go!\n", name)
})

// gosh.Func() can also be used to hide exported commands
// from the CLI, while leaving them accessible from script.
var _ = gosh.Func("secret", PoorlyGuardedSecret)

// PoorlyGuardedSecret could be from a 3rd party code.
func PoorlyGuardedSecret() {
	fmt.Println("Shh!  The secret word is 'gosh'!")
}
