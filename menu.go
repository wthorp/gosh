package gosh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Menu displays usage information or invokes an exported command
func Menu() {
	// show usage information if no command specified
	if len(os.Args) <= 1 {
		// if goSrc := determineGoFile(); goSrc != "" {
		// 	showUsageFromSrc(goSrc)
		//  return
		// }
		showUsageFromReflection()
	} else {
		Run(strings.Join(os.Args[1:], " "))
	}
}

// showUsageFromReflection displays what functions are callable from the CLI, using reflection.
func showUsageFromReflection() {
	foundTargets := false
	exe, _ := os.Executable()
	fmt.Printf("Usage: 'go run %s.go' [command]:\n", filepath.Base(exe))
	for _, c := range Calls {
		if c.Exported {
			fmt.Printf("    %s ", c.Name)
			rt := c.Func.Type()
			for p := 0; p < rt.NumIn(); p++ {
				// todo: see if there's a hack to get parameter names and comments
				fmt.Printf("[%s] ", rt.In(p).Name())
			}
			fmt.Printf("\n")
			foundTargets = true
		}
	}
	if !foundTargets {
		fmt.Printf("    No targets found!\n")
	}
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
