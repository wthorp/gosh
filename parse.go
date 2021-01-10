package gosh

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Args proxies os.Args for code-generation ease.
var Args = os.Args

// Autowire shows usage information or runs methods.
func Autowire() error {
	// determine the go file that was 'go run'
	hasTarget := len(os.Args) > 1
	goSrc, err := determineGoFile()
	if err != nil {
		return err
	}
	// parse go source file
	fs := token.NewFileSet()
	astFile, err := parser.ParseFile(fs, goSrc, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	// if no command line paramters were invoked, show usage
	if !hasTarget {
		return showUsage(goSrc, astFile)
	}

	newGoSrc, err := alterGoSource(goSrc, astFile)
	if err != nil {
		return err
	}
	return runTemporaryGoFile(newGoSrc)
}

// showUsage displays what functions are callable from the CLI.
func showUsage(goSrc string, astFile *ast.File) error {
	foundTargets := false
	fmt.Printf("Available targest for 'go run %s' [target]:\n", filepath.Base(goSrc))
	ast.Inspect(astFile, func(x ast.Node) bool {
		astFunc, isFunc := x.(*ast.FuncDecl)
		if isFunc && astFunc.Recv == nil && astFunc.Name.IsExported() {
			foundTargets = true
			fmt.Printf("    %s ", astFunc.Name.Name)
			for _, p := range astFunc.Type.Params.List {
				fmt.Printf("%v ", p.Names)
			}
			fmt.Printf(": %s\n", strings.Trim(astFunc.Doc.Text(), "\t\n "))
		}
		return !isFunc
	})
	if !foundTargets {
		fmt.Printf("    No targets exist in go file '%s'!\n", goSrc)
	}
	return nil
}

// alterGoSource finds 'gosh.Autowire()' and replaced it with hardwired commands.
func alterGoSource(goSrc string, astFile *ast.File) (string, error) {
	hardWire := "\tswitch gosh.Args[1] {\n"
	ast.Inspect(astFile, func(x ast.Node) bool {
		astFunc, isFunc := x.(*ast.FuncDecl)
		if isFunc && astFunc.Recv == nil && astFunc.Name.IsExported() {
			hardWire += fmt.Sprintf("\tcase \"%s\":\n\t\t%s(", astFunc.Name.Name, astFunc.Name.Name)
			for n := range astFunc.Type.Params.List {
				hardWire += fmt.Sprintf("gosh.Args[%d], ", n+2)
			}
			if len(astFunc.Type.Params.List) > 0 {
				hardWire = hardWire[0 : len(hardWire)-2] // trim tailing comma
			}
			hardWire += ")\n"
		}
		return !isFunc
	})
	hardWire += "\t}\n"

	findGoshCalls(astFile)

	src, err := ioutil.ReadFile(goSrc)
	if err != nil {
		return "", err
	}
	return strings.Replace(string(src), "gosh.Autowire()", hardWire, 1), nil
}

func runTemporaryGoFile(goCode string) error {
	// todo: usage build cache to speed this up?
	// todo: use a better
	// f, err := ioutil.TempFile(".", "gosh")
	// if err != nil {
	// 	return err
	// }
	defer os.Remove("temp.go")
	//f.Write([]byte(goCode))
	//f.Close()
	ioutil.WriteFile("temp.go", []byte(goCode), 0600)

	fmt.Printf("Running things!\n")
	args := append([]string{"run", "temp.go"}, os.Args[1:]...)
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// findGoshCalls filters a list of functions to ones suitable as gosh calls.
func findGoshCalls(astFile *ast.File) []*ast.FuncDecl {
	goshCalls := make([]*ast.FuncDecl, 0)
	ast.Inspect(astFile, func(x ast.Node) bool {
		astFunc, isFunc := x.(*ast.FuncDecl)
		if isFunc && astFunc.Recv != nil && types.ExprString(astFunc.Recv.List[0].Type) == "*gosh.Block" {
			goshCalls = append(goshCalls, astFunc)
		}
		// params := astFunc.Type.Params.List
		// if len(params) == 0 {
		// 	continue
		// }
		// if types.ExprString(params[0].Type) != "*gosh.Block" {
		// 	continue
		// }
		return !isFunc // skip the inside of any functions
	})
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
