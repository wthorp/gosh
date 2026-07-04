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

	validation := validateToolArgShape(tool, args)
	if !validation.Valid {
		return validation
	}

	for i := 0; i < len(args) && i < len(tool.Params); i++ {
		if err := validateParamValue(tool.Params[i], args[i]); err != nil {
			validation.Errors = append(validation.Errors, err.Error())
		}
	}
	validation.Valid = len(validation.Errors) == 0
	return validation
}

func validateToolArgShape(tool ToolSpec, args []string) argValidation {
	if !tool.Structured {
		return argValidation{Valid: true}
	}

	var errors []string
	errors = append(errors, validateToolLayout(tool)...)

	required := 0
	for _, param := range tool.Params {
		if param.Required {
			required++
		}
	}

	if len(args) < required {
		errors = append(errors, fmt.Sprintf("expected at least %d args, got %d", required, len(args)))
	}
	if len(args) > len(tool.Params) {
		errors = append(errors, fmt.Sprintf("expected at most %d args, got %d", len(tool.Params), len(args)))
	}

	return argValidation{Valid: len(errors) == 0, Errors: errors}
}

func validateCallArgs(call Call, args []string) argValidation {
	if !call.Tool.Structured {
		if legacyCallSupported(call.Func.Type()) {
			return argValidation{Valid: true}
		}
		return argValidation{
			Valid:  false,
			Errors: []string{fmt.Sprintf("legacy command %s has unsupported signature for direct routing", call.Name)},
		}
	}

	validation := validateToolArgShape(call.Tool, args)
	if !validation.Valid {
		return validation
	}

	paramTypes := reflectedParamTypes(call.Func.Type())
	if len(paramTypes) != len(call.Tool.Params) {
		return argValidation{
			Valid:  false,
			Errors: []string{fmt.Sprintf("tool %s declares %d params but function expects %d reflected args", call.Name, len(call.Tool.Params), len(paramTypes))},
		}
	}

	for i := 0; i < len(args) && i < len(call.Tool.Params); i++ {
		if err := validateParamValueForTarget(call.Tool.Params[i], args[i], paramTypes[i]); err != nil {
			validation.Errors = append(validation.Errors, err.Error())
		}
	}
	validation.Valid = len(validation.Errors) == 0
	return validation
}

func validateToolLayout(tool ToolSpec) []string {
	seenOptional := false
	var errors []string
	for _, param := range tool.Params {
		if !param.Required {
			seenOptional = true
			continue
		}
		if seenOptional {
			errors = append(errors, "required params cannot follow optional params")
			break
		}
	}
	return errors
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

func validateParamValueForTarget(param ParamSpec, value string, target reflect.Type) error {
	if len(param.Enum) > 0 {
		if err := validateParamValue(ParamSpec{Name: param.Name, Type: "string", Enum: param.Enum}, value); err != nil {
			return err
		}
	}

	switch param.Type {
	case "":
	case "string":
	case "boolean":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be boolean", param.Name)
		}
		return nil
	case "integer":
		switch target.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if _, err := strconv.ParseUint(value, 10, target.Bits()); err != nil {
				return fmt.Errorf("%s must be integer", param.Name)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if _, err := strconv.ParseInt(value, 10, target.Bits()); err != nil {
				return fmt.Errorf("%s must be integer", param.Name)
			}
		default:
			if _, err := strconv.ParseInt(value, 10, 64); err != nil {
				return fmt.Errorf("%s must be integer", param.Name)
			}
		}
		return nil
	case "number":
		bits := 64
		if target.Kind() == reflect.Float32 || target.Kind() == reflect.Float64 {
			bits = target.Bits()
		}
		if _, err := strconv.ParseFloat(value, bits); err != nil {
			return fmt.Errorf("%s must be number", param.Name)
		}
		return nil
	default:
		return fmt.Errorf("%s has unsupported type %q", param.Name, param.Type)
	}

	switch target.Kind() {
	case reflect.String:
		return nil
	case reflect.Bool:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be boolean", param.Name)
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if _, err := strconv.ParseInt(value, 10, target.Bits()); err != nil {
			return fmt.Errorf("%s must be integer", param.Name)
		}
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if _, err := strconv.ParseUint(value, 10, target.Bits()); err != nil {
			return fmt.Errorf("%s must be integer", param.Name)
		}
		return nil
	case reflect.Float32, reflect.Float64:
		if _, err := strconv.ParseFloat(value, target.Bits()); err != nil {
			return fmt.Errorf("%s must be number", param.Name)
		}
		return nil
	default:
		return validateParamValue(param, value)
	}
}

func invokeCall(script *Script, call Call, rawArgs string, args []string) error {
	if call.Tool.Structured {
		validation := validateCallArgs(call, args)
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

	switch classifyLegacyCall(rt) {
	case legacyNoArgs:
		return collectCallError(call.Func.Call(in))
	case legacyScriptOnly:
		in = append(in, reflect.ValueOf(script))
		return collectCallError(call.Func.Call(in))
	case legacyScriptRawInput:
		in = append(in, reflect.ValueOf(script), reflect.ValueOf(rawArgs))
		return collectCallError(call.Func.Call(in))
	case legacyRawInput:
		in = append(in, reflect.ValueOf(rawArgs))
		return collectCallError(call.Func.Call(in))
	default:
		return fmt.Errorf("legacy command %s has unsupported signature; use Tool params for typed binding", call.Name)
	}
}

func invokeStructuredCall(script *Script, call Call, args []string) error {
	rt := call.Func.Type()
	in := []reflect.Value{}
	start := 0

	if rt.NumIn() > 0 && rt.In(0) == reflect.TypeOf(&Script{}) {
		in = append(in, reflect.ValueOf(script))
		start = 1
	}

	if rt.NumIn()-start != len(call.Tool.Params) {
		return fmt.Errorf("tool %s declares %d params but function expects %d reflected args", call.Name, len(call.Tool.Params), rt.NumIn()-start)
	}

	for i := range call.Tool.Params {
		arg := zeroValueString(call.Tool.Params[i])
		if i < len(args) {
			arg = args[i]
		}
		value, err := convertArg(arg, rt.In(start+i))
		if err != nil {
			return fmt.Errorf("%s: %w", call.Tool.Params[i].Name, err)
		}
		in = append(in, value)
	}
	return collectCallError(call.Func.Call(in))
}

func zeroValueString(param ParamSpec) string {
	switch param.Type {
	case "boolean":
		return "false"
	case "integer":
		return "0"
	case "number":
		return "0"
	default:
		return ""
	}
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
