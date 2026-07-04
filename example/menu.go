//go:build ignore
// +build ignore

package main

import (
	"fmt"

	"github.com/wthorp/gosh"
)

func main() {
	// gosh.Menu() displays usage information if this
	// program is run without parameters. It also handles:
	//
	//	go run example/menu.go --resolve "Deploy staging 2"
	//	go run example/menu.go tools --json
	//	go run example/menu.go serve mcp
	gosh.Menu()
}

// Use gosh.Register with an exported function to
// allow it to be called from the command line.
var _ = gosh.Register(HelloGoshAndGo)

// HelloGoshAndGo is meant to be called from the CLI.
func HelloGoshAndGo(name string) {
	gosh.Run(`
		echo Hello ${name} from Gosh!
		HelloGo ${name}
	`)
}

// gosh.Cmd() can also register anonymous functions for
// use via command line.  Simple capitalize the name.
var _ = gosh.Cmd("HelloGo", func(name string) {
	fmt.Println("Welcome!")
	fmt.Printf("Hello %s directly from Go!\n", name)
})

// gosh.Cmd() can also be used to hide exported commands
// from the CLI, while leaving them accessible from script.
var _ = gosh.Cmd("secret", PoorlyGuardedSecret)

// PoorlyGuardedSecret could be from a 3rd party code.
func PoorlyGuardedSecret() {
	fmt.Println("Shh!  The secret word is 'gosh'!")
}

// Use gosh.Tool to make a command discoverable and typed for agents.
var _ = gosh.Tool("Deploy", Deploy,
	gosh.Desc("Deploy the example service to an environment"),
	gosh.Param("env", gosh.Enum("staging", "prod")),
	gosh.Param("replicas", gosh.Type("integer")),
	gosh.Risk(gosh.RiskHigh),
	gosh.RequiresApproval(),
)

// Deploy is callable from scripts, the CLI, tools --json, and MCP.
func Deploy(env string, replicas int) {
	fmt.Printf("Deploying %d replicas to %s\n", replicas, env)
}
