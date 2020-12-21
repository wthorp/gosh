package main

func main() {
	NewBlock(`
	# be polite
	echo Hello World
	mkdir test

	// run some executable programs
	date
	git status
	ls

	# okay, I was kidding about being polite
	rmdir test
	echo Goodbye Cruel World
	`).Run()
}
