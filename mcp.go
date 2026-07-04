package gosh

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
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

// MCPOptions controls which registered tools may be invoked through MCP.
type MCPOptions struct {
	// AllowApprovalRequired permits tools marked RequiresApproval to run through
	// MCP. Leave this false unless the hosting client has its own confirmation
	// flow before calling the tool.
	AllowApprovalRequired bool

	// AllowHighRisk permits tools marked RiskHigh to run through MCP.
	AllowHighRisk bool
}

// ServeMCP serves exported Gosh tools over the MCP stdio transport.
func ServeMCP(in io.Reader, out io.Writer, logs io.Writer) error {
	return ServeMCPWithOptions(in, out, logs, MCPOptions{})
}

// ServeMCPWithOptions serves exported Gosh tools over MCP with explicit policy.
func ServeMCPWithOptions(in io.Reader, out io.Writer, logs io.Writer, options MCPOptions) error {
	if logs == nil {
		logs = os.Stderr
	}

	reader := bufio.NewReader(in)

	for {
		payload, err := readMCPMessage(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var req mcpRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if err := writeMCPError(out, json.RawMessage("null"), -32700, "Parse error", err.Error()); err != nil {
				return err
			}
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if err := writeMCPError(out, req.IDOrNull(), -32600, "Invalid Request", nil); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			handleMCPNotification(req, logs)
			continue
		}

		result, rpcErr := handleMCPRequest(req, options)
		if rpcErr != nil {
			if err := writeMCPError(out, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data); err != nil {
				return err
			}
			continue
		}
		if err := writeMCPMessage(out, mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result}); err != nil {
			return err
		}
	}
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

func handleMCPRequest(req mcpRequest, options MCPOptions) (interface{}, *mcpError) {
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
			"instructions": "Use these tools for deterministic Gosh commands. Tools marked destructive or requiring approval are rejected unless the Gosh MCP host explicitly enables them after its own confirmation flow.",
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
		decoder := json.NewDecoder(bytes.NewReader(req.Params))
		decoder.UseNumber()
		if err := decoder.Decode(&params); err != nil {
			return nil, &mcpError{Code: -32602, Message: "Invalid params", Data: err.Error()}
		}
		return callMCPToolWithOptions(params.Name, params.Arguments, options)
	default:
		return nil, &mcpError{Code: -32601, Message: "Method not found", Data: req.Method}
	}
}

func writeMCPError(out io.Writer, id json.RawMessage, code int, message string, data interface{}) error {
	return writeMCPMessage(out, mcpResponse{
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
	return callMCPToolWithOptions(name, arguments, MCPOptions{})
}

func callMCPToolWithOptions(name string, arguments map[string]interface{}, options MCPOptions) (interface{}, *mcpError) {
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
	if validation := validateMCPCallArgs(call, arguments); !validation.Valid {
		return nil, &mcpError{Code: -32602, Message: "Invalid arguments", Data: validation.Errors}
	}
	if call.Tool.RequiresApproval && !options.AllowApprovalRequired {
		return nil, &mcpError{Code: -32000, Message: "Tool requires approval", Data: name}
	}
	if call.Tool.Risk == RiskHigh && !options.AllowHighRisk {
		return nil, &mcpError{Code: -32000, Message: "High-risk tool disabled", Data: name}
	}

	output, callErr := captureStdout(func() error {
		script, err := newScriptContext()
		if err != nil {
			return err
		}
		if call.Tool.Structured {
			if err := invokeStructuredCall(script, call, argv); err != nil {
				return err
			}
		} else {
			if err := invokeLegacyCall(script, call, rawArgs); err != nil {
				return err
			}
		}
		return script.firstErr
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
		allowed := make(map[string]struct{}, len(tool.Params))
		for _, param := range tool.Params {
			allowed[param.Name] = struct{}{}
		}
		for name := range arguments {
			if _, ok := allowed[name]; !ok {
				return "", nil, fmt.Errorf("unknown argument %s", name)
			}
		}
		if value, ok := arguments["input"]; ok {
			return fmt.Sprint(value), nil, nil
		}
		return "", nil, nil
	}

	allowed := make(map[string]struct{}, len(tool.Params))
	for _, param := range tool.Params {
		allowed[param.Name] = struct{}{}
	}
	for name := range arguments {
		if _, ok := allowed[name]; !ok {
			return "", nil, fmt.Errorf("unknown argument %s", name)
		}
	}

	args := make([]string, 0, len(tool.Params))
	for _, param := range tool.Params {
		value, ok := arguments[param.Name]
		if !ok {
			if param.Required {
				return "", nil, fmt.Errorf("missing required argument %s", param.Name)
			}
			args = append(args, zeroValueString(param))
			continue
		}
		args = append(args, argumentString(value))
	}
	return strings.Join(args, " "), args, nil
}

func validateMCPCallArgs(call Call, arguments map[string]interface{}) argValidation {
	if !call.Tool.Structured {
		return validateCallArgs(call, nil)
	}

	errors := validateToolLayout(call.Tool)
	paramTypes := reflectedParamTypes(call.Func.Type())
	if len(paramTypes) != len(call.Tool.Params) {
		errors = append(errors, fmt.Sprintf("tool %s declares %d params but function expects %d reflected args", call.Name, len(call.Tool.Params), len(paramTypes)))
		return argValidation{Valid: false, Errors: errors}
	}

	for i, param := range call.Tool.Params {
		value, ok := arguments[param.Name]
		if !ok {
			if param.Required {
				errors = append(errors, fmt.Sprintf("missing required argument %s", param.Name))
			}
			continue
		}
		if err := validateParamValueForTarget(param, argumentString(value), paramTypes[i]); err != nil {
			errors = append(errors, err.Error())
		}
	}
	return argValidation{Valid: len(errors) == 0, Errors: errors}
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

func readMCPMessage(reader *bufio.Reader) ([]byte, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			trimmed := strings.TrimSpace(line)
			if err == io.EOF && trimmed == "" {
				return nil, io.EOF
			}
			if err == io.EOF && looksLikeMCPJSON(trimmed) {
				return []byte(trimmed), nil
			}
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}

		headerLine := strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(headerLine)
		if trimmed == "" {
			continue
		}
		if looksLikeMCPJSON(trimmed) {
			return []byte(trimmed), nil
		}

		headers := map[string]string{}
		for {
			name, value, ok := strings.Cut(headerLine, ":")
			if !ok {
				return nil, fmt.Errorf("invalid MCP header line %q", headerLine)
			}
			headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)

			next, err := reader.ReadString('\n')
			if err != nil {
				return nil, err
			}
			headerLine = strings.TrimRight(next, "\r\n")
			if strings.TrimSpace(headerLine) == "" {
				break
			}
		}

		lengthValue, ok := headers["content-length"]
		if !ok {
			return nil, fmt.Errorf("missing Content-Length header")
		}
		length, err := strconv.Atoi(lengthValue)
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid Content-Length %q", lengthValue)
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
		return payload, nil
	}
}

func writeMCPMessage(out io.Writer, payload interface{}) error {
	return json.NewEncoder(out).Encode(payload)
}

func looksLikeMCPJSON(line string) bool {
	return strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[")
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
		env:  env,
		ctx:  context.Background(),
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
