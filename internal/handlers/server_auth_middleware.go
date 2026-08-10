package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

var errUnauthenticated = errors.New("UNAUTHENTICATED")

func (h *Handler) OptionalAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// OIDC mode replaces password auth entirely
		if h.App.IsOidcMode() {
			return h.oidcSessionAuth(next, c)
		}

		if h.App.Config.Server.Password == "" {
			return next(c)
		}

		req := c.Request()
		authKey := authFailureRateLimitKey(req)

		path := req.URL.Path
		passwordHash := req.Header.Get("X-Seanime-Token")

		// Allow the following paths to be accessed by anyone
		if path == "/api/v1/status" || // public but restricted
			path == "/events" { // for server events (auth handled by websocket handler)

			if path == "/api/v1/status" {
				// allow status requests by all clients but mark as unauthenticated
				if passwordHash != h.App.ServerPasswordHash {
					c.Set("unauthenticated", true)
				}
			}

			return next(c)
		}

		if passwordHash == h.App.ServerPasswordHash {
			authFailureRateLimits.reset(authKey)
			return next(c)
		}

		// Check HMAC token in query parameter
		token := req.URL.Query().Get("token")
		if token != "" {
			hmacAuth := h.App.GetServerHMACAuth()
			_, err := hmacAuth.ValidateToken(token, path)
			if err == nil {
				authFailureRateLimits.reset(authKey)
				return next(c)
			} else {
				h.App.Logger.Debug().Err(err).Str("path", path).Msg("server auth: HMAC token validation failed")
			}
		}

		if h.tryNakamaAuth(c) {
			return next(c)
		}

		if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
			return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
		}

		return h.RespondWithStatusError(c, http.StatusUnauthorized, errUnauthenticated)
	}
}

// oidcSessionAuth authenticates requests with the OIDC session cookie.
// The password header and password-derived HMAC tokens are never consulted.
func (h *Handler) oidcSessionAuth(next echo.HandlerFunc, c echo.Context) error {
	req := c.Request()
	path := req.URL.Path

	// Reachable without a session: the OIDC flow itself, and a restricted /status
	switch path {
	case "/api/v1/auth/oidc/login", "/api/v1/auth/oidc/callback":
		return next(c)
	case "/api/v1/status":
		if _, ok := h.resolveServerSession(req); !ok {
			c.Set("unauthenticated", true)
		}
		return next(c)
	case "/events":
		// auth handled by the websocket handler
		return next(c)
	}

	if session, ok := h.resolveServerSession(req); ok {
		c.Set("sessionSubject", session.Subject)
		c.Set("sessionUsername", session.Username)
		c.Set("sessionPicture", session.Picture)
		return next(c)
	}

	// Media/query HMAC tokens (signed with the per-boot secret) keep working for
	// external players that cannot send cookies
	if token := req.URL.Query().Get("token"); token != "" {
		hmacAuth := h.App.GetServerHMACAuth()
		if _, err := hmacAuth.ValidateToken(token, path); err == nil {
			return next(c)
		}
	}

	// Nakama peers authenticate with their own machine credential
	if h.tryNakamaAuth(c) {
		return next(c)
	}

	authKey := authFailureRateLimitKey(req)
	if _, err := req.Cookie(h.sessionCookieName()); err == nil {
		// A cookie was presented but is invalid/expired: charge the failure limiter.
		// A merely absent cookie is not charged (every first page load would trip it).
		if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
			return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
		}
	}

	return h.RespondWithStatusError(c, http.StatusUnauthorized, errUnauthenticated)
}

// tryNakamaAuth authenticates Nakama peer connections on their dedicated paths
// using the Nakama host password header. Works in both password and OIDC modes.
func (h *Handler) tryNakamaAuth(c echo.Context) bool {
	if !h.App.Settings.GetNakama().Enabled || !h.App.Settings.GetNakama().IsHost {
		return false
	}

	req := c.Request()
	path := req.URL.Path
	if path != "/api/v1/nakama/ws" && !strings.HasPrefix(path, "/api/v1/nakama/host/") {
		return false
	}

	if req.Header.Get("X-Seanime-Nakama-Token") != h.App.Settings.GetNakama().HostPassword {
		return false
	}

	authFailureRateLimits.reset(authFailureRateLimitKey(req))
	c.Response().Header().Set("X-Seanime-Nakama-Is-Client", "true")
	return true
}
