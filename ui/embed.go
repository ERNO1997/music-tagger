// Package ui embeds the static frontend assets so the compiled server
// binary can serve them without any external files at runtime.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Assets is dist/'s contents rooted at "/", stripping the embed's "dist"
// prefix so http.FS(Assets) serves e.g. /assets/index-XXXX.js rather than
// /dist/assets/index-XXXX.js.
var Assets fs.FS

func init() {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	Assets = sub
}
