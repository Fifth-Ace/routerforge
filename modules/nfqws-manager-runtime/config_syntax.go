package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// Choose a fixed, trusted shell that actually supports syntax-only validation.
// KeeneticOS /bin/sh may reject -n; Entware provides a compatible BusyBox sh.
// The probe parses only a constant and never evaluates user content.
func nf1ShellForSyntax(ctx context.Context, candidates []string) (string, error) {
	for _, path := range candidates {
		cmd, err := safety.CommandContext(ctx, path, "-n")
		if err != nil {
			continue
		}
		cmd.Stdin = strings.NewReader(":\n")
		if err := cmd.Run(); err == nil {
			return path, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("нет доступной shell-оболочки с поддержкой -n")
}

// checkNFQWSShellSyntax parses the user-supplied nfqws2-keenetic shell config
// without evaluating assignments, substitutions, or any commands inside it.
// This is a shell grammar check, not a substitute for nfqws2 --dry-run.
func checkNFQWSShellSyntax(content string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	path, err := nf1ShellForSyntax(ctx, []string{"/opt/usr/bin/sh", "/opt/bin/sh", "/bin/sh"})
	if err != nil {
		return fmt.Errorf("не удалось подготовить проверку синтаксиса shell: %w", err)
	}
	cmd, err := safety.CommandContext(ctx, path, "-n")
	if err != nil {
		return fmt.Errorf("не удалось подготовить проверку синтаксиса shell: %w", err)
	}
	cmd.Stdin = strings.NewReader(content)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("проверка синтаксиса shell прервана: %w", ctx.Err())
	}
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if len(message) > 2048 {
		message = message[:2048]
	}
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("ошибка синтаксиса shell: %s", message)
}
