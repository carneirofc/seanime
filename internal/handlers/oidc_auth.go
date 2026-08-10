package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"seanime/internal/core"
	"seanime/internal/database/models"
	"seanime/internal/util"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

// OIDC login flow. Active only when server.oidc is configured (App.IsOidcMode).
// While active, the server password is ignored and browser access requires a
// ServerSession established through the IdP.

const (
	oidcCallbackPath       = "/api/v1/auth/oidc/callback"
	oidcStateCookieMaxAge  = 10 * time.Minute
	errParamIdpDenied      = "idp_denied"
	errParamNotAllowed     = "not_allowed"
	errParamOidcFailed     = "oidc_failed"
	oidcLoginErrorPagePath = "/login"
)

var errOidcNotReady = errors.New("OIDC provider discovery has not completed yet")

// oidcRuntime holds the lazily-discovered provider metadata. Discovery is
// retried on demand so the server still boots when the IdP is unreachable.
type oidcRuntime struct {
	mu       sync.Mutex
	provider *oidc.Provider
	oauthCfg *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

var oidcRt oidcRuntime

func (h *Handler) getOidcRuntime(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	oidcRt.mu.Lock()
	defer oidcRt.mu.Unlock()

	if oidcRt.provider != nil {
		return oidcRt.oauthCfg, oidcRt.verifier, nil
	}

	cfg := h.App.Config.Server.Oidc

	discoveryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, cfg.IssuerURL)
	if err != nil {
		h.App.Logger.Warn().Err(err).Str("issuer", cfg.IssuerURL).Msg("oidc: Provider discovery failed")
		return nil, nil, errOidcNotReady
	}

	oidcRt.provider = provider
	oidcRt.oauthCfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  strings.TrimRight(h.App.Config.Server.ExternalURL, "/") + oidcCallbackPath,
		Scopes:       cfg.Scopes,
	}
	oidcRt.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	h.App.Logger.Info().Str("issuer", cfg.IssuerURL).Msg("oidc: Provider discovery succeeded")

	return oidcRt.oauthCfg, oidcRt.verifier, nil
}

// ---------------------------------------------------------------------------
// Cookies
// ---------------------------------------------------------------------------

// Cookie names use the __Host- prefix (requires Secure, Path=/, no Domain) unless
// the config opts into insecure local development.
func (h *Handler) sessionCookieName() string {
	return h.App.SessionCookieName()
}

func (h *Handler) oidcStateCookieName() string {
	if h.App.Config.Server.Oidc.AllowInsecure {
		return "seanime-oidc-state"
	}
	return "__Host-seanime-oidc-state"
}

func (h *Handler) newAuthCookie(name, value string, maxAge time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   !h.App.Config.Server.Oidc.AllowInsecure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (h *Handler) expireAuthCookie(c echo.Context, name string) {
	cookie := h.newAuthCookie(name, "", 0)
	cookie.MaxAge = -1
	c.SetCookie(cookie)
}

// ---------------------------------------------------------------------------
// Redirect validation
// ---------------------------------------------------------------------------

// safeRedirectPath only accepts same-site absolute paths, preventing open redirects.
func safeRedirectPath(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, "\\") {
		return "/"
	}
	if u, err := url.Parse(p); err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return p
}

func oidcErrorRedirect(c echo.Context, errParam string) error {
	return c.Redirect(http.StatusFound, oidcLoginErrorPagePath+"?error="+url.QueryEscape(errParam))
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// HandleOidcLogin
//
//	@summary starts the OIDC login flow.
//	@desc Redirects the browser to the configured identity provider's authorization endpoint.
//	@desc Only available when OIDC login is configured on the server.
//	@route /api/v1/auth/oidc/login [GET]
//	@returns bool
func (h *Handler) HandleOidcLogin(c echo.Context) error {
	if !h.App.IsOidcMode() {
		return h.RespondWithStatusError(c, http.StatusNotFound, errors.New("OIDC login is not configured"))
	}

	req := c.Request()
	if !oauthFlowRateLimits.allow(oauthFlowRateLimitKey(req), maxOauthFlowsPerWindow, oauthFlowWindow) {
		return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
	}

	oauthCfg, _, err := h.getOidcRuntime(req.Context())
	if err != nil {
		return h.RespondWithStatusError(c, http.StatusServiceUnavailable, err)
	}

	state, err := generateRandomToken(16)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	nonce, err := generateRandomToken(16)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	pkceVerifier := oauth2.GenerateVerifier()

	stateValues := url.Values{}
	stateValues.Set("state", state)
	stateValues.Set("nonce", nonce)
	stateValues.Set("verifier", pkceVerifier)
	stateValues.Set("redirect", safeRedirectPath(c.QueryParam("redirect")))
	c.SetCookie(h.newAuthCookie(h.oidcStateCookieName(), url.QueryEscape(stateValues.Encode()), oidcStateCookieMaxAge))

	authURL := oauthCfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(pkceVerifier))
	return c.Redirect(http.StatusFound, authURL)
}

// HandleOidcCallback
//
//	@summary completes the OIDC login flow.
//	@desc Exchanges the authorization code, verifies the ID token, checks the identity
//	@desc against the configured allowlist and establishes a login session cookie.
//	@route /api/v1/auth/oidc/callback [GET]
//	@returns bool
func (h *Handler) HandleOidcCallback(c echo.Context) error {
	if !h.App.IsOidcMode() {
		return h.RespondWithStatusError(c, http.StatusNotFound, errors.New("OIDC login is not configured"))
	}

	req := c.Request()
	authKey := authFailureRateLimitKey(req)
	if !oauthFlowRateLimits.allow(oauthFlowRateLimitKey(req), maxOauthFlowsPerWindow, oauthFlowWindow) {
		return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
	}

	oauthCfg, verifier, err := h.getOidcRuntime(req.Context())
	if err != nil {
		return h.RespondWithStatusError(c, http.StatusServiceUnavailable, err)
	}

	// The state cookie is single-use regardless of the outcome
	stateCookie, err := req.Cookie(h.oidcStateCookieName())
	h.expireAuthCookie(c, h.oidcStateCookieName())
	if err != nil || stateCookie.Value == "" {
		return oidcErrorRedirect(c, errParamOidcFailed)
	}
	rawState, err := url.QueryUnescape(stateCookie.Value)
	if err != nil {
		return oidcErrorRedirect(c, errParamOidcFailed)
	}
	stateValues, err := url.ParseQuery(rawState)
	if err != nil {
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	if errCode := c.QueryParam("error"); errCode != "" {
		h.App.Logger.Warn().Str("error", errCode).Msg("oidc: IdP returned an error")
		return oidcErrorRedirect(c, errParamIdpDenied)
	}

	if c.QueryParam("state") == "" || c.QueryParam("state") != stateValues.Get("state") {
		if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
			return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
		}
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	exchangeCtx, cancel := context.WithTimeout(req.Context(), 15*time.Second)
	defer cancel()
	// The IdP tokens are used once to establish identity and then discarded
	token, err := oauthCfg.Exchange(exchangeCtx, c.QueryParam("code"), oauth2.VerifierOption(stateValues.Get("verifier")))
	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("oidc: Code exchange failed")
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		h.App.Logger.Warn().Msg("oidc: Token response did not contain an ID token")
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	idToken, err := verifier.Verify(exchangeCtx, rawIDToken)
	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("oidc: ID token verification failed")
		return oidcErrorRedirect(c, errParamOidcFailed)
	}
	if idToken.Nonce != stateValues.Get("nonce") {
		h.App.Logger.Warn().Msg("oidc: Nonce mismatch")
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return oidcErrorRedirect(c, errParamOidcFailed)
	}

	claimString := func(name string) string {
		v, _ := claims[name].(string)
		return v
	}

	oidcCfg := h.App.Config.Server.Oidc
	username := claimString(oidcCfg.UsernameClaim)
	if username == "" {
		username = claimString("email")
	}

	if !isOidcIdentityAllowed(idToken.Subject, username, oidcCfg.AllowedSubjects, oidcCfg.AllowedUsernames) {
		h.App.Logger.Warn().Str("sub", idToken.Subject).Str("username", username).Msg("oidc: Identity not in allowlist")
		if !authFailureRateLimits.allow(authKey, maxAuthFailuresPerWindow, authFailureWindow) {
			return h.RespondWithStatusError(c, http.StatusTooManyRequests, errTooManyAuthenticationAttempts)
		}
		return oidcErrorRedirect(c, errParamNotAllowed)
	}
	if !slices.Contains(oidcCfg.AllowedSubjects, idToken.Subject) {
		h.App.Logger.Warn().Str("sub", idToken.Subject).Str("username", username).
			Msg("oidc: Identity admitted by username; pin server.oidc.allowedSubjects to this subject so the account survives username changes")
	}

	rawSessionToken, err := generateRandomToken(32)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	now := time.Now()
	session := &models.ServerSession{
		TokenHash:  util.HashSHA256Hex(rawSessionToken),
		Subject:    idToken.Subject,
		Username:   username,
		Email:      claimString("email"),
		Picture:    claimString("picture"),
		CreatedIP:  requestClientIP(req),
		ExpiresAt:  now.Add(time.Duration(oidcCfg.SessionTTLDays) * 24 * time.Hour),
		LastSeenAt: now,
	}
	if _, err := h.App.Database.CreateServerSession(session); err != nil {
		return h.RespondWithError(c, err)
	}

	authFailureRateLimits.reset(authKey)
	c.SetCookie(h.newAuthCookie(h.sessionCookieName(), rawSessionToken, time.Duration(oidcCfg.SessionTTLDays)*24*time.Hour))
	h.App.Logger.Info().Str("username", username).Str("sub", idToken.Subject).Msg("oidc: Login successful")

	return c.Redirect(http.StatusFound, safeRedirectPath(stateValues.Get("redirect")))
}

// HandleOidcLogout
//
//	@summary logs out the current OIDC session.
//	@desc Deletes the server-side session and expires the session cookie.
//	@route /api/v1/auth/oidc/logout [POST]
//	@returns bool
func (h *Handler) HandleOidcLogout(c echo.Context) error {
	if !h.App.IsOidcMode() {
		return h.RespondWithStatusError(c, http.StatusNotFound, errors.New("OIDC login is not configured"))
	}

	if cookie, err := c.Request().Cookie(h.sessionCookieName()); err == nil && cookie.Value != "" {
		tokenHash := util.HashSHA256Hex(cookie.Value)
		_ = h.App.Database.DeleteServerSession(tokenHash)
		core.InvalidateServerSessionCache(tokenHash)
	}
	h.expireAuthCookie(c, h.sessionCookieName())

	return h.RespondWithData(c, true)
}

// HandleGetMediaToken
//
//	@summary mints an HMAC query token for the given endpoint path.
//	@desc While OIDC login is active, media query tokens are signed with a server-side
//	@desc secret and cannot be minted client-side; authenticated clients request them here
//	@desc (e.g. for external player URLs).
//	@route /api/v1/auth/media-token [GET]
//	@returns string
func (h *Handler) HandleGetMediaToken(c echo.Context) error {
	endpoint := c.QueryParam("endpoint")
	if endpoint == "" || !strings.HasPrefix(endpoint, "/") || strings.HasPrefix(endpoint, "//") {
		return h.RespondWithStatusError(c, http.StatusBadRequest, errors.New("invalid endpoint"))
	}

	token, err := h.App.GetServerHMACAuth().GenerateToken(endpoint)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, token)
}

func isOidcIdentityAllowed(subject, username string, allowedSubjects, allowedUsernames []string) bool {
	if slices.Contains(allowedSubjects, subject) {
		return true
	}
	if username == "" {
		return false
	}
	for _, allowed := range allowedUsernames {
		if strings.EqualFold(allowed, username) {
			return true
		}
	}
	return false
}

// resolveServerSession validates the session cookie on the request; see core.App.ResolveServerSession.
func (h *Handler) resolveServerSession(req *http.Request) (*models.ServerSession, bool) {
	return h.App.ResolveServerSession(req)
}
