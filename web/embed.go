// Package web holds the built page. Run `make build` to fill dist/.
package web

import "embed"

// Dist is the built page from dist/.
//
//go:embed all:dist
var Dist embed.FS
