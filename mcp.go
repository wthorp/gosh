package gosh

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const mcpProtocolVersion = "2025-06-18"

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type mcpTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ServeMCP serves exported Gosh tools over the MCP stdio transport.
func ServeMCP(in io.Reader, out io.Writer, logs io.Writer) error {
	if logs == nil {
		logs = os.Stderr
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req mcpRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeMCPError(encoder, json.RawMessage("null"), -32700, "Parse error", err.Error())
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			writeMCPError(encoder, req.IDOrNull(), -32600, "Invalid Request", nil)
			continue
		}
		if len(req.ID) == 0 {
			handleMCPNotification(req, logs)
			continue
		}

		result, rpcErr := handleMCPRequest(req)
		if rpcErr != nil {
			writeMCPError(encoder, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			continue
		}
		if err := encoder.Encode(mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (r mcpRequest) IDOrNull() json.RawMessage {
	if len(r.ID) == 0 {
		return json.RawMessage("null")
	}
	return r.ID
}

func handleMCPNotification(req mcpRequest, logs io.Writer) {
	if req.Method != "notifications/initialized" {
		writef(logs, "gosh mcp ignored notification %s\n", req.Method)
	}
}

func handleMCPRequest(req mcpRequest) (interface{}, *mcpError) {
	switch req.Method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": false,
				},
			},
			"serverInfo": map[string]interface{}{
				"name":    "gosh",
				"title":   "Gosh Tool Server",
				"version": "0.0.0",
			},
			"instructions": "Use these tools for deterministic Gosh commands. Tools marked destructive or requiring approval should be confirmed with the user before invocation.",
		}, nil
	case "ping":
		return map[string]interface{}{}, nil
	case "tools/list":
		return map[string]interface{}{"tools": mcpTools()}, nil
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &mcpError{Code: -32602, Message: "Invalid params", Data: err.Error()}
		}
		return callMCPTool(params.Name, params.Arguments)
	default:
		return nil, &mcpError{Code: -32601, Message: "Method not found", Data: req.Method}
	}
}

func writeMCPError(encoder *json.Encoder, id json.RawMessage, code int, message string, data interface{}) {
	_ = encoder.Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message, Data: data},
	})
}

func mcpTools() []mcpTool {
	infos := Tools()
	out := make([]mcpTool, 0, len(infos))
	for _, info := range infos {
		annotations := map[string]interface{}{
			"readOnlyHint":    info.Risk == RiskLow && !info.RequiresApproval,
			"destructiveHint": info.Risk == RiskHigh || info.RequiresApproval,
		}
		out = append(out, mcpTool{
			Name:        info.Name,
			Description: info.Description,
			InputSchema: info.InputSchema,
			Annotations: annotations,
		})
	}
	return out
}

func callMCPTool(name string, arguments map[string]interface{}) (interface{}, *mcpError) {
	call, ok := Calls[strings.ToLower(name)]
	if !ok || !call.Exported {
		return nil, &mcpError{Code: -32602, Message: "Unknown tool", Data: name}
	}
	if arguments == nil {
		arguments = map[string]interface{}{}
	}

	rawArgs, argv, err := mcpArgs(call.Tool, arguments)
	if err != nil {
		return nil, &mcpError{Code: -32602, Message: "Invalid arguments", Data: err.Error()}
	}
	if validation := validateToolArgs(call.Tool, argv); !validation.Valid {
		return nil, &mcpError{Code: -32602, Message: "Invalid arguments", Data: validation.Errors}
	}

	output, callErr := captureStdout(func() error {
		script, err := newScriptContext()
		if err != nil {
			return err
		}
		return invokeCall(script, call, rawArgs, argv)
	})
	if callErr != nil {
		return map[string]interface{}{
			"content": []map[string]string{{
				"type": "text",
				"text": callErr.Error(),
			}},
			"isError": true,
		}, nil
	}
	if strings.TrimSpace(output) == "" {
		output = "ok"
	}
	return map[string]interface{}{
		"content": []map[string]string{{
			"type": "text",
			"text": output,
		}},
		"isError": false,
	}, nil
}

func mcpArgs(tool ToolSpec, arguments map[string]interface{}) (string, []string, error) {
	if !tool.Structured {
		if value, ok := arguments["input"]; ok {
			return fmt.Sprint(value), nil, nil
		}
		return "", nil, nil
	}

	args := []string{}
	for _, param := range tool.Params {
		value, ok := arguments[param.Name]
		if !ok {
			if param.Required {
				return "", nil, fmt.Errorf("missing required argument %s", param.Name)
			}
			continue
		}
		args = append(args, argumentString(value))
	}
	return strings.Join(args, " "), args, nil
}

func argumentString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(typed)
	}
}

func newScriptContext() (*Script, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(os.Environ()))
	for _, pair := range os.Environ() {
		i := strings.Index(pair, "=")
		env[pair[0:i]] = pair[i+1:]
	}
	return &Script{
		dirs: []string{workingDir},
		onErr: func(error) {
		},
		env: env,
	}, nil
}

func captureStdout(fn func() error) (string, error) {
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}

	var buffer bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buffer, reader)
		done <- copyErr
	}()

	os.Stdout = writer
	var panicValue interface{}
	var callErr error
	func() {
		defer func() {
			panicValue = recover()
		}()
		callErr = fn()
	}()
	_ = writer.Close()
	os.Stdout = oldStdout

	if copyErr := <-done; copyErr != nil && callErr == nil {
		callErr = copyErr
	}
	_ = reader.Close()
	if panicValue != nil {
		panic(panicValue)
	}
	return buffer.String(), callErr
}
