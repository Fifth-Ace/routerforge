//go:build !embed_frontend

package main

import (
	"io/fs"
	"os"
)

func frontendFS() (fs.FS, error) {
	return os.DirFS("frontend/build"), nil
}
