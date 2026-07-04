package gosh

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSafePolicyAndRiskBranches(t *testing.T) {
	policy := SafePolicy()
	if !policy.RejectHighRiskExternal || policy.MaxExternalRisk != RiskLow || len(policy.AllowedExternal) == 0 {
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
	disallowed := ResolveWithPolicy("go version", policy)
	if disallowed.Kind != RouteRejected {
		t.Fatalf("go version route = %+v", disallowed)
	}
	medium := ResolveWithPolicy("git fetch", DefaultPolicy())
	if medium.Kind != RouteExternalCLI || medium.Risk != RiskMedium {
		t.Fatalf("git fetch route = %+v", medium)
	}
	denied := ResolveWithPolicy("go version", Policy{DeniedExternal: []string{"go"}, AllowExternal: true})
	if denied.Kind != RouteRejected {
		t.Fatalf("denied route = %+v", denied)
	}
	safeDenied := ResolveWithPolicy("go version", policy)
	if safeDenied.Kind != RouteRejected {
		t.Fatalf("safe go route = %+v", safeDenied)
	}
	findDelete := ResolveWithPolicy("find . -delete", policy)
	if findDelete.Kind != RouteRejected || findDelete.Risk != RiskHigh {
		t.Fatalf("find delete route = %+v", findDelete)
	}
	gitFetchSafe := ResolveWithPolicy("git fetch", policy)
	if gitFetchSafe.Kind != RouteRejected || gitFetchSafe.Risk != RiskMedium {
		t.Fatalf("safe git fetch route = %+v", gitFetchSafe)
	}
	sedInPlace := ResolveWithPolicy("sed -i s/a/b/ file", policy)
	if sedInPlace.Kind != RouteRejected || sedInPlace.Risk != RiskHigh {
		t.Fatalf("sed in-place route = %+v", sedInPlace)
	}
	gitBranchDelete := ResolveWithPolicy("git branch -D old", policy)
	if gitBranchDelete.Kind != RouteRejected || gitBranchDelete.Risk != RiskHigh {
		t.Fatalf("git branch delete route = %+v", gitBranchDelete)
	}
	gitBranchCreate := ResolveWithPolicy("git branch new", policy)
	if gitBranchCreate.Kind != RouteRejected || gitBranchCreate.Risk != RiskMedium {
		t.Fatalf("git branch create route = %+v", gitBranchCreate)
	}

	if commandRisk("rm", []string{"file"}) != RiskHigh ||
		commandRisk("find", []string{".", "-delete"}) != RiskHigh ||
		commandRisk("sed", []string{"-i", "s/a/b/", "file"}) != RiskHigh ||
		commandRisk("curl", []string{"https://example.com"}) != RiskMedium ||
		commandRisk("git", []string{"apply", "patch.diff"}) != RiskHigh ||
		commandRisk("git", []string{"branch", "-D", "old"}) != RiskHigh ||
		commandRisk("git", []string{"branch", "new"}) != RiskMedium ||
		commandRisk("go", []string{"install"}) != RiskHigh ||
		commandRisk("go", []string{"build"}) != RiskHigh ||
		commandRisk("go", []string{"test"}) != RiskHigh ||
		commandRisk("go", []string{"version"}) != RiskLow ||
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

func TestRouteRejectsMultilineInputBeforeExecution(t *testing.T) {
	target := "route-newline-probe"
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(target) }()

	err := RouteWithOptions(context.Background(), "git status\nrm "+target, RouteOptions{
		Policy: Policy{AllowExternal: true, RejectHighRiskExternal: true},
	})
	if err == nil || !strings.Contains(err.Error(), "single line") {
		t.Fatalf("RouteWithOptions error = %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target should not be touched: %v", err)
	}

	result := Resolve("git status\nrm " + target)
	if result.Kind != RouteRejected || !strings.Contains(result.Reason, "single line") {
		t.Fatalf("resolve result = %+v", result)
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

func TestRouteWithOptionsZeroPolicyRejectsExternal(t *testing.T) {
	if err := RouteWithOptions(context.Background(), "go version", RouteOptions{PolicySet: true}); err == nil {
		t.Fatalf("expected zero-value policy to reject external commands")
	}
	if err := RouteWithOptions(context.Background(), "go version", RouteOptions{}); err != nil {
		t.Fatalf("expected zero-value options to use default policy: %v", err)
	}
}

func TestRouteWithOptionsHonorsContextForExternalCommands(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow-route-context-test")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := RouteWithOptions(ctx, "slow-route-context-test", RouteOptions{
		Policy: Policy{AllowExternal: true},
	})
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("external command was not canceled promptly")
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
	MenuWithOptions(MenuOptions{Policy: DefaultPolicy(), Stdout: &resolveOut})
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

func TestMenuRejectsInvalidMetaCommands(t *testing.T) {
	oldArgs := os.Args
	oldDefaultErr := defaultErr
	defer func() {
		os.Args = oldArgs
		defaultErr = oldDefaultErr
	}()

	backend := &recordingBackend{}
	cases := [][]string{
		{"goshfile", "tools"},
		{"goshfile", "serve"},
		{"goshfile", "serve", "mcpx"},
	}

	for _, args := range cases {
		os.Args = args
		var gotErr error
		defaultErr = func(err error) { gotErr = err }

		MenuWithOptions(MenuOptions{
			Policy:  DefaultPolicy(),
			Backend: backend,
			Stdout:  &bytes.Buffer{},
			Stderr:  &bytes.Buffer{},
		})

		if gotErr == nil || !strings.Contains(gotErr.Error(), "invalid meta command") {
			t.Fatalf("args=%v err=%v", args, gotErr)
		}
		if backend.called {
			t.Fatalf("backend should not be called for invalid meta command %v", args)
		}
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
