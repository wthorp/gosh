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

## Using Multiple Script support

Gosh automatically creates CLI mapping, supporting multiple targets or recipes in a single file:

```
func main() {	
	gosh.MultiTarget()
}
```

This will display usage information and available targets if run without command-line parameters.

See the [examples directory](./tree/main/example) to get a better feel for usage.

## Design

Gosh allows interaction with Go functions in one of two ways:

1. Calling Go functions from Gosh scripts within Go code
2. Calling Go functions within Go code via the command line (CLI)

Note the assumumption that there are no functions which Gosh scripts _must not_ call; mapping some or all functions are currently equally valid solutions.  The notable options to automatically map Go functions with their string names include:

1. Programatically create mapping from source code at runtime
    - Find the source code, alter it, compile it, and run it again
    - Requires only "safe" Go code, though the idea itself feels questionable
    - Always requires Go source, or would require a special command to build and executable
    - Allows running unexported Go functions, making uppercase the intuitive way to define CLI calls 
    - Allows access to code comments for reporting CLI usage
2. Programatically create mapping via reflection at runtime
    - Only exported receivers can be discovered via reflection
      - some "less inuitive than capitalization" way must define CLI-callable code
      - users must define a custom struct to support their receivers
    - Requires only "safe" Go code
    - Does not allow access to code comments for reporting CLI usage
3. Programatically create mapping via linker and runtime data
    - Use very "unsafe" tricks can access function names from linker data
      - `//go:linkname Firstmoduledata runtime.firstmoduledata`
    - Allows running unexported Go functions, making uppercase the intuitive way to define CLI calls
      - Does it though?  Would the linker ever remove an inlined, unexported function?  a receiver?
    - Does not allow access to code comments for reporting CLI usage
 
### Current non-goals:
 - returning error codes when used with `go run`
   - its known that `go run` returns the error code from compiling, not running a Go program
   - to return error levels correctly, first compile then run the code
 - declarative dependencies (like Make)
   - its assumed that calling Go functions covers the 85% use case
   - `sync.Once()` and infinite-loop checks are easy imperative fixes
 - differentiating which functions cannot be called from GoSh script
   - it must linked into the Go code, and mapped so it can be resolved by name
   - if `time.Now()` could be wired up there's likely no strong reason to prevent it
   - if `time.utcTime()`, unexported, were somehow wired up... well that's okay too
 - support for calling arbitraty function in external modules
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
