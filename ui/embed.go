// Package ui embeds the static frontend assets so the compiled server
// binary can serve them without any external files at runtime.
package ui

import "embed"

//go:embed index.html css js
var Assets embed.FS
