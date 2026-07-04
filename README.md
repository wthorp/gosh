# GoSh

GoSh makes it easy to write command-running scripts from Go code:

- author multiline, easy to write, command-running scripts from Go code
- invoke different scripts from the same file, similar to Make or Just
- run scripts concurrently without working directory or environment contention
- experiment with agent-friendly commands that are still deterministic

## GoSh Scripts

The basis of GoSh is that it can run multi-line scripts:

```
gosh.Run(`
	set yinz = World
	echo Hello ${yinz}
	git diff
`)
```

## GoSh Commands

GoSh has the following shell-like commands built in, for use from scripts:

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

You can also register anonymous functions, 3rd party code, or set custom names:

```
var _ = gosh.Cmd("helloWorld", func(who string) { ... })
```

Commands are invoked in a case insensitive manner. However, when registering a function, initial
letter capitalization means that the function will be exposed via the command line.

## Gosh Menu

Gosh supports invoking exposed commands from the CLI, allowing multiple commands to coexist in the
same file. Running `gosh.Menu()` will either display usage information or invoke the requested CLI
parameters.

```
func main() {
	gosh.Menu()
}
```

## Agentic Commands

GoSh helps agents do more deterministic work.

The idea is close to Make: define known commands in code, give them names and simple inputs, and let
an agent choose from those commands instead of inventing shell scripts from scratch every time.

For commands that need a little more shape, use `Tool`:

```
var _ = gosh.Tool("Deploy", deploy,
	gosh.Desc("Deploy to an environment"),
	gosh.Param("env", gosh.Enum("staging", "prod")),
)
```

You can list exposed tools as JSON:

```
go run my-goshfile.go tools --json
```

You can also expose them over MCP:

```
go run my-goshfile.go serve mcp
```

MCP here is not meant to turn GoSh into a sandbox or a full workflow engine. It is a way to make
agentic work look more like calling a Make target: inspect the known commands, pass structured
arguments, run the selected command, and get the output back.

If a CLI input is not a known GoSh command or a normal executable command, GoSh can fall back to
`codex exec`. Known commands stay deterministic; unknown requests can still be handled by an agent.

See the [examples directory](./example) to get a better feel for usage.

## Current non-goals

- returning error codes when used with `go run`
  - its known that `go run` returns the error code from compiling, not running a Go program
  - to return error levels correctly, first compile then run the code
- declarative dependencies (like Make)
  - its assumed that calling Go functions covers the 85% use case
  - `sync.Once()` and infinite-loop checks are easy imperative fixes

## Todos

- better script / CLI / param binding
- expose function params as script variables by name
- add support for non-local targets like Docker or SSH
- see if there's a way to hack `go run` at runtime to return Gosh error codes

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
