// Package web bettet den Frontend-Build (frontend/ → web/dist) in die exe ein.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS liefert die Frontend-Dateien (index.html im Wurzelverzeichnis).
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
