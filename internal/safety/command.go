package safety

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// RunCommand executes a program without a shell, captures combined stdout/stderr,
// and caps the returned output to maxOutput bytes.
func RunCommand(ctx context.Context, maxOutput int, program string, args ...string) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("command context is nil")
	}
	if strings.TrimSpace(program) == "" {
		return nil, errors.New("command program is empty")
	}
	if strings.IndexByte(program, 0) >= 0 {
		return nil, errors.New("command program contains NUL")
	}
	for _, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return nil, errors.New("command argument contains NUL")
		}
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
