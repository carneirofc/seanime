package core

import (
	"embed"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rs/zerolog"
)

func TestEmbeddedWebDist(t *testing.T) {
	// fs.Sub accepts any well-formed name, so an empty FS has to be told apart by
	// probing for the entry document; otherwise an API-only build would mount a
	// static handler with nothing behind it.
	cases := []struct {
		name  string
		fsys  fstest.MapFS
		serve bool
	}{
		{"empty", fstest.MapFS{}, false},
		{"web dir without index", fstest.MapFS{"web/.gitkeep": {}}, false},
		{"built UI", fstest.MapFS{"web/index.html": {Data: []byte("<html>")}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, serve := embeddedWebDist(tc.fsys); serve != tc.serve {
				t.Fatalf("serve = %v, want %v", serve, tc.serve)
			}
		})
	}

	if _, serve := embeddedWebDist(nil); serve {
		t.Fatal("a nil FS was treated as a servable UI")
	}
}

func newGateTestApp(t *testing.T, oidc bool) *App {
	t.Helper()
	logger := zerolog.Nop()
	cfg := &Config{}
	cfg.Web.AssetDir = t.TempDir()
	cfg.Offline.AssetDir = t.TempDir()
	if oidc {
		cfg.Server.Oidc.IssuerURL = "https://idp.example.com"
		cfg.Server.Oidc.ClientID = "seanime"
		cfg.Server.Oidc.ClientSecret = "secret"
	}
	return &App{Config: cfg, Logger: &logger}
}

func TestWebSessionGate(t *testing.T) {
	cases := []struct {
		name     string
		oidc     bool
		header   map[string]string
		status   int
		location string
	}{
		// Password mode never gated the bundle; the API enforces auth
		{"no oidc", false, nil, http.StatusNoContent, ""},
		{"oidc page load without session", true, map[string]string{"Sec-Fetch-Dest": "document"}, http.StatusFound, "/login"},
		{"oidc html accept without session", true, map[string]string{"Accept": "text/html,*/*"}, http.StatusFound, "/login"},
		{"oidc asset without session", true, map[string]string{"Sec-Fetch-Dest": "script"}, http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newGateTestApp(t, tc.oidc)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session-check", nil)
			for k, v := range tc.header {
				req.Header.Set(k, v)
			}

			status, location := app.WebSessionGate(req)
			if status != tc.status || location != tc.location {
				t.Fatalf("got (%d, %q), want (%d, %q)", status, location, tc.status, tc.location)
			}
		})
	}
}

func TestNewEchoAppAPIOnly(t *testing.T) {
	var empty embed.FS

	t.Run("no SPA fallback", func(t *testing.T) {
		e := NewEchoApp(newGateTestApp(t, false), &empty)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET / = %d, want 404 (the front web server owns UI routes)", rec.Code)
		}
		if rec.Header().Get("Cross-Origin-Embedder-Policy") != "" {
			t.Fatal("COEP was set on a UI route Go does not serve")
		}
	})

	t.Run("oidc login shell still served", func(t *testing.T) {
		e := NewEchoApp(newGateTestApp(t, true), &empty)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/api/v1/auth/oidc/login") {
			t.Fatalf("GET /login = %d, want the fallback login shell", rec.Code)
		}
	})
}
