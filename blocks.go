package gosh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

var lineRegEx = regexp.MustCompile(`[^\s"]+|"([^"]*)"`)
var defaultErr = func(err error) {
	fmt.Printf("FAIL: %+v", err)
	os.Exit(1)
}

// Block represents an execution block context.
type Block struct {
	cmds  []string
	dirs  []string
	onErr func(error)
	env   map[string]string
}

// Run creates a new execution block context.
func Run(cmdBlock string) {
	workingDir, _ := os.Getwd()
	env := make(map[string]string, len(os.Environ()))
	for _, pair := range os.Environ() {
		i := strings.Index(pair, "=")
		env[pair[0:i]] = pair[i+1:]
	}
	block := Block{
		cmds:  strings.Split(strings.Trim(cmdBlock, "\n"), "\n"),
		dirs:  []string{workingDir},
		onErr: defaultErr,
		env:   env,
	}
	block.RunCmds()
}

// Run executes all commands defined in a block.
func (b *Block) Run(cmdBlock string) {
	b.cmds = strings.Split(strings.Trim(cmdBlock, "\n"), "\n")
	b.RunCmds()
}

// RunCmds executes all commands defined in a block.
func (b *Block) RunCmds() {
	for lineNum, cmd := range b.cmds {
		cmd = strings.Trim(strings.ReplaceAll(cmd, "\t", " "), " ")
		if cmd == "" || strings.HasPrefix(cmd, "//") || strings.HasPrefix(cmd, "#") {
			continue
		}
		cmd = os.Expand(cmd, func(s string) string {
			e, _ := b.env[s]
			return e
		})

		space := strings.Index(cmd, " ")
		firstWord := cmd
		otherWords := ""
		if space != -1 {
			firstWord = cmd[0:space]
			otherWords = cmd[space+1:]
		}
		fmt.Printf("Looking for %s in %+v\n", firstWord, Calls)
		if f, ok := Calls[strings.ToLower(firstWord)]; ok {
			// call go code via reflection
			in := []reflect.Value{reflect.ValueOf(b), reflect.ValueOf(otherWords)}
			err := f.Func.Call(in)[0].Interface().(error)
			if err != nil {
				b.onErr(fmt.Errorf("error in Go code, line %d\n[%s]\n%w", lineNum, cmd, err))
			}
		} else {
			// run executable program
			err := b.Exec(cmd)
			if err != nil {
				b.onErr(fmt.Errorf("error executing program, line %d\n[%s]\n%w", lineNum, cmd, err))
			}
		}
	}
}

// Echo writes to standard output.
func (b *Block) Echo(text string) error {
	_, err := fmt.Println(text)
	return err
}

// Exec runs a program on the operating system.
func (b *Block) Exec(input string) error {
	params := lineRegEx.FindAllString(input, -1)
	cmd := params[0]
	args := []string{}
	if len(params) > 1 {
		args = params[1:]
	}
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = b.dirs[0]
	for k, v := range b.env {
		c.Env = append(c.Env, k+"="+v)
	}
	return c.Run()
}

// ShowFunc displays what functions are callable from the CLI.
func ShowFunc(b interface{}) error {
	fooType := reflect.TypeOf(b)
	for i := 0; i < fooType.NumMethod(); i++ {
		method := fooType.Method(i)
		fmt.Println(method.Name)
	}
	return nil
}

// ///////////// Built in calls /////////

var _ = Register(Getwd, Cd, MkDir, Pushd, Popd, Rm, RmDir, Set)

// Getwd returns the current directory.
func Getwd(b *Block) (dir string) {
	return b.dirs[0]
}

// Cd changes out of the current directory.
func Cd(b *Block, dir string) error {
	b.dirs[0] = filepath.Join(b.dirs[0], dir)
	return nil
}

// MkDir adds a directory to the file system.
func MkDir(b *Block, dir string) error {
	return os.MkdirAll(dir, 0744)
}

// Pushd changes out of the current directory to the previous directory.
func Pushd(b *Block, dir string) error {
	b.dirs = append([]string{dir}, b.dirs...)
	return nil
}

// Popd changes out of the current directory to the previous directory.
func Popd(b *Block, _ string) error {
	if len(b.dirs) > 1 {
		b.dirs = b.dirs[1:]
	}
	return nil
}

// Rm removes a file from the file system.
func Rm(b *Block, file string) error {
	return os.Remove(file)
}

// RmDir removes a directory from the file system.
func RmDir(b *Block, dir string) error {
	return os.RemoveAll(dir)
}

// Set adds or removes a named string to the block's environment.
func Set(b *Block, pair string) error {
	i := strings.Index(pair, "=")
	b.env[strings.Trim(pair[:i], " ")] = strings.Trim(pair[i+1:], " ")
	return nil
}

// ///////////// Built in calls /////////
