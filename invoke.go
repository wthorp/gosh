package gosh

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type argValidation struct {
	Valid  bool
	Errors []string
}

func validateToolArgs(tool ToolSpec, args []string) argValidation {
	if !tool.Structured {
		return argValidation{Valid: true}
	}

	required := 0
	for _, param := range tool.Params {
		if param.Required {
			required++
		}
	}

	var errors []string
	if len(args) < required {
		errors = append(errors, fmt.Sprintf("expected at least %d args, got %d", required, len(args)))
	}
	if len(args) > len(tool.Params) {
		errors = append(errors, fmt.Sprintf("expected at most %d args, got %d", len(tool.Params), len(args)))
	}

	limit := len(args)
	if limit > len(tool.Params) {
		limit = len(tool.Params)
	}
	for i := 0; i < limit; i++ {
		param := tool.Params[i]
		if err := validateParamValue(param, args[i]); err != nil {
			errors = append(errors, err.Error())
		}
	}

	return argValidation{Valid: len(errors) == 0, Errors: errors}
}

func validateParamValue(param ParamSpec, value string) error {
	if len(param.Enum) > 0 {
		found := false
		for _, allowed := range param.Enum {
			if value == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s must be one of %s", param.Name, strings.Join(param.Enum, ", "))
		}
	}

	switch param.Type {
	case "", "string":
		return nil
	case "boolean":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be boolean", param.Name)
		}
	case "integer":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("%s must be integer", param.Name)
		}
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("%s must be number", param.Name)
		}
	default:
		return fmt.Errorf("%s has unsupported type %q", param.Name, param.Type)
	}
	return nil
}

func invokeCall(script *Script, call Call, rawArgs string, args []string) error {
	if call.Tool.Structured {
		validation := validateToolArgs(call.Tool, args)
		if !validation.Valid {
			return fmt.Errorf("invalid arguments: %s", strings.Join(validation.Errors, "; "))
		}
		return invokeStructuredCall(script, call, args)
	}
	return invokeLegacyCall(script, call, rawArgs)
}

func invokeLegacyCall(script *Script, call Call, rawArgs string) error {
	rt := call.Func.Type()
	in := []reflect.Value{}

	if rt.NumIn() == 0 {
		return collectCallError(call.Func.Call(in))
	}

	if rt.In(0) == reflect.TypeOf(&Script{}) {
		in = append(in, reflect.ValueOf(script))
		if rt.NumIn() > 1 {
			if rt.NumIn() > 2 {
				return fmt.Errorf("legacy command %s has %d non-script parameters; use Tool params for typed binding", call.Name, rt.NumIn()-1)
			}
			in = append(in, reflect.ValueOf(rawArgs))
		}
		return collectCallError(call.Func.Call(in))
	}

	if rt.NumIn() > 1 {
		return fmt.Errorf("legacy command %s has %d parameters; use Tool params for typed binding", call.Name, rt.NumIn())
	}
	in = append(in, reflect.ValueOf(rawArgs))
	return collectCallError(call.Func.Call(in))
}

func invokeStructuredCall(script *Script, call Call, args []string) error {
	rt := call.Func.Type()
	in := []reflect.Value{}
	start := 0

	if rt.NumIn() > 0 && rt.In(0) == reflect.TypeOf(&Script{}) {
		in = append(in, reflect.ValueOf(script))
		start = 1
	}

	if rt.NumIn()-start != len(args) {
		return fmt.Errorf("tool %s expects %d reflected args, got %d", call.Name, rt.NumIn()-start, len(args))
	}

	for i, arg := range args {
		value, err := convertArg(arg, rt.In(start+i))
		if err != nil {
			return fmt.Errorf("%s: %w", call.Tool.Params[i].Name, err)
		}
		in = append(in, value)
	}
	return collectCallError(call.Func.Call(in))
}

func convertArg(value string, target reflect.Type) (reflect.Value, error) {
	switch target.Kind() {
	case reflect.String:
		return reflect.ValueOf(value).Convert(target), nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return reflect.Value{}, err
		}
		return reflect.ValueOf(parsed).Convert(target), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, target.Bits())
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(target).Elem()
		out.SetInt(parsed)
		return out, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, target.Bits())
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(target).Elem()
		out.SetUint(parsed)
		return out, nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, target.Bits())
		if err != nil {
			return reflect.Value{}, err
		}
		out := reflect.New(target).Elem()
		out.SetFloat(parsed)
		return out, nil
	default:
		return reflect.Value{}, fmt.Errorf("unsupported target type %s", target)
	}
}

func collectCallError(results []reflect.Value) error {
	for _, result := range results {
		if !result.IsValid() || !result.CanInterface() {
			continue
		}
		err, ok := result.Interface().(error)
		if ok && err != nil {
			return err
		}
	}
	return nil
}
