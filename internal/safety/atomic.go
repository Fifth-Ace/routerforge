package safety

import (
	"errors"
	"os"
	"path/filepath"
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

// WriteFileAtomic writes a complete file through a same-directory temporary file,
// flushes it, closes it, and atomically publishes it with rename.
func WriteFileAtomic(destination string, data []byte, mode os.FileMode) error {
	if destination == "" {
		return errors.New("atomic destination is empty")
	}

	dir := filepath.Dir(destination)
	pattern := "." + filepath.Base(destination) + ".tmp-*"
	atomicFile, err := NewAtomicFile(dir, pattern)
	if err != nil {
		return err
	}
	defer atomicFile.Cleanup()

	temp := atomicFile.File()
	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := atomicFile.Sync(); err != nil {
		return err
	}
	if err := atomicFile.Close(); err != nil {
		return err
	}
	return atomicFile.Publish(destination)
}

// CreateExclusiveFile creates a new file and fails if the path already exists.
func CreateExclusiveFile(path string, mode os.FileMode) (*os.File, error) {
	if path == "" {
		return nil, errors.New("exclusive file path is empty")
	}
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
}
