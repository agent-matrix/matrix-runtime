// Package web embeds the Matrix Cloud enterprise console static assets so the
// runtime can serve the UI from the same binary (no external CDN at runtime).
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var staticFS embed.FS

// Static returns the console's static asset filesystem rooted at the asset dir.
func Static() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
