package gosh

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unicode"
)

// Call associates code with a name, so that can be invoked from script or CLI
type Call struct {
	Name     string
	Func     reflect.Value
	Exported bool
}

// Calls reference all code that can be invoked from script or CLI
var Calls map[string]Call = make(map[string]Call)

// Register associates a Go function with its name, so that it can be
// invoked via scripts or the command line.
func Register(funcs ...interface{}) interface{} {
	for _, f := range funcs {
		rv := reflect.ValueOf(f)
		if rv.Kind() != reflect.Func {
			panic(fmt.Sprintf("Cannot create go call from '%+v'", f))
		}
		longName := runtime.FuncForPC(rv.Pointer()).Name()
		shortName := longName[strings.LastIndex(longName, ".")+1:]
		Cmd(shortName, f)
	}
	return nil
}

// Cmd associates a Go function with a name, so that it can be invoked
// via scripts or the command line.
func Cmd(name string, call interface{}) interface{} {
	rv := reflect.ValueOf(call)
	if rv.Kind() != reflect.Func {
		panic(fmt.Sprintf("Cannot create go call from '%s'", name))
	}
	if _, found := Calls[name]; found {
		panic(fmt.Sprintf("Cannot create more than one call named '%s'", name))
	}
	Calls[strings.ToLower(name)] = Call{Name: name, Func: rv, Exported: unicode.IsUpper([]rune(name)[0])}
	return nil
}
