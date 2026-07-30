//go:build linux && !nosystray

package server

import (
	"embed"
)

func StartServer(webFS embed.FS, embeddedLogo []byte) {
	app, flags, selfupdater := startApp(embeddedLogo)

	// Blocks until systray.Quit() is called
	runSystray(&webFS, app, flags, selfupdater)
}
