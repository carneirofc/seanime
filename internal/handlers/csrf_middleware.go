package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

var errCrossOriginRequestDenied = errors.New("cross-origin request denied")

// csrfOriginCheckMiddleware protects cookie-authenticated state-changing
// requests while OIDC login is active. SameSite=Lax already blocks cross-site
// non-GET cookie sends in modern browsers; this server-side check backstops
// legacy browsers and Lax carve-outs without introducing CSRF tokens — the UI
// is served same-origin by this very server, so no legitimate cross-origin
// cookie-bearing client exists.
//
// Policy for POST/PATCH/PUT/DELETE on /api/:
//   - Sec-Fetch-Site present  -> must be "same-origin" or "none"
//   - else Origin/Referer set -> host must match the request host or ExternalURL
//   - all absent              -> non-browser client (curl, scripts), allowed;
//     these cannot be driven by a victim's browser
//
// Nakama peer paths are exempt: peers authenticate with a header credential,
// which cross-site requests cannot forge.
func (h *Handler) csrfOriginCheckMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !h.App.IsOidcMode() {
			return next(c)
		}

		req := c.Request()
		if req == nil || req.URL == nil || !strings.HasPrefix(req.URL.Path, "/api/") {
			return next(c)
		}

		switch req.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		default:
			return next(c)
		}

		if strings.HasPrefix(req.URL.Path, "/api/v1/nakama/") {
			return next(c)
		}

		if secFetchSite := strings.ToLower(strings.TrimSpace(req.Header.Get("Sec-Fetch-Site"))); secFetchSite != "" {
			if secFetchSite == "same-origin" || secFetchSite == "none" {
				return next(c)
			}
			return h.RespondWithStatusError(c, http.StatusForbidden, errCrossOriginRequestDenied)
		}

		rawOrigin := strings.TrimSpace(req.Header.Get("Origin"))
		if rawOrigin == "" {
			rawOrigin = strings.TrimSpace(req.Header.Get("Referer"))
		}
		if rawOrigin == "" {
			// Non-browser client
			return next(c)
		}

		parsed, ok := parseTrustedOrigin(rawOrigin)
		if !ok {
			return h.RespondWithStatusError(c, http.StatusForbidden, errCrossOriginRequestDenied)
		}

		if isReqSameLiteralHost(req, parsed) {
			return next(c)
		}

		if external, okExt := parseTrustedOrigin(h.App.Config.Server.ExternalURL); okExt &&
			strings.EqualFold(external.Hostname(), parsed.Hostname()) &&
			getEffectivePort(external.Scheme, external.Port()) == getEffectivePort(parsed.Scheme, parsed.Port()) {
			return next(c)
		}

		return h.RespondWithStatusError(c, http.StatusForbidden, errCrossOriginRequestDenied)
	}
}
