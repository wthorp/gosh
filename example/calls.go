//+build ignore

package main

import (
	"fmt"

	"github.com/wthorp/gosh"
)

func main() {
	gosh.Run(`
		set yinz = gosh
		goHello ${yinz}
		count3
		count2
		count1
		goOdBye ${yinz}
	`)
}

// One method of registering code for scripts to run
// is to use gosh.Register with an existing function.
var _ = gosh.Register(goHello)

func goHello(who string) {
	fmt.Printf("Hello %s\n", who)
}

// gosh.Register may be called once per function, or
// can be called with multiple functions at once.
var _ = gosh.Register(count1, count2, count3)

func count1() {
	fmt.Println(1)
}
func count2() {
	fmt.Println(1)
}
func count3() {
	fmt.Println(1)
}

// An alternative to gosh.Register is gosh.Func,
// which uses a string name and an anonymous function.
var _ = gosh.Func("goOdBye", func(who string) {
	fmt.Printf("Hello %s\n", who)
})
