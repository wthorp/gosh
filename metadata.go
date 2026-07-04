package gosh

import (
	"reflect"
	"sort"
	"strings"
)

// ToolSpec describes a registered Gosh command for agents and CLIs.
type ToolSpec struct {
	Name             string      `json:"name"`
	Description      string      `json:"description,omitempty"`
	Risk             RiskLevel   `json:"risk,omitempty"`
	RequiresApproval bool        `json:"requires_approval"`
	Exported         bool        `json:"exported"`
	Structured       bool        `json:"structured"`
	Params           []ParamSpec `json:"params,omitempty"`
}

// ParamSpec describes one positional tool parameter.
type ParamSpec struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
}

// ToolInfo is the JSON-facing representation of one registered tool.
type ToolInfo struct {
	ToolSpec
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolOption configures Tool metadata.
type ToolOption func(*ToolSpec)

// ParamOption configures one tool parameter.
type ParamOption func(*ParamSpec)

// Tool associates a Go function with a command name and agent-readable metadata.
func Tool(name string, call interface{}, options ...ToolOption) interface{} {
	return registerCall(name, call, options...)
}

// Desc sets a human-readable tool description.
func Desc(description string) ToolOption {
	return func(t *ToolSpec) {
		t.Description = description
	}
}

// Param appends a structured positional parameter to a tool.
func Param(name string, options ...ParamOption) ToolOption {
	return func(t *ToolSpec) {
		param := ParamSpec{
			Name:     name,
			Type:     "string",
			Required: true,
		}
		for _, option := range options {
			option(&param)
		}
		if param.Type == "" {
			param.Type = "string"
		}
		t.Structured = true
		t.Params = append(t.Params, param)
	}
}

// Risk sets the tool risk level.
func Risk(level RiskLevel) ToolOption {
	return func(t *ToolSpec) {
		t.Risk = level
	}
}

// RequiresApproval marks a tool as needing human confirmation before execution.
func RequiresApproval() ToolOption {
	return func(t *ToolSpec) {
		t.RequiresApproval = true
	}
}

// Enum restricts a string parameter to a fixed set of values.
func Enum(values ...string) ParamOption {
	return func(p *ParamSpec) {
		p.Type = "string"
		p.Enum = append([]string{}, values...)
	}
}

// ParamDesc sets a human-readable parameter description.
func ParamDesc(description string) ParamOption {
	return func(p *ParamSpec) {
		p.Description = description
	}
}

// Optional marks a parameter as optional.
func Optional() ParamOption {
	return func(p *ParamSpec) {
		p.Required = false
	}
}

// Type sets a JSON Schema primitive type for a parameter.
func Type(name string) ParamOption {
	return func(p *ParamSpec) {
		p.Type = name
	}
}

// Tools returns exported tool metadata sorted by name.
func Tools() []ToolInfo {
	return tools(false)
}

func tools(includeHidden bool) []ToolInfo {
	out := []ToolInfo{}
	for _, call := range Calls {
		if !includeHidden && !call.Exported {
			continue
		}
		if !call.Tool.Structured && !legacyCallSupported(call.Func.Type()) {
			continue
		}
		info := ToolInfo{
			ToolSpec:    call.Tool,
			InputSchema: call.Tool.inputSchema(),
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func minimalToolSpec(name string, exported bool) ToolSpec {
	return ToolSpec{
		Name:       name,
		Risk:       commandRisk(name, nil),
		Exported:   exported,
		Structured: false,
	}
}

func ensureLegacyInputParam(spec *ToolSpec, rt reflect.Type) {
	if spec.Structured || len(spec.Params) > 0 || !legacyCallAcceptsRawInput(rt) {
		return
	}

	// Backward-compatible commands keep receiving raw input, but the MCP schema
	// gives clients a conventional way to pass that input when needed.
	spec.Params = []ParamSpec{{
		Name:     "input",
		Type:     "string",
		Required: false,
	}}
}

type legacyCallKind int

const (
	legacyUnsupported legacyCallKind = iota
	legacyNoArgs
	legacyRawInput
	legacyScriptOnly
	legacyScriptRawInput
)

func classifyLegacyCall(rt reflect.Type) legacyCallKind {
	if rt.NumIn() == 0 {
		return legacyNoArgs
	}
	if rt.In(0) == reflect.TypeOf(&Script{}) {
		switch rt.NumIn() {
		case 1:
			return legacyScriptOnly
		case 2:
			if rt.In(1).Kind() == reflect.String {
				return legacyScriptRawInput
			}
		}
		return legacyUnsupported
	}
	if rt.NumIn() == 1 && rt.In(0).Kind() == reflect.String {
		return legacyRawInput
	}
	return legacyUnsupported
}

func legacyCallSupported(rt reflect.Type) bool {
	return classifyLegacyCall(rt) != legacyUnsupported
}

func legacyCallAcceptsRawInput(rt reflect.Type) bool {
	switch classifyLegacyCall(rt) {
	case legacyRawInput, legacyScriptRawInput:
		return true
	default:
		return false
	}
}

func reflectedParamTypes(rt reflect.Type) []reflect.Type {
	start := 0
	if rt.NumIn() > 0 && rt.In(0) == reflect.TypeOf(&Script{}) {
		start = 1
	}

	types := []reflect.Type{}
	for i := start; i < rt.NumIn(); i++ {
		types = append(types, rt.In(i))
	}
	return types
}

func inferParamTypes(spec *ToolSpec, rt reflect.Type) {
	paramTypes := reflectedParamTypes(rt)
	for i := range spec.Params {
		if i >= len(paramTypes) {
			continue
		}
		if spec.Params[i].Type == "" || spec.Params[i].Type == "string" {
			spec.Params[i].Type = jsonTypeForReflect(paramTypes[i])
		}
	}
}

func jsonTypeForReflect(rt reflect.Type) string {
	switch rt.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "string"
	}
}

func (t ToolSpec) inputSchema() map[string]interface{} {
	properties := map[string]interface{}{}
	required := []string{}

	params := t.Params
	if !t.Structured && len(params) > 0 {
		params = params[:1]
	}

	for _, param := range params {
		property := map[string]interface{}{
			"type": param.Type,
		}
		if param.Description != "" {
			property["description"] = param.Description
		}
		if len(param.Enum) > 0 {
			property["enum"] = param.Enum
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
