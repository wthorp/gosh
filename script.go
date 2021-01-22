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

// Script represents an execution script context.
type Script struct {
	cmds  []string
	dirs  []string
	onErr func(error)
	env   map[string]string
}

// Run creates a new execution script context.
func Run(cmdScript string) {
	workingDir, _ := os.Getwd()
	env := make(map[string]string, len(os.Environ()))
	for _, pair := range os.Environ() {
		i := strings.Index(pair, "=")
		env[pair[0:i]] = pair[i+1:]
	}
	script := Script{
		cmds:  strings.Split(strings.Trim(cmdScript, "\n"), "\n"),
		dirs:  []string{workingDir},
		onErr: defaultErr,
		env:   env,
	}
	script.RunCmds()
}

// Run executes all commands defined in a script.
func (b *Script) Run(cmdScript string) {
	b.cmds = strings.Split(strings.Trim(cmdScript, "\n"), "\n")
	b.RunCmds()
}

// RunCmds executes all commands defined in a script.
func (b *Script) RunCmds() {
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

		if f, ok := Calls[strings.ToLower(firstWord)]; ok {
			// add script as a parameter if the function supports it
			var in []reflect.Value
			if f.Func.Type().NumIn() == 0 {
				in = []reflect.Value{}
			} else if f.Func.Type().In(0) == reflect.TypeOf(&Script{}) {
				in = []reflect.Value{reflect.ValueOf(b), reflect.ValueOf(otherWords)}
			} else {
				in = []reflect.Value{reflect.ValueOf(otherWords)}
			}
			// call go code via reflection
			callResults := f.Func.Call(in)
			for _, result := range callResults {
				err, ok := result.Interface().(error)
				if ok && err != nil {
					b.onErr(fmt.Errorf("error in Go code, line %d\n[%s]\n%w", lineNum, cmd, err))
				}
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

// Exec runs a program on the operating system.
func (b *Script) Exec(input string) error {
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

// ShowCmd displays what functions are callable from the CLI.
func ShowCmd(b interface{}) error {
	fooType := reflect.TypeOf(b)
	for i := 0; i < fooType.NumMethod(); i++ {
		method := fooType.Method(i)
		fmt.Println(method.Name)
	}
	return nil
}

// ///////////// Built in calls /////////

var _ = Register(Echo, Getwd, Cd, MkDir, Pushd, Popd, Rm, RmDir, Set)

// Echo writes to standard output.
func Echo(text string) error {
	_, err := fmt.Println(text)
	return err
}

// Getwd returns the current directory.
func Getwd(b *Script) (dir string) {
	return b.dirs[0]
}

// Cd changes out of the current directory.
func Cd(b *Script, dir string) error {
	b.dirs[0] = filepath.Join(b.dirs[0], dir)
	return nil
}

// MkDir adds a directory to the file system.
func MkDir(b *Script, dir string) error {
	return os.MkdirAll(dir, 0744)
}

// Pushd changes out of the current directory to the previous directory.
func Pushd(b *Script, dir string) error {
	b.dirs = append([]string{dir}, b.dirs...)
	return nil
}

// Popd changes out of the current directory to the previous directory.
func Popd(b *Script, _ string) error {
	if len(b.dirs) > 1 {
		b.dirs = b.dirs[1:]
	}
	return nil
}

// Rm removes a file from the file system.
func Rm(b *Script, file string) error {
	return os.Remove(file)
}

// RmDir removes a directory from the file system.
func RmDir(b *Script, dir string) error {
	return os.RemoveAll(dir)
}

// Set adds or removes a named string to the script's environment.
func Set(b *Script, pair string) error {
	i := strings.Index(pair, "=")
	b.env[strings.Trim(pair[:i], " ")] = strings.Trim(pair[i+1:], " ")
	return nil
}

// ///////////// Built in calls /////////
