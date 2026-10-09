//go:build !noembedweb

package main

import "embed"

// WebFS holds the built web UI (seanime-web/out, copied into web/ by `npm run build:embed`).
//
//go:embed all:web
var WebFS embed.FS
