package handlers

import (
	"strings"

	"github.com/labstack/echo/v4"
)

const ctxKeyOAuthAuthenticated = "oauth_authenticated"

// OAuthMiddleware validates an OAuth 2.0 Bearer token when present.
// If the token is valid it sets the "oauth_authenticated" context key so that
// OptionalAuthMiddleware lets the request through without checking the server
// password hash.
//
// Requests that carry no Authorization header are left untouched — they
// continue to the existing X-Seanime-Token / HMAC-based auth.
func (h *Handler) OAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return next(c)
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		if rawToken == "" {
			return next(c)
		}

		_, err := h.App.Database.GetValidOAuthAccessToken(rawToken)
		if err != nil {
			// Invalid / expired token — let OptionalAuthMiddleware handle it.
			return next(c)
		}

		// Mark the request as OAuth-authenticated so downstream middleware
		// (OptionalAuthMiddleware) skips the password-hash check.
		c.Set(ctxKeyOAuthAuthenticated, true)
		return next(c)
	}
}
