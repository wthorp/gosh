package gosh

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

var _ = Tool("GoshMCPErrorTest", func() error {
	return errors.New("tool failed")
})

var _ = Tool("GoshMCPOkNoOutputTest", func() {})

func TestServeMCPErrorsNotificationsAndPing(t *testing.T) {
	input := strings.Join([]string{
		`not-json`,
		`{"jsonrpc":"2.0","method":"notifications/unknown"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":2,"method":"unknown"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":[]}`,
	}, "\n")

	var out bytes.Buffer
	var logs bytes.Buffer
	if err := ServeMCP(strings.NewReader(input), &out, &logs); err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}
	if !strings.Contains(logs.String(), "ignored notification") {
		t.Fatalf("expected notification log, got %q", logs.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("responses = %d\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], "Parse error") ||
		!strings.Contains(lines[1], `"result":{}`) ||
		!strings.Contains(lines[2], "Method not found") ||
		!strings.Contains(lines[3], "Invalid params") {
		t.Fatalf("unexpected MCP responses:\n%s", out.String())
	}
}

func TestServeMCPInvalidRequestUsesNullID(t *testing.T) {
	var out bytes.Buffer
	if err := ServeMCP(strings.NewReader(`{"jsonrpc":"1.0","method":""}`+"\n"), &out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id":null`) || !strings.Contains(out.String(), "Invalid Request") {
		t.Fatalf("invalid request output = %q", out.String())
	}
}

func TestCallMCPToolErrorsAndOkDefaultOutput(t *testing.T) {
	if _, rpcErr := callMCPTool("missing-tool", nil); rpcErr == nil {
		t.Fatalf("expected unknown tool error")
	}
	if _, rpcErr := callMCPTool("GoshTypedDeployTest", map[string]interface{}{"env": "staging"}); rpcErr == nil {
		t.Fatalf("expected missing required arg error")
	}
	if _, rpcErr := callMCPTool("GoshTypedDeployTest", map[string]interface{}{"env": "dev", "count": 1}); rpcErr == nil {
		t.Fatalf("expected invalid enum error")
	}

	result, rpcErr := callMCPTool("GoshMCPErrorTest", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"isError":true`) || !strings.Contains(string(encoded), "tool failed") {
		t.Fatalf("tool error result = %s", encoded)
	}

	result, rpcErr = callMCPTool("GoshMCPOkNoOutputTest", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcErr)
	}
	encoded, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"text":"ok"`) {
		t.Fatalf("default ok result = %s", encoded)
	}
}

func TestMCPArgsAndArgumentString(t *testing.T) {
	raw, argv, err := mcpArgs(ToolSpec{Structured: false}, map[string]interface{}{"input": 123})
	if err != nil {
		t.Fatal(err)
	}
	if raw != "123" || argv != nil {
		t.Fatalf("legacy args raw=%q argv=%#v", raw, argv)
	}

	tool := ToolSpec{
		Structured: true,
		Params: []ParamSpec{
			{Name: "enabled", Type: "boolean", Required: true},
			{Name: "name", Type: "string", Required: false},
		},
	}
	_, argv, err = mcpArgs(tool, map[string]interface{}{"enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 1 || argv[0] != "true" {
		t.Fatalf("argv = %#v", argv)
	}
	if _, _, err := mcpArgs(tool, map[string]interface{}{}); err == nil {
		t.Fatalf("expected missing required arg")
	}
	if argumentString(false) != "false" || argumentString(1.5) != "1.5" {
		t.Fatalf("unexpected argumentString results")
	}
}

func TestCaptureStdoutPanicsAndWriteMCPError(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	_, _ = captureStdout(func() error {
		panic("boom")
	})
}

func TestWriteMCPError(t *testing.T) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	writeMCPError(encoder, json.RawMessage(`"abc"`), -32600, "bad", map[string]string{"why": "test"})
	if !strings.Contains(out.String(), `"id":"abc"`) || !strings.Contains(out.String(), `"why":"test"`) {
		t.Fatalf("error response = %q", out.String())
	}
}

func TestIDOrNull(t *testing.T) {
	if string((mcpRequest{}).IDOrNull()) != "null" {
		t.Fatalf("empty id did not become null")
	}
	id := json.RawMessage(`"id"`)
	if !reflect.DeepEqual((mcpRequest{ID: id}).IDOrNull(), id) {
		t.Fatalf("id was not preserved")
	}
}
