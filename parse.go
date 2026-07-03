package gosh

import (
	"fmt"
	"strings"
)

// SplitArgs splits a single command line into argv-style tokens.
//
// Gosh executes programs directly instead of through a shell. Shell control
// operators are therefore rejected here so routed CLI commands stay explicit.
func SplitArgs(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	argStarted := false

	flush := func() {
		if argStarted {
			args = append(args, current.String())
			current.Reset()
			argStarted = false
		}
	}

	for _, ch := range strings.TrimSpace(input) {
		if escaped {
			current.WriteRune(ch)
			argStarted = true
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			argStarted = true
			continue
		}

		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteRune(ch)
			argStarted = true
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
			argStarted = true
		case ' ', '\t', '\n', '\r':
			flush()
		case '|', ';', '&', '<', '>', '`':
			return nil, fmt.Errorf("shell operator %q is not supported by direct command routing", ch)
		default:
			current.WriteRune(ch)
			argStarted = true
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	flush()
	return args, nil
}
