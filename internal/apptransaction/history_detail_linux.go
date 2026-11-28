//go:build linux

package apptransaction

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// A historical read never starts, replays or edits a package operation.
type UpdateHistoryDetail struct {
	Slot         SlotStatus `json:"slot"`
	Log          string     `json:"log"`
	LogAvailable bool       `json:"log_available"`
	LogTruncated bool       `json:"log_truncated"`
}

func terminalHistoryState(state string) bool {
	switch state {
	case string(Completed), string(Failed), string(Cancelled), string(RecoveryRequired):
		return true
	}
	return false
}

func historyLog(dir string) (string, bool, bool) {
	const limit = 1 << 20
	fd, err := syscall.Open(filepath.Join(dir, "worker.log"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", false, false
	}
	f := os.NewFile(uintptr(fd), "worker.log")
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) {
		return "", false, false
	}
	truncated := st.Size() > limit
	if truncated {
		if _, err = f.Seek(-limit, io.SeekEnd); err != nil {
			return "", false, false
		}
	}
	b, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return "", false, false
	}
	return string(b), true, truncated
}

// IDs are validated before path construction. Only known transaction archives
// or the matching terminal active slot may be read; no client paths are accepted.
func (c *Coordinator) HistoryDetail(id string) (UpdateHistoryDetail, error) {
	if !token.MatchString(id) {
		return UpdateHistoryDetail{}, errors.New("invalid transaction identity")
	}
	if _, err := Open(c.root); err != nil {
		return UpdateHistoryDetail{}, err
	}
	for _, name := range []string{"completed-" + id, "failed-" + id, activeSlot} {
		dir := filepath.Join(c.root, name)
		var v SlotStatus
		var err error
		if name == activeSlot {
			v, err = c.Status()
		} else {
			v, err = inspectTransactionSlot(dir)
		}
		if err != nil || v.Plan == nil || v.Plan.ID != id || v.Busy {
			continue
		}
		if name == activeSlot && !terminalHistoryState(v.State) {
			continue
		}
		if name == "completed-"+id && v.State != string(Completed) {
			continue
		}
		log, available, truncated := historyLog(dir)
		// The active directory may have been renamed while reading. Never attach a
		// different transaction's log to this ID.
		after, err := inspectTransactionSlot(dir)
		if err != nil || after.Plan == nil || after.Plan.ID != id {
			continue
		}
		return UpdateHistoryDetail{Slot: v, Log: log, LogAvailable: available, LogTruncated: truncated}, nil
	}
	return UpdateHistoryDetail{}, os.ErrNotExist
}
