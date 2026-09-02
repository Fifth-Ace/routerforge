//go:build embed_frontend

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/build
var frontendBuild embed.FS

func frontendFS() (fs.FS, error) {
	return fs.Sub(frontendBuild, "frontend/build")
}
