//go:build !embed_frontend

package main

import (
	"io/fs"
	"os"
)

func frontendFS() (fs.FS, error) {
	for _, root := range []string{
		"components/core/frontend/build",
		"frontend/build",
	} {
		if _, err := os.Stat(root); err == nil {
			return os.DirFS(root), nil
		}
	}
	return os.DirFS("components/core/frontend/build"), nil
}
