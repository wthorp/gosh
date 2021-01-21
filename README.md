# GoSh

GoSh makes it easy to write command-running scripts from Go code:
 - author multiline, easy to write, command-running scripts from Go code
 - invoke different scripts from the same file, similar to Make or Just
 - run scripts concurrently without working directory or environment contention

## Using GoSh Scripts

The basis of GoSh is that it can run multi-line blocks of code.  
```
func main() {	
	gosh.Run(`
		set yinz = World
		echo Hello ${yinz}
		git diff
	`)
}
```

GoSh has the following Shell-like commands built in, but it's easy to add your own:

 - cd : change the working directory
 - echo : write to the console
 - mkdir : create a directory
 - pushd : change the working directory, remembering the old
 - popd : restore the last remembered directory
 - rm : remove a file
 - rmdir : remove a directory
 - set : save text as a variable

You can register custom functions with Gosh:

```
var _ = gosh.Register(helloWorld)
```
Alternatively, gosh.Func() can registers anoymous functions or existing functions using custom names:
```
var _ = gosh.Func("helloWorld", func(who string) { ... })
```

Commands may be invoked in a case insensitive manner, however when registering a function, initial 
capitalization means that the function will be callable via the command line.

## Using Multiple Script support

Gosh automatically creates CLI mapping, supporting multiple exposed functions in a single file:

```
func main() {	
	gosh.Menu()
}
```

This will display usage information and available targets if run without command-line parameters.

See the [examples directory](./example) to get a better feel for usage.

### Current non-goals:
 - returning error codes when used with `go run`
   - its known that `go run` returns the error code from compiling, not running a Go program
   - to return error levels correctly, first compile then run the code
 - declarative dependencies (like Make)
   - its assumed that calling Go functions covers the 85% use case
   - `sync.Once()` and infinite-loop checks are easy imperative fixes
 - support for calling arbitrary functions in external modules
   - if external function needs to be supported, it can be wrapped in an appropriate local function 
  

## Todos:
 - combining all Call and Target logic
 - autowire function params into script variables
 - write more tests
 - more research on not adding cruft to modules
 - add support for non-local targets like Docker or SSH
 - look at [Just](https://github.com/casey/just)
 - see if there's a way to hack `go run` at runtime to return results correctly

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
