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
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"GoshMCPEchoTest","arguments":{"name":"world"}}}`,
	}, "\n")

	mcpEchoName = ""
	var out bytes.Buffer
	var logs bytes.Buffer
	if err := ServeMCP(strings.NewReader(input), &out, &logs); err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d response lines:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[1], "GoshMCPEchoTest") {
		t.Fatalf("tools/list missing echo tool: %s", lines[1])
	}
	if !strings.Contains(lines[2], "hello world") {
		t.Fatalf("tools/call missing captured output: %s", lines[2])
	}
	if mcpEchoName != "world" {
		t.Fatalf("mcp tool did not run, name=%q", mcpEchoName)
	}
}
