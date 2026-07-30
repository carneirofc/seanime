//go:build windows && !nosystray

package server

import (
	"embed"

	"github.com/gonutz/w32/v2"
)

func StartServer(webFS embed.FS, embeddedLogo []byte) {
	hideConsole()

	app, flags, selfupdater := startApp(embeddedLogo)

	// Blocks until systray.Quit() is called
	runSystray(&webFS, app, flags, selfupdater)
}

// hideConsole will hide the terminal window if the app was not started with the -H=windowsgui flag.
// NOTE: This will only minimize the terminal window on Windows 11 if the 'default terminal app' is set to 'Windows Terminal' or 'Let Windows choose' instead of 'Windows Console Host'
func hideConsole() {
	console := w32.GetConsoleWindow()
	if console == 0 {
		return // no console attached
	}
	// If this application is the process that created the console window, then
	// this program was not compiled with the -H=windowsgui flag and on start-up
	// it created a console along with the main application window. In this case
	// hide the console window.
	// See
	// http://stackoverflow.com/questions/9009333/how-to-check-if-the-program-is-run-from-a-console
	_, consoleProcID := w32.GetWindowThreadProcessId(console)
	if w32.GetCurrentProcessId() == consoleProcID {
		w32.ShowWindow(console, w32.SW_HIDE)
	}
}
