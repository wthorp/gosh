package gosh

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCodexBackendReadsEnvironment(t *testing.T) {
	t.Setenv("GOSH_CODEX_BIN", "custom-codex")
	t.Setenv("GOSH_CODEX_MODEL", "gpt-test")
	t.Setenv("GOSH_CODEX_SANDBOX", "read-only")
	t.Setenv("GOSH_CODEX_APPROVAL", "on-request")
	t.Setenv("GOSH_CODEX_ARGS", `--search --profile "work profile"`)

	backend := DefaultCodexBackend()
	if backend.Binary != "custom-codex" {
		t.Fatalf("binary = %q", backend.Binary)
	}
	if backend.Model != "gpt-test" || backend.Sandbox != "read-only" || backend.Approval != "on-request" {
		t.Fatalf("unexpected backend config: %+v", backend)
	}
	wantArgs := []string{"--search", "--profile", "work profile"}
	if strings.Join(backend.Args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args = %#v, want %#v", backend.Args, wantArgs)
	}
}

func TestDefaultCodexBackendDefaults(t *testing.T) {
	t.Setenv("GOSH_CODEX_BIN", "")
	t.Setenv("GOSH_CODEX_MODEL", "")
	t.Setenv("GOSH_CODEX_SANDBOX", "")
	t.Setenv("GOSH_CODEX_APPROVAL", "")
	t.Setenv("GOSH_CODEX_ARGS", "")

	backend := DefaultCodexBackend()
	if backend.Binary != "codex" {
		t.Fatalf("binary = %q", backend.Binary)
	}
	if backend.Sandbox != "workspace-write" || backend.Approval != "never" {
		t.Fatalf("defaults = sandbox %q approval %q", backend.Sandbox, backend.Approval)
	}
	if len(backend.Args) != 0 {
		t.Fatalf("args = %#v", backend.Args)
	}
}

func TestCodexBackendRunBuildsCommand(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	fakeCodex := filepath.Join(dir, "codex")
	script := "#!/bin/sh\n: > \"$GOSH_FAKE_CODEX_ARGS\"\nfor arg in \"$@\"; do printf '%s\\n' \"$arg\" >> \"$GOSH_FAKE_CODEX_ARGS\"; done\n"
	if err := os.WriteFile(fakeCodex, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOSH_FAKE_CODEX_ARGS", argsFile)

	var stdout bytes.Buffer
	backend := CodexBackend{
		Binary:   fakeCodex,
		Dir:      dir,
		Model:    "gpt-test",
		Sandbox:  "read-only",
		Approval: "never",
		Args:     []string{"--search"},
		Stdout:   &stdout,
		Stderr:   &stdout,
	}
	if err := backend.Run(context.Background(), "explain the repo"); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	wantPrefix := []string{"exec", "--model", "gpt-test", "--cd", dir, "--sandbox", "read-only", "--ask-for-approval", "never", "--search"}
	if len(args) < len(wantPrefix)+1 {
		t.Fatalf("args too short: %#v", args)
	}
	for i, want := range wantPrefix {
		if args[i] != want {
			t.Fatalf("arg %d = %q, want %q\nall args: %#v", i, args[i], want, args)
		}
	}
	if !strings.Contains(args[len(args)-1], "explain the repo") {
		t.Fatalf("prompt arg missing input: %q", args[len(args)-1])
	}
}

func TestCodexBackendRunReportsMissingBinary(t *testing.T) {
	backend := CodexBackend{Binary: filepath.Join(t.TempDir(), "missing-codex")}
	if err := backend.Run(context.Background(), "anything"); err == nil {
		t.Fatalf("expected missing binary error")
	}
}

func TestCodexPromptAndSplitEnvArgs(t *testing.T) {
	if !strings.Contains(codexPrompt("do work"), "do work") {
		t.Fatalf("prompt did not include input")
	}
	if args := splitEnvArgs(`--flag "two words"`); len(args) != 2 || args[1] != "two words" {
		t.Fatalf("splitEnvArgs = %#v", args)
	}
	if args := splitEnvArgs(`unterminated "`); args != nil {
		t.Fatalf("invalid split should return nil, got %#v", args)
	}
	if envOrDefault("GOSH_MISSING_TEST_ENV", "fallback") != "fallback" {
		t.Fatalf("env fallback not used")
	}
}
