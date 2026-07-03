package gosh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type scriptBackend struct {
	inputs []string
	err    error
}

func (b *scriptBackend) Run(_ context.Context, input string) error {
	b.inputs = append(b.inputs, input)
	return b.err
}

func testScript(dir string) *Script {
	return &Script{
		dirs: []string{dir},
		env: map[string]string{
			"PATH": os.Getenv("PATH"),
		},
		onErr: func(error) {},
	}
}

func TestRunAgenticFallbackUsesExpandedInput(t *testing.T) {
	dir := t.TempDir()
	backend := &scriptBackend{}
	var gotErr error
	script := &Script{
		cmds: []string{
			"set yinz = World",
			"display hello ${yinz} on the screen",
			"echo after fallback",
		},
		dirs:          []string{dir},
		env:           map[string]string{"PATH": os.Getenv("PATH")},
		AgentFallback: true,
		Backend:       backend,
		onErr: func(err error) {
			gotErr = err
		},
	}

	output, err := captureStdout(func() error {
		script.RunCmds()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotErr != nil {
		t.Fatalf("unexpected error: %v", gotErr)
	}
	if len(backend.inputs) != 1 || backend.inputs[0] != "display hello World on the screen" {
		t.Fatalf("backend inputs = %#v", backend.inputs)
	}
	if !strings.Contains(output, "after fallback") {
		t.Fatalf("script did not continue after fallback: %q", output)
	}
}

func TestRunAgenticEAndStrictRunE(t *testing.T) {
	fakeCodex := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOSH_CODEX_BIN", fakeCodex)

	if err := RunAgenticE(`
		set yinz = World
		display hello ${yinz} on the screen
	`); err != nil {
		t.Fatalf("RunAgenticE returned error: %v", err)
	}
	if err := RunE("display hello World on the screen"); err == nil {
		t.Fatalf("RunE should remain strict without agent fallback")
	}
}

func TestScriptBuiltins(t *testing.T) {
	dir := t.TempDir()
	script := testScript(dir)

	if err := script.set("name = gosh"); err != nil {
		t.Fatal(err)
	}
	if script.env["name"] != "gosh" {
		t.Fatalf("set env = %q", script.env["name"])
	}

	output, err := captureStdout(func() error {
		return script.echo("hello " + script.env["name"])
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(output) != "hello gosh" {
		t.Fatalf("echo output = %q", output)
	}

	if script.getwd() != dir {
		t.Fatalf("getwd = %q, want %q", script.getwd(), dir)
	}
	if err := script.mkDir("sub"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := script.cd("sub"); err != nil {
		t.Fatal(err)
	}
	if script.getwd() != filepath.Join(dir, "sub") {
		t.Fatalf("cd dir = %q", script.getwd())
	}

	previous := script.getwd()
	if err := script.pushd(dir); err != nil {
		t.Fatal(err)
	}
	if script.getwd() != dir {
		t.Fatalf("pushd dir = %q", script.getwd())
	}
	if err := script.popd(""); err != nil {
		t.Fatal(err)
	}
	if script.getwd() != previous {
		t.Fatalf("popd dir = %q, want %q", script.getwd(), previous)
	}
	if err := script.popd(""); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(script.getwd(), "delete-me.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := script.rm("delete-me.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("rm did not remove file: %v", err)
	}

	if err := script.mkDir("gone"); err != nil {
		t.Fatal(err)
	}
	if err := script.rmDir("gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(script.getwd(), "gone")); !os.IsNotExist(err) {
		t.Fatalf("rmDir did not remove dir: %v", err)
	}
}

func TestRunCmdsUsesLegacyBuiltinsAndStopsOnError(t *testing.T) {
	dir := t.TempDir()
	var gotErr error
	script := &Script{
		cmds: []string{
			"# comment",
			"",
			"set name = World",
			"echo Hello ${name}",
			"mkdir sub",
			"cd sub",
			"bad-command-that-does-not-exist",
			"echo should-not-run",
		},
		dirs: []string{dir},
		env: map[string]string{
			"PATH": os.Getenv("PATH"),
		},
		onErr: func(err error) {
			gotErr = err
		},
	}

	output, err := captureStdout(func() error {
		script.RunCmds()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Hello World") {
		t.Fatalf("output = %q", output)
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "bad-command-that-does-not-exist") {
		t.Fatalf("expected command error, got %v", gotErr)
	}
	if script.getwd() != filepath.Join(dir, "sub") {
		t.Fatalf("script dir = %q", script.getwd())
	}
}

func TestScriptRunAndRunE(t *testing.T) {
	output, err := captureStdout(func() error {
		Run("echo from-run")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "from-run") {
		t.Fatalf("Run output = %q", output)
	}

	if err := RunE("bad-command-that-does-not-exist"); err == nil {
		t.Fatalf("expected RunE error")
	}
}

func TestScriptMethodRunAndExec(t *testing.T) {
	dir := t.TempDir()
	var gotErr error
	script := &Script{
		dirs:  []string{dir},
		env:   map[string]string{"PATH": os.Getenv("PATH")},
		onErr: func(err error) { gotErr = err },
	}

	output, err := captureStdout(func() error {
		script.Run("echo from-method")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "from-method") || gotErr != nil {
		t.Fatalf("output=%q err=%v", output, gotErr)
	}
	if err := script.Exec(""); err != nil {
		t.Fatalf("empty exec returned error: %v", err)
	}
	if err := script.Exec("echo from-exec"); err != nil {
		t.Fatalf("exec returned error: %v", err)
	}
	if err := script.Exec("echo nope | wc -l"); err == nil {
		t.Fatalf("expected shell operator parse error")
	}
}
