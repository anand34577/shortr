// Package webdist embeds the built React frontend (web/dist) into the
// shortr binary. It must live at the module root because Go's go:embed
// patterns cannot contain ".." path elements, and web/dist sits at
// <root>/web/dist — a file under cmd/shortr couldn't reach it directly.
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var distFS embed.FS

// FS returns the embedded frontend build rooted at web/dist (i.e. FS()
// contains index.html directly, not web/dist/index.html).
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "web/dist")
}
