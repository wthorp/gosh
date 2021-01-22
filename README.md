# GoSh

GoSh makes it easy to write command-running scripts from Go code:
 - author multiline, easy to write, command-running scripts from Go code
 - invoke different scripts from the same file, similar to Make or Just
 - run scripts concurrently without working directory or environment contention

## GoSh Scripts

The basis of GoSh is that it can run multi-line scripts of code.  
```
gosh.Run(`
	set yinz = World
	echo Hello ${yinz}
	git diff
`)
```

## GoSh Commands

GoSh has the following shell-like commands built in:

 - cd : change the working directory
 - echo : write to the console
 - mkdir : create a directory
 - pushd : change the working directory, remembering the old
 - popd : restore the last remembered directory
 - rm : remove a file
 - rmdir : remove a directory
 - set : save text as a variable

It's easy to add your own:
```
var _ = gosh.Register(helloWorld)
```
You can also register anoymous functions, 3rd party code, or set custom names:
```
var _ = gosh.Cmd("helloWorld", func(who string) { ... })
```

Commands are invoked in a case insensitive manner. However, when registering a function, initial 
letter capitalization means that the function will be exposed via the command line.

## Gosh Menu

Gosh supports invoking exposed commands from the CLI, allowing multiple commands to 
coexist in the same file.  Running `gosh.Menu()` will either display usage information 
or invoke the requested CLI parameters.

```
func main() {	
	gosh.Menu()
}
```

See the [examples directory](./example) to get a better feel for usage.

### Current non-goals:
 - returning error codes when used with `go run`
   - its known that `go run` returns the error code from compiling, not running a Go program
   - to return error levels correctly, first compile then run the code
 - declarative dependencies (like Make)
   - its assumed that calling Go functions covers the 85% use case
   - `sync.Once()` and infinite-loop checks are easy imperative fixes

## Todos:
 - clean up script / CLI / param binding
 - expose function params as script variables by name
 - write tests
 - add support for non-local targets like Docker or SSH
 - see if there's a way to hack `go run` at runtime to return Gosh error codes

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
