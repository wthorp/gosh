package gosh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	if err := RunE(cmdScript); err != nil {
		defaultErr(err)
	}
}

// RunE creates a new execution script context and returns the first error.
func RunE(cmdScript string) error {
	workingDir, _ := os.Getwd()
	env := make(map[string]string, len(os.Environ()))
	for _, pair := range os.Environ() {
		i := strings.Index(pair, "=")
		env[pair[0:i]] = pair[i+1:]
	}
	var firstErr error
	script := Script{
		cmds: strings.Split(strings.Trim(cmdScript, "\n"), "\n"),
		dirs: []string{workingDir},
		onErr: func(err error) {
			if firstErr == nil {
				firstErr = err
			}
		},
		env: env,
	}
	script.RunCmds()
	return firstErr
}

// Run executes all commands defined in a script.
func (s *Script) Run(cmdScript string) {
	s.cmds = strings.Split(strings.Trim(cmdScript, "\n"), "\n")
	s.RunCmds()
}

// RunCmds executes all commands defined in a script.
func (s *Script) RunCmds() {
	for lineNum, cmd := range s.cmds {
		cmd = strings.Trim(strings.ReplaceAll(cmd, "\t", " "), " ")
		if cmd == "" || strings.HasPrefix(cmd, "//") || strings.HasPrefix(cmd, "#") {
			continue
		}
		cmd = os.Expand(cmd, func(x string) string { return s.env[x] })

		space := strings.Index(cmd, " ")
		firstWord := cmd
		otherWords := ""
		if space != -1 {
			firstWord = cmd[0:space]
			otherWords = cmd[space+1:]
		}

		if f, ok := Calls[strings.ToLower(firstWord)]; ok {
			args := []string{}
			if f.Tool.Structured {
				params, err := SplitArgs(cmd)
				if err != nil {
					s.onErr(fmt.Errorf("error parsing args, line %d\n[%s]\n%w", lineNum, cmd, err))
					return
				}
				if len(params) > 1 {
					args = params[1:]
				}
			}
			if err := invokeCall(s, f, otherWords, args); err != nil {
				s.onErr(fmt.Errorf("error in Go code, line %d\n[%s]\n%w", lineNum, cmd, err))
				return
			}
		} else {
			// run executable program
			err := s.Exec(cmd)
			if err != nil {
				s.onErr(fmt.Errorf("error executing program, line %d\n[%s]\n%w", lineNum, cmd, err))
				return
			}
		}
	}
}

// Exec runs a program on the operating system.
func (s *Script) Exec(input string) error {
	params, err := SplitArgs(input)
	if err != nil {
		return err
	}
	if len(params) == 0 {
		return nil
	}
	cmd := params[0]
	args := []string{}
	if len(params) > 1 {
		args = params[1:]
	}
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = s.dirs[0]
	for k, v := range s.env {
		c.Env = append(c.Env, k+"="+v)
	}
	return c.Run()
}

// ///////////// Built in calls /////////

var _ = Register((*Script).echo, (*Script).getwd, (*Script).cd, (*Script).mkDir,
	(*Script).pushd, (*Script).popd, (*Script).rm, (*Script).rmDir, (*Script).set)

// Echo writes to standard output.
func (*Script) echo(text string) error {
	_, err := fmt.Println(text)
	return err
}

// Getwd returns the current directory.
func (s *Script) getwd() (dir string) {
	return s.dirs[0]
}

// Cd changes out of the current directory.
func (s *Script) cd(dir string) error {
	s.dirs[0] = filepath.Join(s.dirs[0], dir)
	return nil
}

// MkDir adds a directory to the file system.
func (s *Script) mkDir(dir string) error {
	dir = filepath.Join(s.dirs[0], dir)
	return os.MkdirAll(dir, 0744)
}

// Pushd changes out of the current directory to the previous directory.
func (s *Script) pushd(dir string) error {
	s.dirs = append([]string{dir}, s.dirs...)
	return nil
}

// Popd changes out of the current directory to the previous directory.
func (s *Script) popd(_ string) error {
	if len(s.dirs) > 1 {
		s.dirs = s.dirs[1:]
	}
	return nil
}

// Rm removes a file from the file system.
func (s *Script) rm(file string) error {
	file = filepath.Join(s.dirs[0], file)
	return os.Remove(file)
}

// RmDir removes a directory from the file system.
func (s *Script) rmDir(dir string) error {
	dir = filepath.Join(s.dirs[0], dir)
	return os.RemoveAll(dir)
}

// Set adds or removes a named string to the script's environment.
func (s *Script) set(pair string) error {
	i := strings.Index(pair, "=")
	s.env[strings.Trim(pair[:i], " ")] = strings.Trim(pair[i+1:], " ")
	return nil
}

// ///////////// Built in calls /////////
