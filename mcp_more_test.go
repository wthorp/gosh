package gosh

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var _ = Tool("GoshMCPErrorTest", func() error {
	return errors.New("tool failed")
})

var _ = Tool("GoshMCPOkNoOutputTest", func() {})

func TestServeMCPErrorsNotificationsAndPing(t *testing.T) {
	input := strings.Join([]string{
		mcpFrame(`not-json`),
		mcpFrame(`{"jsonrpc":"2.0","method":"notifications/unknown"}`),
		mcpFrame(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
		mcpFrame(`{"jsonrpc":"2.0","id":2,"method":"unknown"}`),
		mcpFrame(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":[]}`),
	}, "")

	var out bytes.Buffer
	var logs bytes.Buffer
	if err := ServeMCP(strings.NewReader(input), &out, &logs); err != nil {
		t.Fatalf("ServeMCP returned error: %v", err)
	}
	if !strings.Contains(logs.String(), "ignored notification") {
		t.Fatalf("expected notification log, got %q", logs.String())
	}

	lines := decodeMCPMessages(t, out.String())
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
	if err := ServeMCP(strings.NewReader(mcpFrame(`{"jsonrpc":"1.0","method":""}`)), &out, nil); err != nil {
		t.Fatal(err)
	}
	messages := decodeMCPMessages(t, out.String())
	if len(messages) != 1 || !strings.Contains(messages[0], `"id":null`) || !strings.Contains(messages[0], "Invalid Request") {
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
	if _, rpcErr := callMCPTool("GoshTypedDeployTest", map[string]interface{}{"env": "prod", "count": 1}); rpcErr == nil {
		t.Fatalf("expected approval-required tool to be blocked by default")
	}

	typedToolEnv = ""
	typedToolCount = 0
	result, rpcErr := callMCPToolWithOptions("GoshTypedDeployTest", map[string]interface{}{"env": "prod", "count": 1}, MCPOptions{
		AllowApprovalRequired: true,
		AllowHighRisk:         true,
	})
	if rpcErr != nil {
		t.Fatalf("unexpected rpc error with explicit options: %+v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if typedToolEnv != "prod" || typedToolCount != 1 || !strings.Contains(string(encoded), `"isError":false`) {
		t.Fatalf("tool did not run with options; env=%q count=%d result=%s", typedToolEnv, typedToolCount, encoded)
	}

	result, rpcErr = callMCPTool("GoshMCPErrorTest", nil)
	if rpcErr != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcErr)
	}
	encoded, err = json.Marshal(result)
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
	raw, argv, err := mcpArgs(ToolSpec{
		Structured: false,
		Params: []ParamSpec{{
			Name: "input",
			Type: "string",
		}},
	}, map[string]interface{}{"input": 123})
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
	if len(argv) != 2 || argv[0] != "true" || argv[1] != "" {
		t.Fatalf("argv = %#v", argv)
	}
	if _, _, err := mcpArgs(tool, map[string]interface{}{}); err == nil {
		t.Fatalf("expected missing required arg")
	}
	if _, _, err := mcpArgs(tool, map[string]interface{}{"enabled": true, "extra": "nope"}); err == nil {
		t.Fatalf("expected unknown arg error")
	}
	if _, _, err := mcpArgs(ToolSpec{}, map[string]interface{}{"input": "nope"}); err == nil {
		t.Fatalf("expected unknown legacy arg error without schema")
	}
	if argumentString(false) != "false" || argumentString(1.5) != "1.5" {
		t.Fatalf("unexpected argumentString results")
	}
}

func TestCallMCPToolRejectsUnknownArguments(t *testing.T) {
	if _, rpcErr := callMCPTool("GoshTypedDeployTest", map[string]interface{}{
		"env":        "staging",
		"count":      2,
		"unexpected": true,
	}); rpcErr == nil {
		t.Fatalf("expected invalid arguments rpc error")
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
	if err := writeMCPError(&out, json.RawMessage(`"abc"`), -32600, "bad", map[string]string{"why": "test"}); err != nil {
		t.Fatal(err)
	}
	messages := decodeMCPMessages(t, out.String())
	if len(messages) != 1 || !strings.Contains(messages[0], `"id":"abc"`) || !strings.Contains(messages[0], `"why":"test"`) {
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

func mcpFrame(payload string) string {
	return "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + payload
}

func decodeMCPMessages(t *testing.T, wire string) []string {
	t.Helper()

	reader := bufio.NewReader(strings.NewReader(wire))
	var messages []string
	for {
		payload, err := readMCPMessage(reader)
		if errors.Is(err, io.EOF) {
			return messages
		}
		if err != nil {
			t.Fatalf("failed to decode MCP message: %v", err)
		}
		messages = append(messages, string(payload))
	}
}
