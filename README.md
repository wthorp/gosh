# GoSh

GoSh is a command-runner for Go, designed as a simpler alternative to bash/make scripts.

## Design

### Goals:
 - mix multiline, easy to write program execution with standard Go code execution 
 - allow multiple scripts to be exist in the same file, similar to make
 - allow scripts to run concurrently without working directory concerns

## The design crux:

Gosh allows interaction with Go functions in one of two ways:

1. Calling Go functions from Gosh scripts within Go code
2. Calling Go functions from Go progams via the command line (CLI), either using compiled executables or `go run`

Note the assumumption that there are no functions which Gosh scripts _must not_ call; mapping some or all functions are currently equally valid solutions.  The notable options to automatically map Go functions with their string names include:

1. Programatically create mapping from source code at runtime
    - Find the source code, alter it, compile it, and run it again
    - Requires only "safe" Go code, though the idea itself feels questionable
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
  
## Using GoSh Scripts

The basis of GoSh is that it can run multi-line blocks of code.  
```
func main() {	
	gosh.Run(``
	set yinz = World
	echo Hello ${yinz}
	``)
}
```

The built-in scripting support is deliberately low on features, focused mostly on working directory support.  This allows multiple execution environments to be run in parallel.  GoSh has the following Shell-like commands built in, but it's easy to add your own:

 - cd : change the working directory
 - echo : write to the console
 - mkdir : create a directory
 - pushd : change the working directory, remembering the old
 - popd : restore the last remembered directory
 - rm : remove a file
 - rmdir : remove a directory
 - set : save text as a variable

See the (examples directory)[./tree/main/example] to get a better feel for usage.

## Using Multiple Script support

Its convenient when working in a new code-base, to understand commonly run commands in that code-base.  Gosh supports having multiple scripts in the same file, and invoked via command-line parameters.  GoSh makes this easy:

```
func main() {	
	gosh.MultiTarget()
}
```

Running a Go program that calls `MultiTarget()` will display usage information and a description of available targets if run without command-line parameters.

## Todos:
 - combining all Call and Target logic
 - autowire function params into script variables
 - write more tests
 - more research on not adding cruft to modules
 - add support for non-local targets like Docker or SSH
 - look at [Just](https://github.com/casey/just)

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
