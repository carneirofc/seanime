package handlers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"seanime/internal/core"
	"seanime/internal/security"
	"strings"

	"github.com/labstack/echo/v4"
)

var errUnauthenticated = errors.New("UNAUTHENTICATED")

// secureCompare is a constant-time string equality check for credentials.
func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

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
				if !secureCompare(passwordHash, h.App.ServerPasswordHash) {
					c.Set("unauthenticated", true)
				}
			}

			return next(c)
		}

		if secureCompare(passwordHash, h.App.ServerPasswordHash) {
			authFailureRateLimits.reset(authKey)
			return next(c)
		}

		// Check HMAC token in query parameter (media endpoints only)
		token := req.URL.Query().Get("token")
		if token != "" && core.IsMediaTokenEndpoint(path) {
			if h.App.ValidateMediaToken(token, path) {
				authFailureRateLimits.reset(authKey)
				return next(c)
			}
			h.App.Logger.Debug().Str("path", path).Msg("server auth: HMAC token validation failed")
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
		if session, ok := h.resolveServerSession(req); ok {
			c.Set("sessionSubject", session.Subject)
			c.Set("sessionUsername", session.Username)
			c.Set("sessionPicture", session.Picture)
		} else {
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
	// external players that cannot send cookies; they are only honored on the
	// media endpoints they are minted for
	if token := req.URL.Query().Get("token"); token != "" && core.IsMediaTokenEndpoint(path) {
		if h.App.ValidateMediaToken(token, path) {
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
//
// This is a second, independent way into the API: it accepts a shared static
// password instead of the session, and it runs before the 401. Everything it opens
// (library listings, file streaming, debrid URLs) is therefore reachable without an
// OIDC session, so it needs both an operator opt-in and a real password.
func (h *Handler) tryNakamaAuth(c echo.Context) bool {
	nakama := h.App.Settings.GetNakama()
	if !nakama.Enabled || !nakama.IsHost {
		return false
	}

	// Peer-to-peer library sharing bypasses the session gate by design; it stays off
	// unless the operator asked for it.
	if !security.Allows(security.CapabilityNakamaHost) {
		return false
	}

	// An empty host password would otherwise authenticate an empty (or absent)
	// header: subtle.ConstantTimeCompare reports two zero-length inputs as equal, so
	// the comparison below is not a gate at all when no password is set.
	if nakama.HostPassword == "" {
		return false
	}

	req := c.Request()
	path := req.URL.Path
	if path != "/api/v1/nakama/ws" && !strings.HasPrefix(path, "/api/v1/nakama/host/") {
		return false
	}

	if !secureCompare(req.Header.Get("X-Seanime-Nakama-Token"), nakama.HostPassword) {
		return false
	}

	authFailureRateLimits.reset(authFailureRateLimitKey(req))
	c.Response().Header().Set("X-Seanime-Nakama-Is-Client", "true")
	return true
}
