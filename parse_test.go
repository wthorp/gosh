package gosh

import "testing"

func TestSplitArgsHandlesQuotes(t *testing.T) {
	args, err := SplitArgs(`git commit -m "hello world"`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}

	want := []string{"git", "commit", "-m", "hello world"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestSplitArgsPreservesEmptyQuotedArgs(t *testing.T) {
	args, err := SplitArgs(`cmd "" ''`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	if len(args) != 3 {
		t.Fatalf("args = %v, want three args", args)
	}
	if args[1] != "" || args[2] != "" {
		t.Fatalf("empty args were not preserved: %v", args)
	}
}

func TestSplitArgsRejectsShellOperators(t *testing.T) {
	if _, err := SplitArgs(`git status | wc -l`); err == nil {
		t.Fatalf("expected shell operator error")
	}
}

func TestSplitArgsRejectsUnterminatedQuote(t *testing.T) {
	if _, err := SplitArgs(`echo "oops`); err == nil {
		t.Fatalf("expected unterminated quote error")
	}
}

func TestSplitArgsPreservesLiteralBackslashes(t *testing.T) {
	args, err := SplitArgs(`cmd C:\temp\foo a\b`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	want := []string{"cmd", `C:\temp\foo`, `a\b`}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestSplitArgsPreservesQuotedWindowsPath(t *testing.T) {
	args, err := SplitArgs(`cmd "C:\temp\foo bar"`)
	if err != nil {
		t.Fatalf("SplitArgs returned error: %v", err)
	}
	if len(args) != 2 || args[1] != `C:\temp\foo bar` {
		t.Fatalf("args = %v", args)
	}
}
