package main

import (
	"fmt"

	"github.com/wthorp/gosh"
)

func main() {
	gosh.Autowire()
}

// GoodStuff is target that shows how to use gosh.Run().
func GoodStuff() {
	gosh.Run(`
	# be polite
	set yinz = World
	echo Hello ${yinz}
	goHello
	
	mkdir test

	// run some executable programs
	date
	git status
	ls

	rmdir test
	echo Goodbye ${yinz}
	`, gosh.Calls{"goHello": goHello})
}

// goHello is a gosh.Call() that says hello from Go.
func goHello(*gosh.Block, string) error {
	fmt.Println(" ... and hello from Go!")
	return nil
}

// BoringHello is a target that doesn't use gosh.Run().
func BoringHello(name string) {
	fmt.Printf(" ... and hello %s from Go!\n", name)
}
