package models

import "testing"

func fullySetSettings() *Settings {
	return &Settings{
		MediaPlayer: &MediaPlayerSettings{
			Default:           "mpv",
			VlcUsername:       "user",
			VlcPassword:       "vlc-secret",
			VcTranslateApiKey: "translate-key",
		},
		Torrent: &TorrentSettings{
			Default:              "qbittorrent",
			QBittorrentUsername:  "user",
			QBittorrentPassword:  "qbit-secret",
			TransmissionPassword: "transmission-secret",
		},
		Nakama: &NakamaSettings{
			Enabled:              true,
			IsHost:               true,
			Username:             "peer",
			HostPassword:         "host-secret",
			RemoteServerURL:      "https://peer.example.com",
			RemoteServerPassword: "remote-secret",
		},
	}
}

func TestRedactedSettingsWithholdsEveryCredential(t *testing.T) {
	settings := fullySetSettings()
	redacted := RedactedSettings(settings)

	secrets := map[string]string{
		"vlcPassword":          redacted.MediaPlayer.VlcPassword,
		"vcTranslateApiKey":    redacted.MediaPlayer.VcTranslateApiKey,
		"qbittorrentPassword":  redacted.Torrent.QBittorrentPassword,
		"transmissionPassword": redacted.Torrent.TransmissionPassword,
		"hostPassword":         redacted.Nakama.HostPassword,
		"remoteServerPassword": redacted.Nakama.RemoteServerPassword,
	}

	for field, value := range secrets {
		if value != RedactedSecret {
			t.Errorf("%s = %q, want the redaction placeholder", field, value)
		}
	}

	// Non-credential fields must survive, or the settings UI has nothing to show.
	if redacted.MediaPlayer.VlcUsername != "user" {
		t.Errorf("vlcUsername = %q, want it left alone", redacted.MediaPlayer.VlcUsername)
	}
	if redacted.Nakama.RemoteServerURL != "https://peer.example.com" {
		t.Errorf("remoteServerURL = %q, want it left alone", redacted.Nakama.RemoteServerURL)
	}
}

func TestRedactedSettingsDoesNotMutateTheSource(t *testing.T) {
	// Database.GetSettings hands out a shared cached pointer, so redacting in place
	// would erase the credentials the server itself needs.
	settings := fullySetSettings()
	_ = RedactedSettings(settings)

	if settings.MediaPlayer.VlcPassword != "vlc-secret" {
		t.Errorf("source vlcPassword = %q, want it untouched", settings.MediaPlayer.VlcPassword)
	}
	if settings.Nakama.HostPassword != "host-secret" {
		t.Errorf("source hostPassword = %q, want it untouched", settings.Nakama.HostPassword)
	}
	if settings.Torrent.QBittorrentPassword != "qbit-secret" {
		t.Errorf("source qbittorrentPassword = %q, want it untouched", settings.Torrent.QBittorrentPassword)
	}
}

func TestRedactedSettingsKeepsUnsetCredentialsEmpty(t *testing.T) {
	// A client has to be able to tell "no password configured" from
	// "password configured but withheld".
	settings := &Settings{MediaPlayer: &MediaPlayerSettings{}, Torrent: &TorrentSettings{}, Nakama: &NakamaSettings{}}
	redacted := RedactedSettings(settings)

	if redacted.MediaPlayer.VlcPassword != "" {
		t.Errorf("unset vlcPassword = %q, want empty", redacted.MediaPlayer.VlcPassword)
	}
	if redacted.Nakama.HostPassword != "" {
		t.Errorf("unset hostPassword = %q, want empty", redacted.Nakama.HostPassword)
	}
}

func TestRedactedSettingsHandlesNilSections(t *testing.T) {
	if RedactedSettings(nil) != nil {
		t.Fatal("RedactedSettings(nil) should be nil")
	}

	redacted := RedactedSettings(&Settings{})
	if redacted.MediaPlayer != nil || redacted.Torrent != nil || redacted.Nakama != nil {
		t.Fatal("absent sections should stay absent")
	}
}

func TestSettingsRoundTripPreservesCredentials(t *testing.T) {
	// The path a settings form actually takes: the client is shown placeholders and
	// hands them straight back. Nothing may be lost.
	stored := fullySetSettings()
	returned := RedactedSettings(stored)

	RestoreSettingsSecrets(returned, stored)

	if returned.MediaPlayer.VlcPassword != "vlc-secret" {
		t.Errorf("vlcPassword = %q, want the stored value restored", returned.MediaPlayer.VlcPassword)
	}
	if returned.Torrent.TransmissionPassword != "transmission-secret" {
		t.Errorf("transmissionPassword = %q, want the stored value restored", returned.Torrent.TransmissionPassword)
	}
	if returned.Nakama.HostPassword != "host-secret" {
		t.Errorf("hostPassword = %q, want the stored value restored", returned.Nakama.HostPassword)
	}
}

func TestRestoreSettingsSecretsHonoursRealChanges(t *testing.T) {
	stored := fullySetSettings()
	incoming := RedactedSettings(stored)

	// A user typing a new password, and a user clearing the field.
	incoming.MediaPlayer.VlcPassword = "brand-new"
	incoming.Nakama.HostPassword = ""

	RestoreSettingsSecrets(incoming, stored)

	if incoming.MediaPlayer.VlcPassword != "brand-new" {
		t.Errorf("vlcPassword = %q, want the new value to win", incoming.MediaPlayer.VlcPassword)
	}
	if incoming.Nakama.HostPassword != "" {
		t.Errorf("hostPassword = %q, want an emptied field to clear the credential", incoming.Nakama.HostPassword)
	}
	// Untouched fields still round-trip.
	if incoming.Torrent.QBittorrentPassword != "qbit-secret" {
		t.Errorf("qbittorrentPassword = %q, want the stored value restored", incoming.Torrent.QBittorrentPassword)
	}
}

func TestDebridSettingsRedactionRoundTrip(t *testing.T) {
	stored := &DebridSettings{Enabled: true, Provider: "torbox", ApiKey: "debrid-key"}

	redacted := RedactedDebridSettings(stored)
	if redacted.ApiKey != RedactedSecret {
		t.Fatalf("apiKey = %q, want the redaction placeholder", redacted.ApiKey)
	}
	if stored.ApiKey != "debrid-key" {
		t.Fatalf("source apiKey = %q, want it untouched", stored.ApiKey)
	}
	if redacted.Provider != "torbox" {
		t.Fatalf("provider = %q, want it left alone", redacted.Provider)
	}

	RestoreDebridSecrets(redacted, stored)
	if redacted.ApiKey != "debrid-key" {
		t.Fatalf("apiKey = %q, want the stored key restored", redacted.ApiKey)
	}
}

func TestRedactedSecretIsNotAPlausibleCredential(t *testing.T) {
	// If this ever collides with a real password, that password becomes
	// unchangeable — the save path would read it as "leave as is".
	if !IsRedactedSecret(RedactedSecret) {
		t.Fatal("IsRedactedSecret must recognise its own placeholder")
	}
	if IsRedactedSecret("") || IsRedactedSecret("hunter2") {
		t.Fatal("IsRedactedSecret matched a real value")
	}
}
