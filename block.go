package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var lineRegEx = regexp.MustCompile(`[^\s"]+|"([^"]*)"`)
var defaultFuncs = map[string](func(*Block, string) error){
	"cd": (*Block).Cd, "echo": (*Block).Echo, "mkdir": (*Block).MkDir,
	"pushd": (*Block).Pushd, "popd": (*Block).Popd, "rm": (*Block).Rm,
	"rmdir": (*Block).RmDir}
var defaultErr = func(err error) {
	fmt.Printf("FAIL: %+v", err)
	os.Exit(1)
}

// Block represents an execution block context.
type Block struct {
	cmds  []string
	dirs  []string
	funcs map[string](func(*Block, string) error)
	onErr func(error)
	env   map[string]string
}

// NewBlock creates a new execution block context.
func NewBlock(cmdBlock string) *Block {
	workingDir, _ := os.Getwd()
	funcs := make(map[string](func(*Block, string) error))
	for k, v := range defaultFuncs {
		funcs[k] = v
	}
	env := make(map[string]string, len(os.Environ()))
	for _, pair := range os.Environ() {
		i := strings.Index(pair, "=")
		env[pair[0:i]] = pair[i+1:]
	}
	return &Block{
		cmds:  strings.Split(strings.Trim(cmdBlock, "\n"), "\n"),
		dirs:  []string{workingDir},
		funcs: funcs,
		onErr: defaultErr,
		env:   env,
	}
}

// Run executes all commands defined in a block.
func (b *Block) Run() {
	for lineNum, cmd := range b.cmds {
		cmd = strings.Trim(strings.ReplaceAll(cmd, "\t", " "), " ")
		if cmd == "" || strings.HasPrefix(cmd, "//") || strings.HasPrefix(cmd, "#") {
			continue
		}
		space := strings.Index(cmd, " ")
		firstWord := cmd
		otherWords := ""
		if space != -1 {
			firstWord = cmd[0:space]
			otherWords = cmd[space+1:]
		}
		if f, ok := b.funcs[firstWord]; ok {
			// call go code
			err := f(b, otherWords)
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

// Cd changes out of the current directory.
func (b *Block) Cd(dir string) error {
	b.dirs[0] = filepath.Join(b.dirs[0], dir)
	return nil
}

// Echo writes to standard output.
func (b *Block) Echo(text string) error {
	_, err := fmt.Println(text)
	return err
}

// Exec runs a program on the operating system.
func (b *Block) Exec(input string) error {
	input = os.Expand(input, func(s string) string {
		e, _ := b.env[s]
		return e
	})
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

// MkDir adds a directory to the file system.
func (b *Block) MkDir(dir string) error {
	return os.MkdirAll(dir, 0600)
}

// Pushd changes out of the current directory to the previous directory.
func (b *Block) Pushd(dir string) error {
	b.dirs = append([]string{dir}, b.dirs...)
	return nil
}

// Popd changes out of the current directory to the previous directory.
func (b *Block) Popd(_ string) error {
	if len(b.dirs) > 1 {
		b.dirs = b.dirs[1:]
	}
	return nil
}

// Rm removes a file from the file system.
func (b *Block) Rm(file string) error {
	return os.Remove(file)
}

// RmDir removes a directory from the file system.
func (b *Block) RmDir(dir string) error {
	return nil
}

// Set adds or removes a named string to the block's environment.
func (b *Block) Set(pair string) error {
	i := strings.Index(pair, "=")
	b.env[strings.Trim(pair[:i], " ")] = strings.Trim(pair[i+1:], " ")
	return nil
}
