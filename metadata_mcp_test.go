package gosh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

var typedToolEnv string
var typedToolCount int
var typedUintValue uint64

var _ = Tool("GoshTypedDeployTest", func(env string, count int) {
	typedToolEnv = env
	typedToolCount = count
},
	Desc("Deploy test fixture"),
	Param("env", Enum("staging", "prod")),
	Param("count", Type("integer")),
	Risk(RiskHigh),
	RequiresApproval(),
)

var mcpEchoName string

var _ = Tool("GoshMCPEchoTest", func(name string) {
	mcpEchoName = name
	fmt.Printf("hello %s", name)
},
	Desc("MCP echo test fixture"),
	Param("name"),
)

var _ = Cmd("GoshLegacyMetadataTest", func(value string) {})

var _ = Tool("GoshLegacyToolMetadataFixture", func(value string) {},
	Desc("Legacy tool fixture with metadata"),
)

var _ = Tool("GoshUintParamTest", func(value uint64) {
	typedUintValue = value
},
	Desc("Unsigned integer fixture"),
	Param("value"),
)

var typedStringIntegerValue string

var _ = Tool("GoshStringIntegerParamTest", func(value string) {
	typedStringIntegerValue = value
},
	Desc("String target with integer schema fixture"),
	Param("value", Type("integer")),
)

var typedOptionalEnumMode string

var _ = Tool("GoshOptionalEnumParamTest", func(mode string) {
	typedOptionalEnumMode = mode
},
	Desc("Optional enum fixture"),
	Param("mode", Optional(), Enum("fast", "slow")),
)

var _ = Cmd("GoshLegacyUnsupportedIntTest", func(value int) {})

var _ = Cmd("GoshLegacyUnsupportedMultiTest", func(a string, b string) {})

var _ = Tool("GoshMCPNestedScriptFailureTest", func(s *Script) {
	s.Run("bad-command-that-does-not-exist")
},
	Desc("Nested script failure fixture"),
)

func TestToolMetadataAndCmdCompatibility(t *testing.T) {
	typed := Calls[strings.ToLower("GoshTypedDeployTest")].Tool
	if typed.Description != "Deploy test fixture" {
		t.Fatalf("description = %q", typed.Description)
	}
	if !typed.Structured {
		t.Fatalf("expected structured tool")
	}
	if typed.Risk != RiskHigh {
		t.Fatalf("risk = %s, want %s", typed.Risk, RiskHigh)
	}
	if !typed.RequiresApproval {
		t.Fatalf("expected requires approval")
	}
	if len(typed.Params) != 2 || typed.Params[0].Name != "env" || typed.Params[1].Type != "integer" {
		t.Fatalf("params = %+v", typed.Params)
	}

	legacy := Calls[strings.ToLower("GoshLegacyMetadataTest")].Tool
	if legacy.Name != "GoshLegacyMetadataTest" {
		t.Fatalf("legacy metadata missing: %+v", legacy)
	}
	if legacy.Structured {
		t.Fatalf("legacy Cmd metadata should not enable structured validation")
	}

	legacyTool := Calls[strings.ToLower("GoshLegacyToolMetadataFixture")].Tool
	if len(legacyTool.Params) != 1 || legacyTool.Params[0].Name != "input" {
		t.Fatalf("legacy tool schema missing input param: %+v", legacyTool)
	}

	for _, name := range []string{"GoshLegacyUnsupportedIntTest", "GoshLegacyUnsupportedMultiTest"} {
		if _, found := findToolInfo(name); found {
			t.Fatalf("unsupported legacy command %s should not be exposed as a tool", name)
		}
	}
}

func TestResolveValidatesTypedArgs(t *testing.T) {
	valid := Resolve("GoshTypedDeployTest staging 2")
	if valid.Kind != RouteGoshCommand || !valid.Valid {
		t.Fatalf("valid resolve = %+v", valid)
	}
	if !valid.RequiresApproval {
		t.Fatalf("expected requires approval in route result")
	}

	invalid := Resolve("GoshTypedDeployTest dev 2")
	if invalid.Kind != RouteRejected || invalid.Valid {
		t.Fatalf("invalid resolve = %+v", invalid)
	}
	if len(invalid.ValidationErrors) == 0 {
		t.Fatalf("expected validation errors")
	}
}

func TestRunETypedToolBinding(t *testing.T) {
	typedToolEnv = ""
	typedToolCount = 0
	if err := RunE("GoshTypedDeployTest prod 7"); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if typedToolEnv != "prod" || typedToolCount != 7 {
		t.Fatalf("typed binding env=%q count=%d", typedToolEnv, typedToolCount)
	}
}

func TestResolveRunEAndMCPHandleUint64(t *testing.T) {
	const maxUint64 = "18446744073709551615"

	result := Resolve("GoshUintParamTest " + maxUint64)
	if result.Kind != RouteGoshCommand || !result.Valid {
		t.Fatalf("resolve result = %+v", result)
	}

	typedUintValue = 0
	if err := RunE("GoshUintParamTest " + maxUint64); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if typedUintValue != ^uint64(0) {
		t.Fatalf("typed uint value = %d", typedUintValue)
	}

	if _, rpcErr := callMCPTool("GoshUintParamTest", map[string]interface{}{"value": json.Number(maxUint64)}); rpcErr != nil {
		t.Fatalf("unexpected mcp rpc error: %+v", rpcErr)
	}
}

func TestDeclaredSchemaTypeValidatedForStringTargets(t *testing.T) {
	result := Resolve("GoshStringIntegerParamTest nope")
	if result.Kind != RouteRejected || len(result.ValidationErrors) == 0 {
		t.Fatalf("resolve result = %+v", result)
	}
	if _, rpcErr := callMCPTool("GoshStringIntegerParamTest", map[string]interface{}{"value": "nope"}); rpcErr == nil {
		t.Fatalf("expected MCP validation error")
	}
}

func TestMCPOptionalEnumCanBeOmitted(t *testing.T) {
	typedOptionalEnumMode = "unset"
	if _, rpcErr := callMCPTool("GoshOptionalEnumParamTest", nil); rpcErr != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcErr)
	}
	if typedOptionalEnumMode != "" {
		t.Fatalf("optional enum value = %q, want empty default", typedOptionalEnumMode)
	}
}

func TestUnsupportedLegacyCommandsRejectEarly(t *testing.T) {
	result := Resolve("GoshLegacyUnsupportedIntTest 1")
	if result.Kind != RouteRejected || len(result.ValidationErrors) == 0 {
		t.Fatalf("resolve result = %+v", result)
	}
	if err := RunE("GoshLegacyUnsupportedIntTest 1"); err == nil {
		t.Fatalf("expected RunE error for unsupported legacy signature")
	}
	if _, rpcErr := callMCPTool("GoshLegacyUnsupportedIntTest", map[string]interface{}{"input": 1}); rpcErr == nil {
		t.Fatalf("expected MCP error for unsupported legacy signature")
	}
}

func TestMenuToolsJSON(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"goshfile", "tools", "--json"}
	var out bytes.Buffer
	MenuWithOptions(MenuOptions{Stdout: &out})

	var tools []ToolInfo
	if err := json.Unmarshal(out.Bytes(), &tools); err != nil {
		t.Fatalf("invalid tools json: %v\n%s", err, out.String())
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "GoshTypedDeployTest" {
			found = true
			if tool.InputSchema["type"] != "object" {
				t.Fatalf("input schema = %+v", tool.InputSchema)
			}
		}
	}
	if !found {
		t.Fatalf("typed tool not found in tools json: %+v", tools)
	}
}

func TestServeMCPListAndCall(t *testing.T) {
	input := strings.Join([]string{
		mcpFrame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`),
		mcpFrame(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`),
		mcpFrame(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"GoshMCPEchoTest","arguments":{"name":"world"}}}`),
	}, "")

	mcpEchoName = ""
	var out bytes.Buffer
	var logs bytes.Buffer
	if err := ServeMCP(strings.NewReader(input), &out, &logs); err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}

	messages := decodeMCPMessages(t, out.String())
	if len(messages) != 3 {
		t.Fatalf("got %d response messages:\n%s", len(messages), out.String())
	}
	if !strings.Contains(messages[1], "GoshMCPEchoTest") {
		t.Fatalf("tools/list missing echo tool: %s", messages[1])
	}
	if !strings.Contains(messages[2], "hello world") {
		t.Fatalf("tools/call missing captured output: %s", messages[2])
	}
	if mcpEchoName != "world" {
		t.Fatalf("mcp tool did not run, name=%q", mcpEchoName)
	}
}

func TestCallMCPToolReturnsNestedScriptFailure(t *testing.T) {
	result, rpcErr := callMCPTool("GoshMCPNestedScriptFailureTest", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"isError":true`) || !strings.Contains(string(encoded), "bad-command-that-does-not-exist") {
		t.Fatalf("nested script failure result = %s", encoded)
	}
}

func findToolInfo(name string) (ToolInfo, bool) {
	for _, tool := range Tools() {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolInfo{}, false
}
