package handlers

import (
	"net/http"
	"net/http/httptest"
	"seanime/internal/core"
	"testing"

	"github.com/labstack/echo/v4"
)

func newOidcTestHandler(t *testing.T) *Handler {
	t.Helper()

	cfg := &core.Config{}
	cfg.Server.ExternalURL = "https://seanime.example.com"
	cfg.Server.Oidc.IssuerURL = "https://idp.example.com"
	cfg.Server.Oidc.ClientID = "client"
	cfg.Server.Oidc.ClientSecret = "secret"

	return &Handler{App: &core.App{Config: cfg}}
}

func runCsrfMiddleware(t *testing.T, h *Handler, req *http.Request) (called bool, rec *httptest.ResponseRecorder) {
	t.Helper()

	e := echo.New()
	rec = httptest.NewRecorder()
	c := e.NewContext(req, rec)

	next := func(c echo.Context) error {
		called = true
		return nil
	}

	if err := h.csrfOriginCheckMiddleware(next)(c); err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	return called, rec
}

func TestCsrfOriginCheckMiddleware(t *testing.T) {
	h := newOidcTestHandler(t)

	tests := []struct {
		name      string
		method    string
		path      string
		headers   map[string]string
		wantAllow bool
	}{
		{
			name: "GET requests bypass the check", method: http.MethodGet, path: "/api/v1/library/collection",
			headers: map[string]string{"Origin": "https://evil.example"}, wantAllow: true,
		},
		{
			name: "sec-fetch-site same-origin allowed", method: http.MethodPost, path: "/api/v1/settings",
			headers: map[string]string{"Sec-Fetch-Site": "same-origin"}, wantAllow: true,
		},
		{
			name: "sec-fetch-site none allowed", method: http.MethodPost, path: "/api/v1/settings",
			headers: map[string]string{"Sec-Fetch-Site": "none"}, wantAllow: true,
		},
		{
			name: "sec-fetch-site cross-site rejected", method: http.MethodPost, path: "/api/v1/settings",
			headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantAllow: false,
		},
		{
			name: "cross-origin Origin rejected", method: http.MethodDelete, path: "/api/v1/settings",
			headers: map[string]string{"Origin": "https://evil.example"}, wantAllow: false,
		},
		{
			name: "external URL origin allowed", method: http.MethodPost, path: "/api/v1/settings",
			headers: map[string]string{"Origin": "https://seanime.example.com"}, wantAllow: true,
		},
		{
			name: "no browser metadata allowed (curl)", method: http.MethodPost, path: "/api/v1/settings",
			headers: nil, wantAllow: true,
		},
		{
			name: "nakama paths exempt", method: http.MethodPost, path: "/api/v1/nakama/host/action",
			headers: map[string]string{"Origin": "https://evil.example"}, wantAllow: true,
		},
		{
			name: "non-api paths exempt", method: http.MethodPost, path: "/events",
			headers: map[string]string{"Origin": "https://evil.example"}, wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://127.0.0.1:43211"+tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			allowed, rec := runCsrfMiddleware(t, h, req)
			if allowed != tt.wantAllow {
				t.Errorf("allowed = %v, want %v (status %d)", allowed, tt.wantAllow, rec.Code)
			}
			if !tt.wantAllow && rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 on rejection, got %d", rec.Code)
			}
		})
	}
}

func TestCsrfMiddlewareInactiveWithoutOidc(t *testing.T) {
	h := &Handler{App: &core.App{Config: &core.Config{}}}

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:43211/api/v1/settings", nil)
	req.Header.Set("Origin", "https://evil.example")

	allowed, _ := runCsrfMiddleware(t, h, req)
	if !allowed {
		t.Error("middleware must be inactive when OIDC mode is off")
	}
}
