package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

var foundationObserverPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
var errFoundationObserver = errors.New("observer identity unavailable")

// Load/create an installation identity. Never overwrite a corrupt existing
// identity. Publish a fully written file with an exclusive hard link so two
// processes cannot generate different accepted IDs. No write on API GET.
func loadFoundationObserver(path string) (string, error) {
	dir := filepath.Dir(path)
	di, e := os.Lstat(dir)
	if e != nil || !di.IsDir() || di.Mode()&os.ModeSymlink != 0 {
		return "", errFoundationObserver
	}
	read := func() (string, error) {
		info, e := os.Lstat(path)
		if e != nil {
			return "", e
		}
		if !info.Mode().IsRegular() || info.Size() != 37 {
			return "", errFoundationObserver
		}
		f, e := os.Open(path)
		if e != nil {
			return "", e
		}
		defer f.Close()
		fi, e := f.Stat()
		if e != nil || !os.SameFile(info, fi) {
			return "", errFoundationObserver
		}
		b, e := io.ReadAll(io.LimitReader(f, 38))
		if e != nil || len(b) != 37 || b[36] != '\n' {
			return "", errFoundationObserver
		}
		id := strings.TrimSuffix(string(b), "\n")
		if !foundationObserverPattern.MatchString(id) {
			return "", errFoundationObserver
		}
		return id, nil
	}
	id, e := read()
	if e == nil {
		return id, nil
	}
	if !os.IsNotExist(e) {
		return "", errFoundationObserver
	}
	id, e = platformevents.NewEventID()
	if e != nil {
		return "", errFoundationObserver
	}
	f, e := os.CreateTemp(dir, ".rf-observer-")
	if e != nil {
		return "", errFoundationObserver
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.WriteString(id + "\n"); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return "", errFoundationObserver
	}
	if e = os.Link(tmp, path); e != nil {
		if os.IsExist(e) {
			return read()
		}
		return "", errFoundationObserver
	}
	// Best-effort directory sync; supported on the target Linux filesystem.
	if d, e := os.Open(dir); e == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return read()
}
