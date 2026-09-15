package safety

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCommandRejectsInvalidInputs(t *testing.T) {
	ctx := context.Background()

	if _, err := RunCommand(nil, 16, "x"); err == nil {
		t.Fatal("nil context must be rejected")
	}
	if _, err := RunCommand(ctx, 16, ""); err == nil {
		t.Fatal("empty program must be rejected")
	}
	if _, err := RunCommand(ctx, 16, "bad\x00program"); err == nil {
		t.Fatal("NUL program must be rejected")
	}
	if _, err := RunCommand(ctx, -1, "x"); err == nil {
		t.Fatal("negative output limit must be rejected")
	}
	if _, err := RunCommand(ctx, 16, "x", "bad\x00arg"); err == nil {
		t.Fatal("NUL argument must be rejected")
	}
}

func TestRunCommandCapturesAndCapsOutput(t *testing.T) {
	if os.Getenv("ROUTERFORGE_COMMAND_HELPER") == "1" {
		_, _ = os.Stdout.WriteString("abcdefghij")
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	old := os.Getenv("ROUTERFORGE_COMMAND_HELPER")
	if err := os.Setenv("ROUTERFORGE_COMMAND_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if old == "" {
			_ = os.Unsetenv("ROUTERFORGE_COMMAND_HELPER")
		} else {
			_ = os.Setenv("ROUTERFORGE_COMMAND_HELPER", old)
		}
	}()

	output, err := RunCommand(
		ctx,
		4,
		os.Args[0],
		"-test.run=TestRunCommandCapturesAndCapsOutput",
	)
	if err != nil {
		t.Fatalf("run helper: %v", err)
	}
	if string(output) != "abcd" {
		t.Fatalf("output=%q want=%q", string(output), "abcd")
	}
}

func TestRunCommandTimeout(t *testing.T) {
	if os.Getenv("ROUTERFORGE_COMMAND_SLEEP_HELPER") == "1" {
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cmd := os.Args[0]
	args := []string{"-test.run=TestRunCommandTimeout"}

	old := os.Getenv("ROUTERFORGE_COMMAND_SLEEP_HELPER")
	if err := os.Setenv("ROUTERFORGE_COMMAND_SLEEP_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if old == "" {
			_ = os.Unsetenv("ROUTERFORGE_COMMAND_SLEEP_HELPER")
		} else {
			_ = os.Setenv("ROUTERFORGE_COMMAND_SLEEP_HELPER", old)
		}
	}()

	_, err := RunCommand(ctx, 64, cmd, args...)
	if err == nil {
		t.Fatal("timed out command must fail")
	}
	if !strings.Contains(ctx.Err().Error(), "deadline exceeded") {
		t.Fatalf("context error=%v", ctx.Err())
	}
}
