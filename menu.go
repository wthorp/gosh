package gosh

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MenuOptions configures MenuWithOptions.
type MenuOptions struct {
	Policy  Policy
	Backend AIBackend
	Stdout  io.Writer
	Stderr  io.Writer
}

// Menu displays usage information or invokes an exported command
func Menu() {
	MenuWithOptions(MenuOptions{})
}

// MenuWithOptions displays usage information, resolves input, or routes it.
func MenuWithOptions(options MenuOptions) {
	// show usage information if no command specified
	if len(os.Args) <= 1 {
		// if goSrc := determineGoFile(); goSrc != "" {
		// 	showUsageFromSrc(goSrc)
		//  return
		// }
		showUsageFromReflection(defaultWriter(options.Stdout, os.Stdout))
		return
	}

	policy := options.Policy
	if !policy.configured() {
		policy = DefaultPolicy()
	}

	if os.Args[1] == "--resolve" {
		input := strings.Join(os.Args[2:], " ")
		result := ResolveWithPolicy(input, policy)
		if err := writeRouteJSON(defaultWriter(options.Stdout, os.Stdout), result); err != nil {
			defaultErr(err)
		}
	} else if os.Args[1] == "tools" && len(os.Args) > 2 && os.Args[2] == "--json" {
		if err := writeJSON(defaultWriter(options.Stdout, os.Stdout), Tools()); err != nil {
			defaultErr(err)
		}
	} else if os.Args[1] == "serve" && len(os.Args) > 2 && os.Args[2] == "mcp" {
		if err := ServeMCP(os.Stdin, defaultWriter(options.Stdout, os.Stdout), defaultWriter(options.Stderr, os.Stderr)); err != nil {
			defaultErr(err)
		}
	} else {
		err := RouteWithOptions(context.Background(), strings.Join(os.Args[1:], " "), RouteOptions{
			Policy:  policy,
			Backend: options.Backend,
			Stdout:  options.Stdout,
			Stderr:  options.Stderr,
		})
		if err != nil {
			defaultErr(err)
		}
	}
}

// showUsageFromReflection displays what functions are callable from the CLI, using reflection.
func showUsageFromReflection(w io.Writer) {
	foundTargets := false
	exe, _ := os.Executable()
	writef(w, "Usage: 'go run %s.go' [command]:\n", filepath.Base(exe))
	for _, c := range Calls {
		if c.Exported {
			writef(w, "    %s ", c.Name)
			rt := c.Func.Type()
			for p := 0; p < rt.NumIn(); p++ {
				// todo: see if there's a hack to get parameter names and comments
				writef(w, "[%s] ", rt.In(p).Name())
			}
			writef(w, "\n")
			foundTargets = true
		}
	}
	if !foundTargets {
		writef(w, "    No targets found!\n")
	}
	writef(w, "\nMeta:\n")
	writef(w, "    --resolve [input]    classify input as JSON without executing\n")
	writef(w, "    tools --json         list exported Gosh tools as JSON\n")
	writef(w, "    serve mcp            serve exported Gosh tools over MCP stdio\n")
}

func writef(w io.Writer, format string, args ...interface{}) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// // showUsageFromSrc displays what functions are callable from the CLI, using Go source.
// func showUsageFromSrc(goSrc string) error {
// 	foundTargets := false
// 	fmt.Printf("Usage: 'go run %s.go' [command]:\n", filepath.Base(goSrc))
// 	// parse go source file
// 	fs := token.NewFileSet()
// 	astFile, err := parser.ParseFile(fs, goSrc, nil, parser.ParseComments)
// 	if err != nil {
// 		return err
// 	}
// 	ast.Inspect(astFile, func(x ast.Node) bool {
// 		astFunc, isFunc := x.(*ast.FuncDecl)
// 		if isFunc && astFunc.Recv == nil && astFunc.Name.IsExported() {
// 			foundTargets = true
// 			fmt.Printf("    %s ", astFunc.Name.Name)
// 			for _, p := range astFunc.Type.Params.List {
// 				fmt.Printf("%v ", p.Names)
// 			}
// 			fmt.Printf(": %s\n", strings.Trim(astFunc.Doc.Text(), "\t\n "))
// 		}
// 		return !isFunc
// 	})
// 	if !foundTargets {
// 		fmt.Printf("    No targets exist in go file '%s'!\n", goSrc)
// 	}
// 	return nil
// }

// // determineGoFile guesses the source Go file, and returns it if it exists.
// func determineGoFile() string {
// 	workingDir, _ := os.Getwd()
// 	exe, _ := os.Executable()
// 	exe = filepath.Base(exe)
// 	if strings.HasSuffix(exe, ".exe") {
// 		exe = exe[:len(exe)-4]
// 	}
// 	srcPath := filepath.Join(workingDir, exe+".go")
// 	if _, err := os.Stat(srcPath); err != nil {
// 		return ""
// 	}
// 	return srcPath
// }
