package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// checkNFQWSShellSyntax parses the user-supplied nfqws2-keenetic shell config
// without evaluating assignments, substitutions, or any commands inside it.
// This is a shell grammar check, not a substitute for nfqws2 --dry-run.
func checkNFQWSShellSyntax(content string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd, err := safety.CommandContext(ctx, "/bin/sh", "-n")
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
