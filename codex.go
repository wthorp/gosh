package gosh

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CodexBackend sends non-deterministic inputs to `codex exec`.
type CodexBackend struct {
	Binary   string
	Dir      string
	Model    string
	Sandbox  string
	Approval string
	Args     []string
	Stdout   io.Writer
	Stderr   io.Writer
}

// DefaultCodexBackend returns a Codex backend configured from the environment.
//
// Supported environment variables:
//   - GOSH_CODEX_BIN
//   - GOSH_CODEX_MODEL
//   - GOSH_CODEX_SANDBOX
//   - GOSH_CODEX_APPROVAL
//   - GOSH_CODEX_ARGS
func DefaultCodexBackend() CodexBackend {
	return CodexBackend{
		Binary:   envOrDefault("GOSH_CODEX_BIN", "codex"),
		Model:    os.Getenv("GOSH_CODEX_MODEL"),
		Sandbox:  envOrDefault("GOSH_CODEX_SANDBOX", "workspace-write"),
		Approval: envOrDefault("GOSH_CODEX_APPROVAL", "never"),
		Args:     splitEnvArgs(os.Getenv("GOSH_CODEX_ARGS")),
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
	}
}

// Run invokes Codex for one unmatched input.
func (b CodexBackend) Run(ctx context.Context, input string) error {
	binary := b.Binary
	if binary == "" {
		binary = "codex"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("codex backend %q not found: %w", binary, err)
	}

	dir := b.Dir
	if dir == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = workingDir
	}

	args := []string{"exec"}
	if b.Model != "" {
		args = append(args, "--model", b.Model)
	}
	if dir != "" {
		args = append(args, "--cd", dir)
	}
	if b.Sandbox != "" {
		args = append(args, "--sandbox", b.Sandbox)
	}
	if b.Approval != "" {
		args = append(args, "-c", fmt.Sprintf("approval_policy=%q", b.Approval))
	}
	args = append(args, b.Args...)
	args = append(args, codexPrompt(input))

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = defaultWriter(b.Stdout, os.Stdout)
	cmd.Stderr = defaultWriter(b.Stderr, os.Stderr)
	cmd.Stdin = os.Stdin
	cmd.Dir = dir
	return cmd.Run()
}

func codexPrompt(input string) string {
	return strings.TrimSpace(`This input did not match a registered gosh command or an executable command on PATH.
Handle it as the user's request for the current repository or working directory.

User input:
` + input)
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func splitEnvArgs(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	args, err := SplitArgs(input)
	if err != nil {
		return nil
	}
	return args
}
