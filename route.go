package gosh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RouteKind describes how an input line should be handled.
type RouteKind string

const (
	RouteGoshCommand RouteKind = "gosh_command"
	RouteExternalCLI RouteKind = "external_cli"
	RouteNeedsAI     RouteKind = "needs_ai"
	RouteRejected    RouteKind = "rejected"
)

// RiskLevel is a coarse execution risk estimate for command routing.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// RouteResult is the deterministic classification for one input line.
type RouteResult struct {
	Kind             RouteKind `json:"kind"`
	Input            string    `json:"input,omitempty"`
	Command          string    `json:"command,omitempty"`
	Args             []string  `json:"args,omitempty"`
	Executable       string    `json:"executable,omitempty"`
	Confidence       float64   `json:"confidence"`
	Valid            bool      `json:"valid"`
	Risk             RiskLevel `json:"risk,omitempty"`
	RequiresApproval bool      `json:"requires_approval,omitempty"`
	Reason           string    `json:"reason,omitempty"`
	ValidationErrors []string  `json:"validation_errors,omitempty"`
}

// Policy controls which external commands are considered routable.
type Policy struct {
	AllowExternal bool

	// AllowedExternal and DeniedExternal are compared to the executable base
	// name, case-insensitively. When AllowedExternal is non-empty, only those
	// commands are allowed.
	AllowedExternal []string
	DeniedExternal  []string

	// RejectHighRiskExternal rejects high-risk external commands instead of
	// routing them. Gosh commands are still classified with risk, but are not
	// blocked by this policy because callers may attach their own approval flow.
	RejectHighRiskExternal bool
}

// DefaultPolicy preserves Gosh's historical behavior: registered commands and
// direct executables are routable. It still reports risk so callers can decide
// whether confirmation is needed.
func DefaultPolicy() Policy {
	return Policy{AllowExternal: true}
}

// SafePolicy routes only common inspection commands and rejects high-risk ones.
func SafePolicy() Policy {
	return Policy{
		AllowExternal:          false,
		AllowedExternal:        []string{"cat", "git", "ls", "pwd", "rg", "wc"},
		RejectHighRiskExternal: true,
	}
}

// Resolve classifies input using DefaultPolicy.
func Resolve(input string) RouteResult {
	return ResolveWithPolicy(input, DefaultPolicy())
}

// ResolveWithPolicy classifies input without executing it.
func ResolveWithPolicy(input string, policy Policy) RouteResult {
	input = strings.TrimSpace(input)
	if input == "" {
		return RouteResult{
			Kind:       RouteRejected,
			Input:      input,
			Confidence: 1,
			Valid:      false,
			Reason:     "empty input",
		}
	}
	if strings.ContainsAny(input, "\n\r") {
		return RouteResult{
			Kind:       RouteRejected,
			Input:      input,
			Confidence: 1,
			Valid:      false,
			Reason:     "route input must be a single line",
		}
	}

	args, err := SplitArgs(input)
	if err != nil {
		return RouteResult{
			Kind:       RouteRejected,
			Input:      input,
			Confidence: 1,
			Valid:      false,
			Reason:     err.Error(),
		}
	}
	if len(args) == 0 {
		return RouteResult{
			Kind:       RouteRejected,
			Input:      input,
			Confidence: 1,
			Valid:      false,
			Reason:     "empty input",
		}
	}

	command := args[0]
	rest := args[1:]
	if call, ok := Calls[strings.ToLower(command)]; ok {
		validation := validateToolArgs(call.Tool, rest)
		if !validation.Valid {
			return RouteResult{
				Kind:             RouteRejected,
				Input:            input,
				Command:          call.Name,
				Args:             rest,
				Confidence:       1,
				Valid:            false,
				Risk:             call.Tool.Risk,
				RequiresApproval: call.Tool.RequiresApproval,
				Reason:           "invalid arguments",
				ValidationErrors: validation.Errors,
			}
		}
		return RouteResult{
			Kind:             RouteGoshCommand,
			Input:            input,
			Command:          call.Name,
			Args:             rest,
			Confidence:       1,
			Valid:            true,
			Risk:             call.Tool.Risk,
			RequiresApproval: call.Tool.RequiresApproval,
			Reason:           "matched registered gosh command",
		}
	}

	executable, err := exec.LookPath(command)
	if err == nil {
		risk := commandRisk(filepath.Base(command), rest)
		if !policy.externalAllowed(command) {
			return RouteResult{
				Kind:       RouteRejected,
				Input:      input,
				Command:    command,
				Args:       rest,
				Executable: executable,
				Confidence: 1,
				Valid:      false,
				Risk:       risk,
				Reason:     "external command is not allowed by policy",
			}
		}
		if policy.RejectHighRiskExternal && risk == RiskHigh {
			return RouteResult{
				Kind:       RouteRejected,
				Input:      input,
				Command:    command,
				Args:       rest,
				Executable: executable,
				Confidence: 1,
				Valid:      false,
				Risk:       risk,
				Reason:     "high-risk external command rejected by policy",
			}
		}
		return RouteResult{
			Kind:       RouteExternalCLI,
			Input:      input,
			Command:    command,
			Args:       rest,
			Executable: executable,
			Confidence: 1,
			Valid:      true,
			Risk:       risk,
			Reason:     "matched executable on PATH",
		}
	}

	return RouteResult{
		Kind:       RouteNeedsAI,
		Input:      input,
		Command:    command,
		Args:       rest,
		Confidence: 0.25,
		Valid:      false,
		Reason:     "no registered gosh command or executable matched",
	}
}

// AIBackend handles inputs that cannot be routed deterministically.
type AIBackend interface {
	Run(ctx context.Context, input string) error
}

// RouteOptions configures RouteWithOptions.
type RouteOptions struct {
	Policy  Policy
	Backend AIBackend
	Stdout  io.Writer
	Stderr  io.Writer
}

// Route routes one input line. Deterministic commands run directly; unmatched
// inputs are sent to the configured AI backend.
func Route(input string) error {
	return RouteWithOptions(context.Background(), input, RouteOptions{})
}

// RouteWithOptions routes one input line with explicit options.
func RouteWithOptions(ctx context.Context, input string, options RouteOptions) error {
	policy := options.Policy
	if !policy.configured() {
		policy = DefaultPolicy()
	}

	result := ResolveWithPolicy(input, policy)
	switch result.Kind {
	case RouteGoshCommand, RouteExternalCLI:
		return RunE(input)
	case RouteNeedsAI:
		backend := options.Backend
		if backend == nil {
			codex := DefaultCodexBackend()
			codex.Stdout = defaultWriter(options.Stdout, os.Stdout)
			codex.Stderr = defaultWriter(options.Stderr, os.Stderr)
			backend = codex
		}
		return backend.Run(ctx, input)
	case RouteRejected:
		return fmt.Errorf("input rejected: %s", result.Reason)
	default:
		return fmt.Errorf("unknown route kind: %s", result.Kind)
	}
}

func (p Policy) configured() bool {
	return p.AllowExternal ||
		len(p.AllowedExternal) > 0 ||
		len(p.DeniedExternal) > 0 ||
		p.RejectHighRiskExternal
}

func (p Policy) externalAllowed(command string) bool {
	name := strings.ToLower(filepath.Base(command))
	for _, denied := range p.DeniedExternal {
		if name == strings.ToLower(denied) {
			return false
		}
	}
	if len(p.AllowedExternal) > 0 {
		for _, allowed := range p.AllowedExternal {
			if name == strings.ToLower(allowed) {
				return true
			}
		}
		return false
	}
	return p.AllowExternal
}

func commandRisk(command string, args []string) RiskLevel {
	name := strings.ToLower(filepath.Base(command))

	for _, arg := range args {
		arg = strings.ToLower(arg)
		if arg == "-rf" || arg == "-fr" || strings.Contains(arg, "--force") {
			return RiskHigh
		}
	}

	switch name {
	case "rm", "rmdir", "mkfs", "shutdown", "reboot":
		return RiskHigh
	case "curl", "wget", "ssh", "scp", "rsync", "docker", "kubectl", "terraform":
		return RiskMedium
	case "find":
		return findRisk(args)
	case "sed":
		return sedRisk(args)
	case "git":
		return gitRisk(args)
	case "go", "npm", "yarn", "pnpm", "make":
		return buildToolRisk(args)
	default:
		return RiskLow
	}
}

func gitRisk(args []string) RiskLevel {
	if len(args) == 0 {
		return RiskLow
	}
	switch strings.ToLower(args[0]) {
	case "status", "diff", "log", "show", "branch", "rev-parse":
		return RiskLow
	case "push", "commit", "merge", "rebase", "checkout", "switch", "reset", "clean":
		return RiskHigh
	default:
		return RiskMedium
	}
}

func findRisk(args []string) RiskLevel {
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "-delete", "-exec", "-execdir", "-ok", "-okdir":
			return RiskHigh
		}
	}
	return RiskLow
}

func sedRisk(args []string) RiskLevel {
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if lower == "-i" || strings.HasPrefix(lower, "-i.") || strings.HasPrefix(lower, "-i") {
			return RiskHigh
		}
	}
	return RiskMedium
}

func buildToolRisk(args []string) RiskLevel {
	if len(args) == 0 {
		return RiskLow
	}
	first := strings.ToLower(args[0])
	switch first {
	case "version", "env", "list":
		return RiskLow
	case "test":
		return RiskMedium
	case "install", "add", "remove", "publish", "deploy":
		return RiskHigh
	default:
		return RiskMedium
	}
}

func writeRouteJSON(w io.Writer, result RouteResult) error {
	return writeJSON(w, result)
}

func writeJSON(w io.Writer, value interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func defaultWriter(w io.Writer, fallback *os.File) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}
