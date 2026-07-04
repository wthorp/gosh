package gosh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testScript(dir string) *Script {
	return &Script{
		dirs: []string{dir},
		env: map[string]string{
			"PATH": os.Getenv("PATH"),
		},
		onErr: func(error) {},
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

func TestRunCmdsUsesLegacyBuiltinsAndContinuesAfterError(t *testing.T) {
	dir := t.TempDir()
	var gotErrs []error
	script := &Script{
		cmds: []string{
			"# comment",
			"",
			"set name = World",
			"echo Hello ${name}",
			"mkdir sub",
			"cd sub",
			"bad-command-that-does-not-exist",
			"echo still-runs",
		},
		dirs: []string{dir},
		env: map[string]string{
			"PATH": os.Getenv("PATH"),
		},
		onErr: func(err error) {
			gotErrs = append(gotErrs, err)
		},
	}

	output, err := captureStdout(func() error {
		script.RunCmds()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "Hello World") || !strings.Contains(output, "still-runs") {
		t.Fatalf("output = %q", output)
	}
	if len(gotErrs) != 1 || !strings.Contains(gotErrs[0].Error(), "bad-command-that-does-not-exist") {
		t.Fatalf("expected command error, got %v", gotErrs)
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
