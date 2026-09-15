package safety

import (
	"errors"
	"os"
)

// AtomicFile owns a temporary file until it is published with rename.
type AtomicFile struct {
	file   *os.File
	path   string
	closed bool
}

// NewAtomicFile creates a temporary file in the destination directory.
func NewAtomicFile(dir, pattern string) (*AtomicFile, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return &AtomicFile{
		file: file,
		path: file.Name(),
	}, nil
}

// File returns the temporary file for content and metadata operations.
func (a *AtomicFile) File() *os.File {
	return a.file
}

// Path returns the temporary path.
func (a *AtomicFile) Path() string {
	return a.path
}

// Sync flushes the temporary file before publish.
func (a *AtomicFile) Sync() error {
	if a == nil || a.file == nil || a.closed {
		return errors.New("atomic file is closed")
	}
	return a.file.Sync()
}

// Close closes the temporary file. Repeated Close calls are harmless.
func (a *AtomicFile) Close() error {
	if a == nil || a.file == nil {
		return errors.New("atomic file is unavailable")
	}
	if a.closed {
		return nil
	}
	err := a.file.Close()
	a.closed = true
	return err
}

// Publish atomically renames the closed temporary file to destination.
func (a *AtomicFile) Publish(destination string) error {
	if a == nil || a.file == nil {
		return errors.New("atomic file is unavailable")
	}
	if !a.closed {
		return errors.New("atomic file must be closed before publish")
	}
	return os.Rename(a.path, destination)
}

// Cleanup closes and removes an unpublished temporary file.
// After a successful Publish the temporary path no longer exists, so Cleanup is a no-op.
func (a *AtomicFile) Cleanup() error {
	if a == nil {
		return nil
	}
	if a.file != nil && !a.closed {
		_ = a.file.Close()
		a.closed = true
	}
	if a.path == "" {
		return nil
	}
	err := os.Remove(a.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
