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
	runes := []rune(strings.TrimSpace(input))
	var args []string
	var current strings.Builder
	var quote rune
	argStarted := false

	flush := func() {
		if argStarted {
			args = append(args, current.String())
			current.Reset()
			argStarted = false
		}
	}

	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if ch == '\\' {
			if i+1 >= len(runes) {
				current.WriteRune('\\')
				argStarted = true
				continue
			}

			next := runes[i+1]
			if quote != 0 {
				if next == quote || next == '\\' {
					current.WriteRune(next)
					argStarted = true
					i++
					continue
				}
				current.WriteRune('\\')
				argStarted = true
				continue
			}

			if isEscapableDirectArgRune(next) {
				current.WriteRune(next)
				argStarted = true
				i++
				continue
			}

			current.WriteRune('\\')
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

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	flush()
	return args, nil
}

func isEscapableDirectArgRune(ch rune) bool {
	switch ch {
	case '\'', '"', '\\', ' ', '\t', '\n', '\r', '|', ';', '&', '<', '>', '`':
		return true
	default:
		return false
	}
}
