package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"seanime/internal/core"
	"seanime/internal/database/models"
	"seanime/internal/security"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRequestHasTrustedLocalOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		referer string
		reqHost string
		want    bool
	}{
		{
			name:    "allows loopback origin",
			origin:  "http://127.0.0.1:43211",
			reqHost: "example.com",
			want:    true,
		},
		{
			name:    "allows localhost dev origin against loopback api",
			origin:  "http://localhost:43210",
			reqHost: "127.0.0.1:43001",
			want:    true,
		},
		{
			name:    "falls back to referer",
			referer: "http://[::1]:43211/settings",
			reqHost: "example.com",
			want:    true,
		},
		{
			name:    "allows same server lan origin",
			origin:  "http://192.168.1.10:43211",
			reqHost: "192.168.1.10:43211",
			want:    true,
		},
		{
			name:    "rejects arbitrary website origins",
			origin:  "https://evil.example",
			reqHost: "192.168.1.10:43211",
			want:    false,
		},
		{
			name:    "rejects different lan origins",
			origin:  "http://192.168.1.10:43211",
			reqHost: "192.168.1.20:43211",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// local browser requests should still be able to change these settings.
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", nil)
			req.Host = tt.reqHost
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}

			assert.Equal(t, tt.want, isRequestFromTrustedOrigin(req))
		})
	}
}

func TestRequestHasTrustedLocalHost(t *testing.T) {
	tests := []struct {
		name    string
		reqHost string
		want    bool
	}{
		{
			name:    "allows localhost host",
			reqHost: "localhost:43211",
			want:    true,
		},
		{
			name:    "allows loopback host",
			reqHost: "127.0.0.1:43211",
			want:    true,
		},
		{
			name:    "allows ipv6 loopback host",
			reqHost: "[::1]:43211",
			want:    true,
		},
		{
			name:    "allows private lan host",
			reqHost: "192.168.1.10:43211",
			want:    true,
		},
		{
			name:    "rejects arbitrary domain host",
			reqHost: "evil.example",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			req.Host = tt.reqHost

			assert.Equal(t, tt.want, isTrustedRequestHost(req))
		})
	}
}

func TestCanAccessLocalServer(t *testing.T) {
	tests := []struct {
		name            string
		origin          string
		reqHost         string
		remoteAddr      string
		hasServerAuth   bool
		accessAllowlist []string
		secureMode      string
		want            bool
	}{
		{
			name:       "allows passwordless local host without browser metadata",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "127.0.0.1:51111",
			want:       true,
		},
		{
			name:       "allows passwordless trusted local origin",
			origin:     "http://127.0.0.1:43211",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "127.0.0.1:51111",
			want:       true,
		},
		{
			name:       "rejects passwordless spoofed local host without local client boundary",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "203.0.113.10:51111",
			want:       true,
		},
		{
			name:       "rejects passwordless spoofed trusted local origin without local client boundary",
			origin:     "http://127.0.0.1:43211",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "203.0.113.10:51111",
			want:       true,
		},
		{
			name:       "rejects passwordless spoofed local host in hardened mode",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "203.0.113.10:51111",
			secureMode: security.SecureModeHardened,
			want:       false,
		},
		{
			name:       "rejects passwordless spoofed trusted local origin in hardened mode",
			origin:     "http://127.0.0.1:43211",
			reqHost:    "127.0.0.1:43211",
			remoteAddr: "203.0.113.10:51111",
			secureMode: security.SecureModeHardened,
			want:       false,
		},
		{
			name:    "rejects passwordless untrusted origin even on local host",
			origin:  "https://evil.example",
			reqHost: "127.0.0.1:43211",
			want:    false,
		},
		{
			name:       "allows passwordless same-server lan host by default",
			reqHost:    "192.168.1.10:43211",
			remoteAddr: "192.168.1.10:51111",
			want:       true,
		},
		{
			name:       "rejects passwordless same-server lan host in hardened mode",
			reqHost:    "192.168.1.10:43211",
			remoteAddr: "192.168.1.10:51111",
			secureMode: security.SecureModeHardened,
			want:       false,
		},
		{
			name:       "rejects passwordless same-server lan host in strict mode",
			reqHost:    "192.168.1.10:43211",
			remoteAddr: "192.168.1.10:51111",
			secureMode: security.SecureModeStrict,
			want:       false,
		},
		{
			name:    "rejects passwordless arbitrary domain host",
			reqHost: "evil.example",
			want:    false,
		},
		{
			name:    "rejects passwordless cross-site browser requests without origin metadata",
			reqHost: "127.0.0.1:43211",
			origin:  "",
			want:    false,
		},
		{
			name:          "allows authenticated requests regardless of host",
			reqHost:       "evil.example",
			hasServerAuth: true,
			want:          true,
		},
		{
			name:          "allows authenticated requests regardless of host in strict mode",
			reqHost:       "evil.example",
			hasServerAuth: true,
			secureMode:    security.SecureModeStrict,
			want:          true,
		},
		{
			name:            "allows passwordless public host when allowlisted",
			reqHost:         "demo.example",
			accessAllowlist: []string{"demo.example"},
			want:            true,
		},
		{
			name:            "allows passwordless public origin when allowlisted",
			origin:          "https://demo.example",
			reqHost:         "demo.example",
			accessAllowlist: []string{"https://demo.example"},
			want:            true,
		},
		{
			name:            "allows passwordless public subdomain when wildcard allowlisted",
			origin:          "https://live.demo.example",
			reqHost:         "live.demo.example",
			accessAllowlist: []string{"*.demo.example"},
			want:            true,
		},
		{
			name:       "allows arbitrary public host in lax mode",
			reqHost:    "demo.example",
			secureMode: security.SecureModeLax,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			security.SetSecureMode(tt.secureMode)
			t.Cleanup(func() {
				security.SetSecureMode("")
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
			req.Host = tt.reqHost
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			} else if tt.name == "rejects passwordless cross-site browser requests without origin metadata" {
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}

			assert.Equal(t, tt.want, isRequestPermitted(req, tt.hasServerAuth, tt.accessAllowlist))
		})
	}
}

func TestTrustedCORSOrigin(t *testing.T) {
	tests := []struct {
		name            string
		origin          string
		hasServerAuth   bool
		accessAllowlist []string
		secureMode      string
		want            bool
	}{
		{
			name:   "allows trusted local origin",
			origin: "http://127.0.0.1:43211",
			want:   true,
		},
		{
			name:   "allows private lan origin by default",
			origin: "http://192.168.1.10:43211",
			want:   true,
		},
		{
			name:       "rejects private lan origin in hardened mode",
			origin:     "http://192.168.1.10:43211",
			secureMode: security.SecureModeHardened,
			want:       false,
		},
		{
			name:       "rejects private lan origin in strict mode",
			origin:     "http://192.168.1.10:43211",
			secureMode: security.SecureModeStrict,
			want:       false,
		},
		{
			name:            "allows allowlisted public origin",
			origin:          "https://demo.example",
			accessAllowlist: []string{"https://demo.example"},
			want:            true,
		},
		{
			name:            "allows wildcard allowlisted public origin",
			origin:          "https://live.demo.example",
			accessAllowlist: []string{"*.demo.example"},
			want:            true,
		},
		{
			name:   "rejects arbitrary public origin without allowlist",
			origin: "https://demo.example",
			want:   false,
		},
		{
			name:       "allows any origin in lax mode",
			origin:     "https://demo.example",
			secureMode: security.SecureModeLax,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			security.SetSecureMode(tt.secureMode)
			t.Cleanup(func() {
				security.SetSecureMode("")
			})

			assert.Equal(t, tt.want, isTrustedCORSOrigin(tt.origin, tt.hasServerAuth, tt.accessAllowlist))
		})
	}
}

func TestCanConsumeMedia(t *testing.T) {
	tests := []struct {
		name            string
		origin          string
		reqHost         string
		remoteAddr      string
		hasServerAuth   bool
		accessAllowlist []string
		secureMode      string
		want            bool
	}{
		{
			name:          "allows authenticated public playback in strict mode",
			origin:        "https://demo.example",
			reqHost:       "demo.example",
			remoteAddr:    "203.0.113.10:51111",
			hasServerAuth: true,
			secureMode:    security.SecureModeStrict,
			want:          true,
		},
		{
			name:            "allows allowlisted public playback in strict mode",
			origin:          "https://demo.example",
			reqHost:         "demo.example",
			remoteAddr:      "203.0.113.10:51111",
			accessAllowlist: []string{"https://demo.example"},
			secureMode:      security.SecureModeStrict,
			want:            true,
		},
		{
			name:       "rejects public playback without auth or allowlist in strict mode",
			origin:     "https://demo.example",
			reqHost:    "demo.example",
			remoteAddr: "203.0.113.10:51111",
			secureMode: security.SecureModeStrict,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			security.SetSecureMode(tt.secureMode)
			t.Cleanup(func() {
				security.SetSecureMode("")
			})

			req := httptest.NewRequest(http.MethodPost, "/api/v1/mediastream/request", nil)
			req.Host = tt.reqHost
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			assert.Equal(t, tt.want, canConsumeMedia(req, tt.hasServerAuth, tt.accessAllowlist))
		})
	}
}

func TestMediaConsumptionHandlersDoNotUseStrictLocalOnlyBoundary(t *testing.T) {
	t.Cleanup(func() {
		security.SetSecureMode("")
	})

	security.SetSecureMode(security.SecureModeStrict)
	e := echo.New()
	h := &Handler{App: &core.App{Config: &core.Config{}}}
	h.App.Config.Server.Password = "configured"

	t.Run("directstream play local file falls through to binding for authenticated hosted requests", func(t *testing.T) {
		// this should no longer short-circuit on the old strict local-only guard.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/directstream/play/localfile", strings.NewReader(`{"path":`))
		req.Host = "demo.example"
		req.RemoteAddr = "203.0.113.10:51111"
		req.Header.Set("Origin", "https://demo.example")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HandleDirectstreamPlayLocalFile(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("mediastream request falls through to binding for authenticated hosted requests", func(t *testing.T) {
		// hosted playback requests should hit normal request validation instead of a strict local-only block.
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mediastream/request", strings.NewReader(`{"path":`))
		req.Host = "demo.example"
		req.RemoteAddr = "203.0.113.10:51111"
		req.Header.Set("Origin", "https://demo.example")
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.HandleRequestMediastreamMediaContainer(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestRequestMatchescontextClientId(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	req.AddCookie(&http.Cookie{Name: clientIdCookieName, Value: "client-1"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(clientIdCookieName, "client-1")

	assert.True(t, isSameContextClientId(c, "client-1"))
	assert.False(t, isSameContextClientId(c, "client-2"))
	assert.Equal(t, "client-1", getContextClientId(c))
	assert.False(t, isSameContextClientId(c, ""))
}

func TestResolveRequestClientId(t *testing.T) {
	t.Run("prefers server context", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mediastream/request", nil)
		req.AddCookie(&http.Cookie{Name: clientIdCookieName, Value: "cookie-client"})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(clientIdCookieName, "server-client")

		assert.Equal(t, "server-client", getRequestClientId(c, "body-client"))
	})

	t.Run("falls back to claimed id when context is missing", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mediastream/request", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		assert.Equal(t, "body-client", getRequestClientId(c, " body-client "))
	})
}

func TestUsesPrivilegedCommandSettings(t *testing.T) {
	t.Run("ignores default executable paths", func(t *testing.T) {
		// default app paths should still behave like the built-in integration.
		// The defaults are per-GOOS, so read them from the same source the check
		// does rather than hardcoding one platform's paths.
		settings := &models.Settings{
			MediaPlayer: &models.MediaPlayerSettings{
				Default: "vlc",
				VlcPath: defaultVLCPaths()[0],
			},
			Torrent: &models.TorrentSettings{
				Default:         "qbittorrent",
				QBittorrentPath: defaultQBittorrentPaths()[0],
			},
		}
		mediastreamSettings := &models.MediastreamSettings{
			FfmpegPath:  "ffmpeg",
			FfprobePath: "ffprobe",
		}

		assert.False(t, isPrivilegedMediaPlayer(settings))
		assert.False(t, isPrivilegedTorrentClient(settings))
		assert.False(t, isPrivilegedMediastream(mediastreamSettings))
	})

	t.Run("detects custom paths and custom args", func(t *testing.T) {
		// custom wrappers and launch args stay behind the trusted-origin gate.
		settings := &models.Settings{
			MediaPlayer: &models.MediaPlayerSettings{
				Default: "mpv",
				MpvPath: "/tmp/mpv-wrapper",
				MpvArgs: "--script=/tmp/hook.lua",
			},
			Torrent: &models.TorrentSettings{
				Default:         "qbittorrent",
				QBittorrentPath: "/tmp/qbit-wrapper",
			},
		}
		mediastreamSettings := &models.MediastreamSettings{
			FfmpegPath:  "/tmp/ffmpeg-wrapper",
			FfprobePath: "/tmp/ffprobe-wrapper",
		}

		assert.True(t, isPrivilegedMediaPlayer(settings))
		assert.True(t, isPrivilegedTorrentClient(settings))
		assert.True(t, isPrivilegedMediastream(mediastreamSettings))
	})
}

// withCapabilities installs a capability set for the duration of a test.
func withCapabilities(t *testing.T, capabilities ...string) {
	t.Helper()
	security.SetCapabilities(capabilities, true)
	t.Cleanup(func() {
		security.SetCapabilities(nil, true)
	})
}

// forgedLocalRequest is the shape an attacker uses to look like a local admin: a
// private source address (every in-cluster peer has one), a loopback Host, a
// matching Origin, and no forwarding headers. Every input here is chosen by the
// caller, which is exactly why none of them may grant anything.
func forgedLocalRequest(method string, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "127.0.0.1:43211"
	req.RemoteAddr = "10.42.0.7:51111"
	req.Header.Set("Origin", "http://127.0.0.1:43211")
	return req
}

func TestPrivilegedGuardsIgnoreRequestShape(t *testing.T) {
	e := echo.New()
	h := &Handler{App: &core.App{Config: &core.Config{}}}

	// A server password used to make isTrustedRequest short-circuit to "allowed".
	h.App.Config.Server.Password = "configured"

	guards := map[string]func(c echo.Context) error{
		"exec":       h.guardPrivilegedLocalExecution,
		"extensions": h.guardPrivilegedExtensionManagement,
		"selfupdate": h.guardSelfUpdate,
		"filesystem": h.guardStrictLocalOnlyAction,
	}

	requests := map[string]func() *http.Request{
		"forged loopback from a private peer": func() *http.Request {
			return forgedLocalRequest(http.MethodPost, "/api/v1/open-in-explorer")
		},
		"genuine loopback": func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/open-in-explorer", nil)
			req.Host = "127.0.0.1:43211"
			req.RemoteAddr = "127.0.0.1:51111"
			req.Header.Set("Origin", "http://127.0.0.1:43211")
			return req
		},
		"public origin": func() *http.Request {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/open-in-explorer", nil)
			req.Host = "demo.example"
			req.RemoteAddr = "203.0.113.10:51111"
			req.Header.Set("Origin", "https://demo.example")
			return req
		},
	}

	for _, secureMode := range []string{"", security.SecureModeLax, security.SecureModeHardened, security.SecureModeStrict} {
		for capability, guard := range guards {
			for shape, build := range requests {
				t.Run(capability+"/denied/"+shape+"/mode="+secureMode, func(t *testing.T) {
					security.SetSecureMode(secureMode)
					t.Cleanup(func() { security.SetSecureMode("") })
					withCapabilities(t)

					rec := httptest.NewRecorder()
					err := guard(e.NewContext(build(), rec))

					assert.ErrorIs(t, err, errGuardResponseWritten)
					assert.Equal(t, http.StatusForbidden, rec.Code)
				})

				t.Run(capability+"/granted/"+shape+"/mode="+secureMode, func(t *testing.T) {
					security.SetSecureMode(secureMode)
					t.Cleanup(func() { security.SetSecureMode("") })
					withCapabilities(t, capability)

					rec := httptest.NewRecorder()
					assert.NoError(t, guard(e.NewContext(build(), rec)))
				})
			}
		}
	}
}

func TestCapabilitiesAreIndependent(t *testing.T) {
	e := echo.New()
	h := &Handler{App: &core.App{Config: &core.Config{}}}

	// Granting exec must not hand over extension installation or self-update.
	withCapabilities(t, security.CapabilityExec)

	rec := httptest.NewRecorder()
	assert.NoError(t, h.guardPrivilegedLocalExecution(e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/open-in-explorer"), rec)))

	for _, guard := range []func(echo.Context) error{h.guardPrivilegedExtensionManagement, h.guardSelfUpdate, h.guardStrictLocalOnlyAction} {
		rec := httptest.NewRecorder()
		err := guard(e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/extensions/external/install"), rec))
		assert.ErrorIs(t, err, errGuardResponseWritten)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	}
}

func TestGuardStrictFilesystemPathContainsEveryCaller(t *testing.T) {
	e := echo.New()
	libraryRoot := t.TempDir()
	h := &Handler{App: &core.App{
		Config:   &core.Config{},
		Settings: &models.Settings{Library: &models.LibrarySettings{LibraryPaths: []string{libraryRoot}}},
	}}

	t.Run("denies a path outside the roots even for a forged local request", func(t *testing.T) {
		withCapabilities(t)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/download-torrent-file"), rec)

		err := h.guardStrictFilesystemPath(c, "/etc")
		assert.ErrorIs(t, err, errGuardResponseWritten)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("allows a path inside the roots without any capability", func(t *testing.T) {
		withCapabilities(t)

		// The containment check resolves paths, so the subdirectory has to exist.
		showDir := filepath.Join(libraryRoot, "Show")
		assert.NoError(t, os.MkdirAll(showDir, 0o755))

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/download-torrent-file"), rec)

		assert.NoError(t, h.guardStrictFilesystemPath(c, showDir))
	})

	t.Run("allows any path once the filesystem capability is granted", func(t *testing.T) {
		withCapabilities(t, security.CapabilityFilesystem)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/download-torrent-file"), rec)

		assert.NoError(t, h.guardStrictFilesystemPath(c, "/etc"))
	})
}

func TestGuardPrivilegedMediaPlayer(t *testing.T) {
	e := echo.New()
	h := &Handler{App: &core.App{Config: &core.Config{}}}
	h.App.Config.Server.Password = "configured"

	customPlayer := &models.Settings{
		MediaPlayer: &models.MediaPlayerSettings{
			Default: "mpv",
			MpvPath: "/tmp/mpv-wrapper",
			MpvArgs: "--no-config",
		},
	}

	t.Run("denies a custom player binary without the exec capability", func(t *testing.T) {
		withCapabilities(t)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/playback-manager/play"), rec)

		err := h.guardPrivilegedMediaPlayer(c, customPlayer)
		assert.ErrorIs(t, err, errGuardResponseWritten)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("allows a custom player binary once exec is granted", func(t *testing.T) {
		withCapabilities(t, security.CapabilityExec)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/playback-manager/play"), rec)

		assert.NoError(t, h.guardPrivilegedMediaPlayer(c, customPlayer))
	})

	t.Run("leaves the built-in web player ungated", func(t *testing.T) {
		withCapabilities(t)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPost, "/api/v1/playback-manager/play"), rec)

		assert.NoError(t, h.guardPrivilegedMediaPlayer(c, &models.Settings{MediaPlayer: &models.MediaPlayerSettings{}}))
	})
}

func TestGuardPrivilegedSettingsMutation(t *testing.T) {
	e := echo.New()
	h := &Handler{App: &core.App{Config: &core.Config{}}}

	prev := &models.Settings{
		MediaPlayer: &models.MediaPlayerSettings{Default: "mpv", MpvPath: "mpv"},
		Torrent:     &models.TorrentSettings{Default: "qbittorrent"},
	}

	t.Run("allows unrelated settings changes with no capability", func(t *testing.T) {
		withCapabilities(t)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPatch, "/api/v1/settings"), rec)

		assert.NoError(t, h.guardPrivilegedSettingsMutation(c, prev,
			&models.MediaPlayerSettings{Default: "mpv", MpvPath: "mpv", Host: "127.0.0.1"},
			&models.TorrentSettings{Default: "qbittorrent"}))
	})

	t.Run("denies repointing the player binary without exec", func(t *testing.T) {
		withCapabilities(t)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPatch, "/api/v1/settings"), rec)

		err := h.guardPrivilegedSettingsMutation(c, prev,
			&models.MediaPlayerSettings{Default: "mpv", MpvPath: "/data/payload"},
			&models.TorrentSettings{Default: "qbittorrent"})
		assert.ErrorIs(t, err, errGuardResponseWritten)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("allows repointing the player binary once exec is granted", func(t *testing.T) {
		withCapabilities(t, security.CapabilityExec)

		rec := httptest.NewRecorder()
		c := e.NewContext(forgedLocalRequest(http.MethodPatch, "/api/v1/settings"), rec)

		assert.NoError(t, h.guardPrivilegedSettingsMutation(c, prev,
			&models.MediaPlayerSettings{Default: "mpv", MpvPath: "/usr/bin/mpv"},
			&models.TorrentSettings{Default: "qbittorrent"}))
	})
}

func TestShouldRestrictSensitiveLocalInfo(t *testing.T) {
	t.Cleanup(func() {
		security.SetSecureMode("")
	})

	security.SetSecureMode("")
	assert.False(t, isStrictModeSensitive(false))

	// Strict mode redacts for every passwordless caller now: the old exemption for
	// callers that "looked local" was forgeable.
	security.SetSecureMode(security.SecureModeStrict)
	assert.True(t, isStrictModeSensitive(false))
	assert.False(t, isStrictModeSensitive(true))

	security.SetSecureMode(security.SecureModeLax)
	assert.False(t, isStrictModeSensitive(false))
}

func TestHandleDirectorySelectorRequiresFilesystemCapability(t *testing.T) {
	withCapabilities(t)

	h := &Handler{App: &core.App{Config: &core.Config{}}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/directory-selector", strings.NewReader(`{"input":"/tmp"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	err := h.HandleDirectorySelector(e.NewContext(req, rec))
	assert.ErrorIs(t, err, errGuardResponseWritten)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleTestDumpRequiresFilesystemCapability(t *testing.T) {
	withCapabilities(t)

	h := &Handler{App: &core.App{Config: &core.Config{}}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test-dump", strings.NewReader(`{"dir":"/tmp","userName":"test"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	err := h.HandleTestDump(e.NewContext(req, rec))
	assert.ErrorIs(t, err, errGuardResponseWritten)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
