package main

func main() {
	NewBlock(`
	# be polite
	echo Hello World

	// run some executable programs
	date
	git status

	# okay, I was kidding about being polite
	echo Goodbye Cruel World
	`).Run()
}
