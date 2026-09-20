package safety

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

func validateCommand(ctx context.Context, program string, args []string) error {
	if ctx == nil {
		return errors.New("command context is nil")
	}
	if strings.TrimSpace(program) == "" {
		return errors.New("command program is empty")
	}
	if strings.IndexByte(program, 0) >= 0 {
		return errors.New("command program contains NUL")
	}
	for _, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return errors.New("command argument contains NUL")
		}
	}
	return nil
}

// RunCommand executes a program without a shell, captures combined stdout/stderr,
// and caps the returned output to maxOutput bytes.
// CommandContext creates a validated command without a shell for callers that
// need streaming I/O while preserving the shared RouterForge safety boundary.
func CommandContext(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	if err := validateCommand(ctx, program, args); err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, program, args...), nil
}

func RunCommand(ctx context.Context, maxOutput int, program string, args ...string) ([]byte, error) {
	if err := validateCommand(ctx, program, args); err != nil {
		return nil, err
	}
	if maxOutput < 0 {
		return nil, errors.New("command output limit is negative")
	}

	output, err := exec.CommandContext(ctx, program, args...).CombinedOutput()
	if len(output) > maxOutput {
		output = output[:maxOutput]
	}
	return output, err
}

// RunCommandOutput executes a program without a shell and returns stdout.
// Its stdout/error behavior intentionally matches exec.Cmd.Output so callers
// that parse command output can migrate without changing product semantics.
func RunCommandOutput(ctx context.Context, program string, args ...string) ([]byte, error) {
	if err := validateCommand(ctx, program, args); err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, program, args...).Output()
}
