//go:build linux

package apptransaction

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CompletedTransaction is a bounded summary, not an executable plan.
type CompletedTransaction struct {
	ID           string `json:"id"`
	Commit       string `json:"commit"`
	PackageCount int    `json:"package_count"`
	State        string `json:"state,omitempty"`
}

// CompletedHistory reads only immutable completed archives. It never creates,
// repairs, removes or replays a transaction. The active slot is reported separately.
func (c *Coordinator) CompletedHistory(limit int) ([]CompletedTransaction, error) {
	return c.updateHistory(limit, false)
}
func (c *Coordinator) UpdateHistory(limit int) ([]CompletedTransaction, error) {
	return c.updateHistory(limit, true)
}
func (c *Coordinator) updateHistory(limit int, includeFailed bool) ([]CompletedTransaction, error) {
	if limit < 1 || limit > 50 {
		return nil, errors.New("invalid history limit")
	}
	if _, err := Open(c.root); err != nil {
		return nil, err
	}
	f, err := os.Open(c.root)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(4097)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > 4096 {
		return nil, errors.New("history scan limit exceeded")
	}
	type archive struct {
		name     string
		modified time.Time
	}
	var archives []archive
	for _, entry := range entries {
		prefix := "completed-"
		if includeFailed && strings.HasPrefix(entry.Name(), "failed-") {
			prefix = "failed-"
		}
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		id := strings.TrimPrefix(entry.Name(), prefix)
		if !token.MatchString(id) || !entry.IsDir() {
			return nil, errors.New("invalid history archive")
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		archives = append(archives, archive{entry.Name(), info.ModTime()})
	}
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].modified.Equal(archives[j].modified) {
			return archives[i].name < archives[j].name
		}
		return archives[i].modified.After(archives[j].modified)
	})
	if len(archives) > limit {
		archives = archives[:limit]
	}
	out := make([]CompletedTransaction, 0, len(archives))
	for _, archive := range archives {
		v, err := inspectTransactionSlot(filepath.Join(c.root, archive.name))
		completed := strings.HasPrefix(archive.name, "completed-")
		prefix := "completed-"
		if !completed {
			prefix = "failed-"
		}
		if err != nil || v.Plan == nil || v.Plan.ID != strings.TrimPrefix(archive.name, prefix) || (completed && v.State != string(Completed)) {
			if includeFailed {
				continue
			}
			return nil, errors.New("invalid completed archive")
		}
		state := ""
		if includeFailed {
			state = "failed"
			if completed {
				state = "completed"
			}
		}
		out = append(out, CompletedTransaction{ID: v.Plan.ID, Commit: v.Plan.Commit, PackageCount: len(v.Plan.Packages), State: state})
	}
	return out, nil
}
