// Package web holds the embedded UI assets (templates and static files).
package web

import "embed"

//go:embed templates static
var FS embed.FS
