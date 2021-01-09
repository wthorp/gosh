package gosh

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Go shows usage information or runs methods.
func Go() error {
	hasTarget := len(os.Args) > 1
	goSrc, err := determineGoFile()
	if err != nil {
		return err
	}
	funcs, err := parseGoFile(goSrc)
	if err != nil {
		return err
	}
	if !hasTarget {
		return showUsage(goSrc, funcs)
	}
	// todo: replace main w/ generated target invoker
	// todo: generate logic to add suitable gosh calls to new Calls global
	// todo: save generated logic, compile, ran run it
	return nil
}

// showUsage displays what functions are callable from the CLI.
func showUsage(goSrc string, funcs []*ast.FuncDecl) error {
	if len(funcs) == 0 {
		fmt.Printf("No targets exist in go file '%s'!\n", goSrc)
		return nil
	}
	fmt.Printf("Available targest for 'go run %s' [target]:\n", filepath.Base(goSrc))
	for _, astFunc := range funcs {
		if astFunc.Name.IsExported() {
			fmt.Printf("    %s ", astFunc.Name.Name)
			for _, p := range astFunc.Type.Params.List {
				fmt.Printf("%v ", p.Names)
			}
			fmt.Printf(": %s\n", strings.Trim(astFunc.Doc.Text(), "\t\n "))
		}
	}
	return nil
}

// findGoshCalls filters a list of functions to ones suitable as gosh calls.
func findGoshCalls(funcs []*ast.FuncDecl) []*ast.FuncDecl {
	// Todo: figure out type checker
	// info := types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	// _, err = (&types.Config{}).Check("gosh", fs, []*ast.File{astFile}, &info)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	goshCalls := make([]*ast.FuncDecl, 0)
	for _, astFunc := range funcs {

		if len(astFunc.Type.Params.List) == 0 {
			continue
		}
		// todo: filter for p0 is *gosh.Block
		// p0 := astFunc.Type.Params.List[0]
		// fmt.Printf("%+v\n", info.Types[p0.Type].Type)
	}
	return goshCalls
}

// determineGoFile figure out what go file was like 'go run'.
func determineGoFile() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe = filepath.Base(exe)
	if strings.HasSuffix(exe, ".exe") {
		exe = exe[:len(exe)-4]
	}
	return filepath.Join(workingDir, exe+".go"), nil
}

// parseGoFile parses a go file and returns a list of global functions.
func parseGoFile(goSrc string) ([]*ast.FuncDecl, error) {
	fs := token.NewFileSet()
	astFile, err := parser.ParseFile(fs, goSrc, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	funcs := make([]*ast.FuncDecl, 0)

	//todo: use errogroup here instead
	ast.Inspect(astFile, func(x ast.Node) bool {
		astFunc, ok := x.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if astFunc.Recv != nil {
			return false
		}
		funcs = append(funcs, astFunc)
		return false
	})
	return funcs, nil
}
