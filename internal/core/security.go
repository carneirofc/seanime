package core

import (
	"seanime/internal/security"
	"strings"

	"github.com/spf13/viper"
)

func (a *App) SyncSecurityConfig() {
	if a == nil || a.Config == nil {
		return
	}

	security.SetRequestBoundaryConfig(a.Config.Server.TrustedProxies, a.Config.Server.ExternalURL)
	security.SetUntrustedExecutableRoots(a.writableRoots())
	a.syncCapabilities()
}

// writableRoots lists the directories Seanime writes to. Nothing under them may be
// spawned as an executable — see internal/security/executable.go.
func (a *App) writableRoots() []string {
	cfg := a.Config

	roots := []string{
		cfg.Data.AppDataDir,
		cfg.Cache.Dir,
		cfg.Cache.TranscodeDir,
		cfg.Logs.Dir,
		cfg.Offline.Dir,
		cfg.Offline.AssetDir,
		cfg.Manga.DownloadDir,
		cfg.Manga.LocalDir,
		cfg.Extensions.Dir,
		cfg.Torrent.Dir,
		cfg.Web.AssetDir,
	}

	if a.Settings != nil {
		roots = append(roots, a.Settings.GetLibrary().GetLibraryPaths()...)
		roots = append(roots, a.Settings.GetManga().LocalSourceDirectory)
	}

	if a.SecondarySettings.Torrentstream != nil {
		roots = append(roots, a.SecondarySettings.Torrentstream.DownloadDir)
	}
	if a.SecondarySettings.Mediastream != nil {
		roots = append(roots, a.SecondarySettings.Mediastream.PreTranscodeLibraryDir)
	}

	return roots
}

// syncCapabilities resolves the privileged capability set from configuration.
//
// Capabilities are never inferred from a request — see internal/security/capability.go.
// The only decision made here is what to do when the operator did not configure them
// at all, and that is a deployment-wide fallback, not a per-caller one.
func (a *App) syncCapabilities() {
	configured := viper.IsSet("server.capabilities")

	capabilities, unknown := security.ParseCapabilities(a.Config.Server.Capabilities)
	for _, entry := range unknown {
		a.Logger.Warn().Str("capability", entry).Msg("app: Unknown server capability, ignoring")
	}

	if !configured {
		capabilities = security.ResolveDefaultCapabilities(
			a.Config.IsOidcMode(),
			a.Config.Server.ExternalURL,
			a.Config.Server.TrustedProxies,
		)
	}

	security.SetCapabilities(capabilities, configured)

	event := a.Logger.Info()
	if len(capabilities) == 0 {
		// Worth surfacing loudly: this is the posture where privileged routes 403
		// for everyone, including the operator's own browser.
		event = a.Logger.Warn()
	}
	event.
		Strs("capabilities", capabilities).
		Bool("configured", configured).
		Msg("app: Privileged capabilities resolved")
}

func (a *App) SetSecureMode(mode string, updateConfig bool) {
	requestedMode := strings.TrimSpace(strings.ToLower(mode))
	normalizedMode := security.NormalizeMode(requestedMode)
	if requestedMode != "" && normalizedMode == security.SecureModeDefault {
		a.Logger.Warn().Str("mode", mode).Msg("app: Invalid secure mode, defaulting to baseline mode")
	}

	security.SetSecureMode(normalizedMode)
	if updateConfig { // unused for now
		a.Config.Server.SecureMode = normalizedMode
		viper.Set("server.secureMode", normalizedMode)
		err := viper.WriteConfig()
		if err != nil {
			a.Logger.Err(err).Msg("app: Failed to write config after setting secure mode")
		}
	}
	logMode := normalizedMode
	if logMode == "" {
		logMode = "default"
	}
	a.Logger.Info().Str("mode", logMode).Msg("app: Secure mode changed")
}
