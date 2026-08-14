package handlers

import (
	"net/http"
	"net/http/httptest"
	"seanime/internal/core"
	"seanime/internal/database/models"
	"seanime/internal/security"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// Nakama host endpoints authenticate with a shared static password instead of the
// session, and tryNakamaAuth runs before the 401 in both password and OIDC modes.
// Everything it opens — library listings, file streams, debrid URLs — is therefore
// reachable without a session, so an empty password must never authenticate.
func TestTryNakamaAuth(t *testing.T) {
	e := echo.New()

	newHandler := func(hostPassword string) *Handler {
		return &Handler{App: &core.App{
			Config: &core.Config{},
			Settings: &models.Settings{
				Nakama: &models.NakamaSettings{
					Enabled:      true,
					IsHost:       true,
					HostPassword: hostPassword,
				},
			},
		}}
	}

	newContext := func(token string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/nakama/host/anime/library", nil)
		if token != "" {
			req.Header.Set("X-Seanime-Nakama-Token", token)
		}
		rec := httptest.NewRecorder()
		return e.NewContext(req, rec), rec
	}

	t.Run("denies an empty host password even with a matching empty header", func(t *testing.T) {
		// subtle.ConstantTimeCompare reports two zero-length inputs as equal, so
		// without an explicit check this is an unauthenticated open door.
		security.SetCapabilities([]string{security.CapabilityNakamaHost}, true)
		t.Cleanup(func() { security.SetCapabilities(nil, true) })

		h := newHandler("")

		c, _ := newContext("")
		assert.False(t, h.tryNakamaAuth(c))

		c, _ = newContext("anything")
		assert.False(t, h.tryNakamaAuth(c))
	})

	t.Run("denies a correct password without the nakama-host capability", func(t *testing.T) {
		security.SetCapabilities(nil, true)
		t.Cleanup(func() { security.SetCapabilities(nil, true) })

		h := newHandler("a-real-password")
		c, _ := newContext("a-real-password")
		assert.False(t, h.tryNakamaAuth(c))
	})

	t.Run("accepts a correct password once the capability is granted", func(t *testing.T) {
		security.SetCapabilities([]string{security.CapabilityNakamaHost}, true)
		t.Cleanup(func() { security.SetCapabilities(nil, true) })

		h := newHandler("a-real-password")

		c, _ := newContext("a-real-password")
		assert.True(t, h.tryNakamaAuth(c))

		c, _ = newContext("wrong-password")
		assert.False(t, h.tryNakamaAuth(c))
	})
}

func TestValidateNakamaHostSettings(t *testing.T) {
	assert.NoError(t, validateNakamaHostSettings(nil))
	assert.NoError(t, validateNakamaHostSettings(&models.NakamaSettings{}))

	// Not hosting: the password is irrelevant.
	assert.NoError(t, validateNakamaHostSettings(&models.NakamaSettings{Enabled: true}))

	assert.Error(t, validateNakamaHostSettings(&models.NakamaSettings{Enabled: true, IsHost: true}))
	assert.Error(t, validateNakamaHostSettings(&models.NakamaSettings{Enabled: true, IsHost: true, HostPassword: "short"}))
	assert.NoError(t, validateNakamaHostSettings(&models.NakamaSettings{Enabled: true, IsHost: true, HostPassword: "long-enough-password"}))
}
