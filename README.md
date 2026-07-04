# GoSh

GoSh makes it easy to write command-running scripts from Go code:

- author multiline, easy to write, command-running scripts from Go code
- invoke different scripts from the same file, similar to Make or Just
- run scripts concurrently without working directory or environment contention
- expose commands in a way that agents can inspect before they act

It still works as a small Go task runner. The newer bits are there so a CLI or agent can ask "is
this a known command?", "what args does it take?", and "should this go to Codex?" before doing
anything.

## GoSh Scripts

The basis of GoSh is that it can run multi-line scripts from Go:

```
gosh.Run(`
	set yinz = World
	echo Hello ${yinz}
	git diff
`)
```

Each script has its own working directory stack and environment map, so concurrent scripts don't
fight over process-wide `cd` or `set` state.

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

It's easy to add your own. `Register` uses the Go function name:

```
var _ = gosh.Register(helloWorld)
```

You can also register anonymous functions, 3rd party code, or set custom names:

```
var _ = gosh.Cmd("helloWorld", func(who string) { ... })
```

Commands are invoked in a case insensitive manner. However, when registering a function, initial
letter capitalization means that the function will be exposed via the command line.

For agent-readable commands, use `Tool` metadata:

```
var _ = gosh.Tool("Deploy", deploy,
	gosh.Desc("Deploy the service to an environment"),
	gosh.Param("env", gosh.Enum("staging", "prod")),
	gosh.Param("replicas", gosh.Type("integer")),
	gosh.Risk(gosh.RiskHigh),
	gosh.RequiresApproval(),
)
```

`Cmd` and `Register` still work as before. They also generate minimal metadata automatically, so
older commands can appear in tool listings without opting into typed argument validation.

Typed params are positional when called from a Gosh script or CLI line. They become JSON Schema
properties when listed for an agent or exposed over MCP.

## Gosh Menu

Gosh supports invoking exposed commands from the CLI, allowing multiple commands to coexist in the
same file. Running `gosh.Menu()` will either display usage information or invoke the requested CLI
parameters.

```
func main() {
	gosh.Menu()
}
```

Exported command names are shown in the menu. Lowercase command names remain script-callable but are
hidden from the CLI and MCP tool list.

## Agentic Routing

`gosh.Menu()` now resolves incoming CLI input before deciding what to do:

1. registered Gosh command -> run it directly
2. executable command on `PATH` -> run it directly
3. invalid or disallowed input -> reject it
4. otherwise -> send it to Codex with `codex exec`

This keeps known commands deterministic and only uses AI when the input is not already a concrete
Gosh or CLI command.

Routed input must be a single command line. Shell control operators such as `|`, `;`, `&`, `<`, `>`,
backticks, and embedded newlines are not directly routed. Wrap complex shell behavior in an explicit
Gosh command or pass it through an intentional command such as `sh -c`.

To classify input without executing it:

```
go run my-goshfile.go --resolve "git status"
go run my-goshfile.go --resolve "Deploy staging 2"
```

Example output:

```
{
  "kind": "external_cli",
  "input": "git status",
  "command": "git",
  "args": ["status"],
  "executable": "/usr/bin/git",
  "confidence": 1,
  "valid": true,
  "risk": "low",
  "reason": "matched executable on PATH"
}
```

You can also call the resolver from Go:

```
result := gosh.Resolve("deploy staging")
```

Typed `Tool` params are validated by `--resolve` before anything runs. Invalid enum values, missing
required params, wrong primitive types, and too many args return `kind: "rejected"` with
`validation_errors`.

## Agent Tooling

To list exported tools in a JSON shape suitable for agents:

```
go run my-goshfile.go tools --json
```

To expose exported tools over MCP stdio:

```
go run my-goshfile.go serve mcp
```

The MCP server implements `initialize`, `tools/list`, `tools/call`, and `ping`. Tool output is
captured and returned as text content so stdout remains reserved for MCP JSON-RPC messages.

MCP tools are generated from the same `Tool`, `Cmd`, and `Register` registry. `Tool` commands get
typed schemas. Older commands get a simple optional `input` string field.

Tools marked `RequiresApproval` or `RiskHigh` are listed with destructive annotations, but
`ServeMCP` rejects calls to them by default. If your MCP host already has a human confirmation flow,
call `ServeMCPWithOptions` and explicitly enable `AllowApprovalRequired` and/or `AllowHighRisk`.

## Security Model

Gosh routes direct commands without invoking a shell. That means shell pipelines, redirection,
backgrounding, command substitution, and multiline input are rejected at the routing layer instead
of being interpreted implicitly.

`SafePolicy` is intentionally narrow: it permits common inspection commands such as `git status`,
`ls`, `pwd`, `rg`, `cat`, and `wc`, while rejecting high-risk external commands. It is a routing
policy, not a sandbox; callers that need stronger isolation should still run Gosh inside their own
process, filesystem, or container boundary.

## Codex Fallback

Unmatched input is handled by `CodexBackend`, which shells out to `codex exec`. The default backend
uses the current working directory, `workspace-write` sandboxing, and `never` approval mode for
non-interactive execution. Override it with `MenuWithOptions` or these environment variables:

- `GOSH_CODEX_BIN`
- `GOSH_CODEX_MODEL`
- `GOSH_CODEX_SANDBOX`
- `GOSH_CODEX_APPROVAL`
- `GOSH_CODEX_ARGS`

For tighter routing, use `gosh.SafePolicy()` or pass a custom `gosh.Policy`.

See the [examples directory](./example) to get a better feel for usage.

## Current non-goals

- returning error codes when used with `go run`
  - its known that `go run` returns the error code from compiling, not running a Go program
  - to return error levels correctly, first compile then run the code
- declarative dependencies (like Make)
  - its assumed that calling Go functions covers the 85% use case
  - `sync.Once()` and infinite-loop checks are easy imperative fixes

## Todos

- expose function params as script variables by name
- add support for non-local targets like Docker or SSH
- see if there's a way to hack `go run` at runtime to return Gosh error codes

GoSh is pronounced 'gosh' if you like it, otherwise 'gauche'.
