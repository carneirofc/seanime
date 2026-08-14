package models

// Credential redaction for settings that leave the server.
//
// The settings blob is handed to every authenticated client — on /api/v1/status,
// on /api/v1/settings, and on the "settings" websocket broadcast. It carries
// stored credentials: media-player and torrent-client passwords, a translation
// API key, the debrid API key, and the Nakama passwords. A session proves the
// caller is on the allowlist; it does not entitle them to the operator's
// credentials for other systems, and the Nakama host password in particular is
// itself an authentication credential for this server's peer endpoints.
//
// Secrets are replaced with RedactedSecret rather than emptied, so the settings
// forms can round-trip a value they were never shown: a save that hands the
// placeholder back means "unchanged", and RestoreSettingsSecrets puts the stored
// value back before anything is persisted. Sending an empty string still clears
// the credential, which is what a user emptying the field expects.

// RedactedSecret is the placeholder a stored credential is replaced with on its
// way out of the server. It is deliberately not a plausible password, so a
// credential that survives a round-trip unchanged is obvious in the database.
const RedactedSecret = "__seanime_redacted__"

// IsRedactedSecret reports whether an incoming value is the placeholder, i.e. a
// client echoing back a secret it never received.
func IsRedactedSecret(value string) bool {
	return value == RedactedSecret
}

// redactSecret returns the placeholder for a set credential, and leaves an unset
// one empty — clients need to be able to tell "no password configured" from
// "password configured but withheld".
func redactSecret(value string) string {
	if value == "" {
		return ""
	}
	return RedactedSecret
}

// restoreSecret resolves an incoming credential against the stored one.
func restoreSecret(next string, prev string) string {
	if IsRedactedSecret(next) {
		return prev
	}
	return next
}

// RedactedSettings returns a copy of the settings with every stored credential
// replaced. The receiver is left untouched: Database.GetSettings hands out a
// shared cached pointer, so redacting in place would erase the live credentials.
func RedactedSettings(s *Settings) *Settings {
	if s == nil {
		return nil
	}

	out := *s

	if s.MediaPlayer != nil {
		mediaPlayer := *s.MediaPlayer
		mediaPlayer.VlcPassword = redactSecret(mediaPlayer.VlcPassword)
		mediaPlayer.VcTranslateApiKey = redactSecret(mediaPlayer.VcTranslateApiKey)
		out.MediaPlayer = &mediaPlayer
	}

	if s.Torrent != nil {
		torrent := *s.Torrent
		torrent.QBittorrentPassword = redactSecret(torrent.QBittorrentPassword)
		torrent.TransmissionPassword = redactSecret(torrent.TransmissionPassword)
		out.Torrent = &torrent
	}

	if s.Nakama != nil {
		nakama := *s.Nakama
		nakama.HostPassword = redactSecret(nakama.HostPassword)
		nakama.RemoteServerPassword = redactSecret(nakama.RemoteServerPassword)
		out.Nakama = &nakama
	}

	return &out
}

// RestoreSettingsSecrets replaces placeholder credentials in an incoming settings
// payload with the stored values. Call it immediately after binding the request
// body, before validation and before persisting — validation that runs on the
// placeholder is validating the wrong string.
func RestoreSettingsSecrets(next *Settings, prev *Settings) {
	if next == nil {
		return
	}

	if next.MediaPlayer != nil {
		RestoreMediaPlayerSecrets(next.MediaPlayer, prev.GetMediaPlayer())
	}
	if next.Torrent != nil {
		RestoreTorrentSecrets(next.Torrent, prev.GetTorrent())
	}
	if next.Nakama != nil {
		RestoreNakamaSecrets(next.Nakama, prev.GetNakama())
	}
}

func RestoreMediaPlayerSecrets(next *MediaPlayerSettings, prev *MediaPlayerSettings) {
	if next == nil || prev == nil {
		return
	}
	next.VlcPassword = restoreSecret(next.VlcPassword, prev.VlcPassword)
	next.VcTranslateApiKey = restoreSecret(next.VcTranslateApiKey, prev.VcTranslateApiKey)
}

func RestoreTorrentSecrets(next *TorrentSettings, prev *TorrentSettings) {
	if next == nil || prev == nil {
		return
	}
	next.QBittorrentPassword = restoreSecret(next.QBittorrentPassword, prev.QBittorrentPassword)
	next.TransmissionPassword = restoreSecret(next.TransmissionPassword, prev.TransmissionPassword)
}

func RestoreNakamaSecrets(next *NakamaSettings, prev *NakamaSettings) {
	if next == nil || prev == nil {
		return
	}
	next.HostPassword = restoreSecret(next.HostPassword, prev.HostPassword)
	next.RemoteServerPassword = restoreSecret(next.RemoteServerPassword, prev.RemoteServerPassword)
}

// RedactedDebridSettings returns a copy with the provider API key withheld.
func RedactedDebridSettings(s *DebridSettings) *DebridSettings {
	if s == nil {
		return nil
	}

	out := *s
	out.ApiKey = redactSecret(out.ApiKey)
	return &out
}

// RestoreDebridSecrets replaces a placeholder API key with the stored one.
func RestoreDebridSecrets(next *DebridSettings, prev *DebridSettings) {
	if next == nil || prev == nil {
		return
	}
	next.ApiKey = restoreSecret(next.ApiKey, prev.ApiKey)
}
