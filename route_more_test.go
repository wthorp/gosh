package gosh

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSafePolicyAndRiskBranches(t *testing.T) {
	policy := SafePolicy()
	if !policy.RejectHighRiskExternal || len(policy.AllowedExternal) == 0 {
		t.Fatalf("unexpected safe policy: %+v", policy)
	}

	low := ResolveWithPolicy("git status", policy)
	if low.Kind != RouteExternalCLI || low.Risk != RiskLow {
		t.Fatalf("git status route = %+v", low)
	}
	high := ResolveWithPolicy("git reset --hard", policy)
	if high.Kind != RouteRejected || high.Risk != RiskHigh {
		t.Fatalf("git reset route = %+v", high)
	}
	medium := ResolveWithPolicy("git fetch", DefaultPolicy())
	if medium.Kind != RouteExternalCLI || medium.Risk != RiskMedium {
		t.Fatalf("git fetch route = %+v", medium)
	}
	denied := ResolveWithPolicy("go version", Policy{DeniedExternal: []string{"go"}, AllowExternal: true})
	if denied.Kind != RouteRejected {
		t.Fatalf("denied route = %+v", denied)
	}

	if commandRisk("rm", []string{"file"}) != RiskHigh ||
		commandRisk("curl", []string{"https://example.com"}) != RiskMedium ||
		commandRisk("go", []string{"install"}) != RiskHigh ||
		commandRisk("go", []string{"build"}) != RiskMedium ||
		commandRisk("unknown", []string{"--force"}) != RiskHigh {
		t.Fatalf("unexpected command risk result")
	}
}

func TestRouteFunctionAndRejectedInput(t *testing.T) {
	if err := Route("echo route-ok"); err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if err := Route("echo nope | wc -l"); err == nil {
		t.Fatalf("expected rejected route")
	}
}

func TestRouteDefaultCodexBackendWithFakeBinary(t *testing.T) {
	dir := t.TempDir()
	fakeCodex := dir + "/codex"
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOSH_CODEX_BIN", fakeCodex)

	if err := RouteWithOptions(context.Background(), "not-a-command-for-fake-codex", RouteOptions{}); err != nil {
		t.Fatalf("RouteWithOptions returned error: %v", err)
	}
}

func TestMenuHelpToolsResolveAndServeMCP(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"goshfile"}
	var help bytes.Buffer
	MenuWithOptions(MenuOptions{Stdout: &help})
	if !strings.Contains(help.String(), "tools --json") {
		t.Fatalf("help output = %q", help.String())
	}

	os.Args = []string{"goshfile", "tools", "--json"}
	var toolsOut bytes.Buffer
	MenuWithOptions(MenuOptions{Stdout: &toolsOut})
	var tools []ToolInfo
	if err := json.Unmarshal(toolsOut.Bytes(), &tools); err != nil {
		t.Fatalf("tools output is not json: %v", err)
	}

	os.Args = []string{"goshfile", "--resolve", "git", "status"}
	var resolveOut bytes.Buffer
	MenuWithOptions(MenuOptions{Stdout: &resolveOut})
	if !strings.Contains(resolveOut.String(), `"kind": "external_cli"`) {
		t.Fatalf("resolve output = %q", resolveOut.String())
	}

	os.Args = []string{"goshfile", "serve", "mcp"}
	var mcpOut bytes.Buffer
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}()
	MenuWithOptions(MenuOptions{
		Stdout: &mcpOut,
		Stderr: &bytes.Buffer{},
	})
	if strings.TrimSpace(mcpOut.String()) != "" {
		t.Fatalf("empty MCP stdin should not write output: %q", mcpOut.String())
	}
}

func TestDefaultWriter(t *testing.T) {
	var out bytes.Buffer
	if defaultWriter(&out, os.Stdout) != &out {
		t.Fatalf("explicit writer not returned")
	}
	if defaultWriter(nil, os.Stdout) != os.Stdout {
		t.Fatalf("fallback writer not returned")
	}
}
