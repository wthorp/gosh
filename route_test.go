package gosh

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

var _ = Cmd("GoshRouteTestCommand", func(value string) {})

type recordingBackend struct {
	called bool
	input  string
}

func (b *recordingBackend) Run(_ context.Context, input string) error {
	b.called = true
	b.input = input
	return nil
}

func TestResolveMatchesGoshCommand(t *testing.T) {
	result := Resolve("goshroutetestcommand hello")
	if result.Kind != RouteGoshCommand {
		t.Fatalf("kind = %s, want %s", result.Kind, RouteGoshCommand)
	}
	if result.Command != "GoshRouteTestCommand" {
		t.Fatalf("command = %q", result.Command)
	}
	if len(result.Args) != 1 || result.Args[0] != "hello" {
		t.Fatalf("args = %v", result.Args)
	}
}

func TestResolveMatchesExternalCLI(t *testing.T) {
	result := Resolve("go version")
	if result.Kind != RouteExternalCLI {
		t.Fatalf("kind = %s, want %s; reason=%s", result.Kind, RouteExternalCLI, result.Reason)
	}
	if result.Command != "go" {
		t.Fatalf("command = %q", result.Command)
	}
	if result.Executable == "" {
		t.Fatalf("expected executable path")
	}
}

func TestResolveRejectsExternalDisallowedByPolicy(t *testing.T) {
	result := ResolveWithPolicy("go version", Policy{
		AllowExternal:   false,
		AllowedExternal: []string{"git"},
	})
	if result.Kind != RouteRejected {
		t.Fatalf("kind = %s, want %s", result.Kind, RouteRejected)
	}
}

func TestResolveNeedsAIForUnknownInput(t *testing.T) {
	result := Resolve("gosh_missing_command_for_ai please do something")
	if result.Kind != RouteNeedsAI {
		t.Fatalf("kind = %s, want %s; reason=%s", result.Kind, RouteNeedsAI, result.Reason)
	}
}

func TestRouteFallsBackToBackendForNeedsAI(t *testing.T) {
	backend := &recordingBackend{}
	input := "gosh_missing_command_for_ai please do something"

	err := RouteWithOptions(context.Background(), input, RouteOptions{
		Policy:  DefaultPolicy(),
		Backend: backend,
	})
	if err != nil {
		t.Fatalf("RouteWithOptions returned error: %v", err)
	}
	if !backend.called {
		t.Fatalf("expected backend to be called")
	}
	if backend.input != input {
		t.Fatalf("backend input = %q, want %q", backend.input, input)
	}
}

func TestRouteDoesNotSendRejectedInputToBackend(t *testing.T) {
	backend := &recordingBackend{}

	err := RouteWithOptions(context.Background(), "git status | wc -l", RouteOptions{
		Policy:  DefaultPolicy(),
		Backend: backend,
	})
	if err == nil {
		t.Fatalf("expected rejected input error")
	}
	if backend.called {
		t.Fatalf("backend should not be called for rejected input")
	}
}

func TestMenuResolveWritesJSON(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"goshfile", "--resolve", "goshroutetestcommand", "hello"}
	var out bytes.Buffer
	MenuWithOptions(MenuOptions{Policy: DefaultPolicy(), Stdout: &out})

	var result RouteResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if result.Kind != RouteGoshCommand {
		t.Fatalf("kind = %s, want %s", result.Kind, RouteGoshCommand)
	}
}
