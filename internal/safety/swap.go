package safety

import (
	"errors"
	"fmt"
	"os"
)

// SwapResult describes whether an old destination existed before the swap.
type SwapResult struct {
	HadCurrent bool
}

// SwapPath replaces destination with staged while keeping the previous destination
// available for automatic rollback if publishing staged fails.
func SwapPath(staged, destination, rollback string) (SwapResult, error) {
	var result SwapResult

	if staged == "" || destination == "" || rollback == "" {
		return result, errors.New("swap paths must not be empty")
	}
	if staged == destination || staged == rollback || destination == rollback {
		return result, errors.New("swap paths must be distinct")
	}

	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, rollback); err != nil {
			return result, fmt.Errorf("stage current destination for rollback: %w", err)
		}
		result.HadCurrent = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("inspect current destination: %w", err)
	}

	if err := os.Rename(staged, destination); err != nil {
		if result.HadCurrent {
			if rollbackErr := os.Rename(rollback, destination); rollbackErr != nil {
				return result, fmt.Errorf("publish staged path: %v; rollback failed: %w", err, rollbackErr)
			}
		}
		return result, fmt.Errorf("publish staged path: %w", err)
	}

	return result, nil
}

// CleanupRollback removes a rollback path after a successful swap.
func CleanupRollback(path string, hadCurrent bool) error {
	if !hadCurrent {
		return nil
	}
	if path == "" {
		return errors.New("rollback path must not be empty")
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove rollback path: %w", err)
	}
	return nil
}
