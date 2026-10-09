//go:build noembedweb

package main

import "embed"

// WebFS is empty in API-only builds: the web UI is served by a separate web server
// on the same origin (see WEB_DEPLOYMENT.md, "Serving the UI separately").
var WebFS embed.FS
