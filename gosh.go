package main

func main() {
	NewBlock(`
	# be polite
	set yinz = World
	echo Hello ${yinz}
	
	mkdir test

	// run some executable programs
	date
	git status
	ls

	rmdir test
	echo Goodbye ${yinz}
	`).Run()
}
