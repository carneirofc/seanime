//go:build (windows || linux) && !nosystray

package server

import (
	"embed"
	"fmt"
	"fyne.io/systray"
	"github.com/cli/browser"
	"github.com/rs/zerolog/log"
	"seanime/internal/constants"
	"seanime/internal/core"
	"seanime/internal/handlers"
	"seanime/internal/icon"
	"seanime/internal/updater"
)

// runSystray runs the system tray. It blocks until systray.Quit() is called.
// The app loop is started from within onReady so it runs alongside the tray.
func runSystray(webFS *embed.FS, app *core.App, flags core.SeanimeFlags, selfupdater *updater.SelfUpdater) {
	onExit := func() {}

	// Blocks until systray.Quit() is called
	systray.Run(onReady(webFS, app, flags, selfupdater), onExit)
}

func addQuitItem() {
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit Seanime", "Quit the whole app")
	mQuit.Enable()
	go func() {
		<-mQuit.ClickedCh
		log.Trace().Msg("systray: Quitting system tray")
		systray.Quit()
		log.Trace().Msg("systray: Quit system tray")
	}()
}

func onReady(webFS *embed.FS, app *core.App, flags core.SeanimeFlags, selfupdater *updater.SelfUpdater) func() {
	return func() {
		buildTime := constants.BuildTime
		if buildTime == "" {
			buildTime = "unknown"
		}

		systray.SetTemplateIcon(icon.Data, icon.Data)
		systray.SetTitle(fmt.Sprintf("Seanime v%s", constants.Version))
		systray.SetTooltip(fmt.Sprintf("Seanime v%s | Build %s", constants.Version, buildTime))
		log.Trace().Msg("systray: App is ready")

		// Menu items
		mVersionInfo := systray.AddMenuItem("Seanime v"+constants.Version, "Seanime version")
		mVersionInfo.Disable()
		mBuildInfo := systray.AddMenuItem("Build: "+buildTime, "Build time")
		mBuildInfo.Disable()
		mWeb := systray.AddMenuItem(app.Config.GetServerURI("127.0.0.1"), "Open web interface")
		mOpenLibrary := systray.AddMenuItem("Open Anime Library", "Open anime library")
		mOpenDataDir := systray.AddMenuItem("Open Data Directory", "Open data directory")
		mOpenLogsDir := systray.AddMenuItem("Open Log Directory", "Open log directory")

		addQuitItem()

		go func() {
			// Close the systray when the app exits
			defer systray.Quit()

			startAppLoop(webFS, app, flags, selfupdater)
		}()

		go func() {
			for {
				select {
				case <-mWeb.ClickedCh:
					_ = browser.OpenURL(app.Config.GetServerURI("127.0.0.1"))
				case <-mOpenLibrary.ClickedCh:
					handlers.OpenDirInExplorer(app.LibraryDir)
				case <-mOpenDataDir.ClickedCh:
					handlers.OpenDirInExplorer(app.Config.Data.AppDataDir)
				case <-mOpenLogsDir.ClickedCh:
					handlers.OpenDirInExplorer(app.Config.Logs.Dir)
				}
			}
		}()
	}
}
