package main

import (
	"fmt"

	"github.com/wthorp/gosh"
)

func main() {
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

func goHello(*gosh.Block, string) error {
	fmt.Println(" ... and hello from Go!")
	return nil
}
