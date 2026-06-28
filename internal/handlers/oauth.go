package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"seanime/internal/database/models"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// -----------------------------------------------------------------------
// Token helpers
// -----------------------------------------------------------------------

func generateRandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
	authCodeTTL     = 10 * time.Minute
)

// -----------------------------------------------------------------------
// Client Management
// -----------------------------------------------------------------------

// HandleOAuthListClients
//
//	@summary returns all registered OAuth 2.0 clients.
//	@route /api/v1/oauth/clients [GET]
//	@returns []models.OAuthClient
func (h *Handler) HandleOAuthListClients(c echo.Context) error {
	clients, err := h.App.Database.ListOAuthClients()
	if err != nil {
		return h.RespondWithError(c, err)
	}
	// Redact secrets in the listing
	for _, cl := range clients {
		cl.ClientSecret = "***"
	}
	return h.RespondWithData(c, clients)
}

// HandleOAuthCreateClient
//
//	@summary registers a new OAuth 2.0 client application.
//	@route /api/v1/oauth/clients [POST]
//	@returns models.OAuthClient
func (h *Handler) HandleOAuthCreateClient(c echo.Context) error {
	type body struct {
		Name         string `json:"name"`
		RedirectURIs string `json:"redirectUris"` // comma or newline-separated
		Scopes       string `json:"scopes"`       // space-separated, defaults to "read"
		Trusted      bool   `json:"trusted"`
	}
	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}
	if strings.TrimSpace(b.Name) == "" {
		return h.RespondWithError(c, errors.New("client name is required"))
	}
	// Normalise redirect URIs (accept newline-separated input too)
	b.RedirectURIs = strings.Join(
		strings.Fields(strings.ReplaceAll(b.RedirectURIs, "\n", " ")),
		",",
	)
	if b.Scopes == "" {
		b.Scopes = "read"
	}

	clientID, err := generateRandomToken(16)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	secret, err := generateRandomToken(32)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	client := &models.OAuthClient{
		ClientID:     clientID,
		ClientSecret: secret,
		Name:         b.Name,
		RedirectURIs: b.RedirectURIs,
		Scopes:       b.Scopes,
		Trusted:      b.Trusted,
	}
	created, err := h.App.Database.CreateOAuthClient(client)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	// Return the full record (including secret) only on creation.
	return h.RespondWithData(c, created)
}

// HandleOAuthDeleteClient
//
//	@summary deletes an OAuth 2.0 client and revokes all its tokens.
//	@route /api/v1/oauth/clients/:id [DELETE]
func (h *Handler) HandleOAuthDeleteClient(c echo.Context) error {
	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
		return h.RespondWithError(c, errors.New("invalid client id"))
	}
	client, err := h.App.Database.GetOAuthClientByClientID(c.Param("id"))
	if err == nil {
		_ = h.App.Database.RevokeAllOAuthTokensForClient(client.ClientID)
	}
	if err := h.App.Database.DeleteOAuthClient(id); err != nil {
		return h.RespondWithError(c, err)
	}
	return h.RespondWithData(c, true)
}

// -----------------------------------------------------------------------
// Authorization Endpoint
// -----------------------------------------------------------------------

// HandleOAuthAuthorize (GET)
//
//	@summary validates the authorization request and returns client details for the consent UI.
//	@route /api/v1/oauth/authorize [GET]
func (h *Handler) HandleOAuthAuthorize(c echo.Context) error {
	clientID := c.QueryParam("client_id")
	redirectURI := c.QueryParam("redirect_uri")
	scope := c.QueryParam("scope")
	state := c.QueryParam("state")
	responseType := c.QueryParam("response_type")

	if responseType != "code" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_response_type"})
	}

	client, err := h.App.Database.GetOAuthClientByClientID(clientID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_client"})
	}

	// Validate redirect_uri
	if err := validateRedirectURI(client.RedirectURIs, redirectURI); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
	}

	// Validate scopes
	if err := validateScopes(client.Scopes, scope); err != nil {
		return sendOAuthError(c, redirectURI, state, "invalid_scope")
	}

	// For trusted clients skip the consent step: immediately issue a code.
	if client.Trusted {
		return h.issueAuthCode(c, client, redirectURI, scope, state)
	}

	// Otherwise return client info so the SPA can render a consent page.
	return h.RespondWithData(c, map[string]interface{}{
		"clientId":    client.ClientID,
		"clientName":  client.Name,
		"scopes":      scope,
		"redirectUri": redirectURI,
		"state":       state,
	})
}

// HandleOAuthApprove (POST) — called by the SPA after the user consents.
//
//	@summary issues an authorization code after the user grants consent.
//	@route /api/v1/oauth/approve [POST]
func (h *Handler) HandleOAuthApprove(c echo.Context) error {
	// Require the user to be authenticated (server password) to approve.
	if h.App.Config.Server.Password != "" {
		token := c.Request().Header.Get("X-Seanime-Token")
		if token != h.App.ServerPasswordHash {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
	}

	type body struct {
		ClientID    string `json:"clientId"`
		RedirectURI string `json:"redirectUri"`
		Scope       string `json:"scope"`
		State       string `json:"state"`
		Approved    bool   `json:"approved"`
	}
	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	if !b.Approved {
		return sendOAuthError(c, b.RedirectURI, b.State, "access_denied")
	}

	client, err := h.App.Database.GetOAuthClientByClientID(b.ClientID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_client"})
	}

	if err := validateRedirectURI(client.RedirectURIs, b.RedirectURI); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
	}

	return h.issueAuthCode(c, client, b.RedirectURI, b.Scope, b.State)
}

func (h *Handler) issueAuthCode(c echo.Context, client *models.OAuthClient, redirectURI, scope, state string) error {
	code, err := generateRandomToken(20)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	expiry := time.Now().Add(authCodeTTL)
	_, err = h.App.Database.CreateOAuthAuthCode(&models.OAuthAuthCode{
		Code:        code,
		ClientID:    client.ClientID,
		RedirectURI: redirectURI,
		Scope:       scope,
		ExpiresAt:   expiry,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	// Build redirect URL
	redir, _ := url.Parse(redirectURI)
	q := redir.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	redir.RawQuery = q.Encode()

	return c.JSON(http.StatusOK, map[string]string{"redirectTo": redir.String()})
}

// -----------------------------------------------------------------------
// Token Endpoint
// -----------------------------------------------------------------------

// HandleOAuthToken
//
//	@summary exchanges an authorization code or refresh token for access/refresh tokens, or issues tokens directly for client_credentials.
//	@route /api/v1/oauth/token [POST]
func (h *Handler) HandleOAuthToken(c echo.Context) error {
	grantType := c.FormValue("grant_type")
	if grantType == "" {
		// Also accept JSON body
		type jsonBody struct {
			GrantType string `json:"grant_type"`
		}
		var jb jsonBody
		_ = c.Bind(&jb)
		grantType = jb.GrantType
	}

	switch grantType {
	case "authorization_code":
		return h.handleAuthCodeGrant(c)
	case "refresh_token":
		return h.handleRefreshGrant(c)
	case "client_credentials":
		return h.handleClientCredentialsGrant(c)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
	}
}

func (h *Handler) handleAuthCodeGrant(c echo.Context) error {
	code := firstNonEmpty(c.FormValue("code"), c.QueryParam("code"))
	clientID := firstNonEmpty(c.FormValue("client_id"), c.QueryParam("client_id"))
	clientSecret := firstNonEmpty(c.FormValue("client_secret"), c.QueryParam("client_secret"))
	redirectURI := firstNonEmpty(c.FormValue("redirect_uri"), c.QueryParam("redirect_uri"))

	client, err := h.App.Database.GetOAuthClientByClientID(clientID)
	if err != nil || client.ClientSecret != clientSecret {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
	}

	authCode, err := h.App.Database.ConsumeOAuthAuthCode(code, clientID, redirectURI)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant", "error_description": err.Error()})
	}

	return h.issueTokenPair(c, client, authCode.Scope)
}

func (h *Handler) handleRefreshGrant(c echo.Context) error {
	refreshToken := firstNonEmpty(c.FormValue("refresh_token"), c.QueryParam("refresh_token"))
	clientID := firstNonEmpty(c.FormValue("client_id"), c.QueryParam("client_id"))
	clientSecret := firstNonEmpty(c.FormValue("client_secret"), c.QueryParam("client_secret"))

	client, err := h.App.Database.GetOAuthClientByClientID(clientID)
	if err != nil || client.ClientSecret != clientSecret {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
	}

	existing, err := h.App.Database.GetOAuthAccessTokenByRefresh(refreshToken)
	if err != nil || existing.ClientID != clientID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}

	// Revoke old token pair
	_ = h.App.Database.RevokeOAuthAccessToken(existing.AccessToken)

	return h.issueTokenPair(c, client, existing.Scope)
}

func (h *Handler) handleClientCredentialsGrant(c echo.Context) error {
	clientID := firstNonEmpty(c.FormValue("client_id"), c.QueryParam("client_id"))
	clientSecret := firstNonEmpty(c.FormValue("client_secret"), c.QueryParam("client_secret"))
	scope := firstNonEmpty(c.FormValue("scope"), c.QueryParam("scope"))

	client, err := h.App.Database.GetOAuthClientByClientID(clientID)
	if err != nil || client.ClientSecret != clientSecret {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
	}

	if err := validateScopes(client.Scopes, scope); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_scope"})
	}
	if scope == "" {
		scope = client.Scopes
	}

	return h.issueTokenPair(c, client, scope)
}

func (h *Handler) issueTokenPair(c echo.Context, client *models.OAuthClient, scope string) error {
	accessToken, err := generateRandomToken(32)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	refreshToken, err := generateRandomToken(32)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	expiresAt := time.Now().Add(accessTokenTTL)
	_, err = h.App.Database.CreateOAuthAccessToken(&models.OAuthAccessToken{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ClientID:     client.ClientID,
		Scope:        scope,
		ExpiresAt:    &expiresAt,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    int(accessTokenTTL.Seconds()),
		"scope":         scope,
	})
}

// -----------------------------------------------------------------------
// Revoke Endpoint
// -----------------------------------------------------------------------

// HandleOAuthRevoke
//
//	@summary revokes an access or refresh token.
//	@route /api/v1/oauth/revoke [POST]
func (h *Handler) HandleOAuthRevoke(c echo.Context) error {
	token := firstNonEmpty(c.FormValue("token"), c.QueryParam("token"))
	if token == "" {
		type jsonBody struct {
			Token string `json:"token"`
		}
		var jb jsonBody
		if err := c.Bind(&jb); err == nil {
			token = jb.Token
		}
	}
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing token"})
	}
	// Per RFC 7009, always return 200 even if token is unknown.
	_ = h.App.Database.RevokeOAuthAccessToken(token)
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func validateRedirectURI(allowedCSV, requested string) error {
	if requested == "" {
		return errors.New("redirect_uri is required")
	}
	for _, uri := range strings.Split(allowedCSV, ",") {
		if strings.TrimSpace(uri) == strings.TrimSpace(requested) {
			return nil
		}
	}
	return fmt.Errorf("redirect_uri %q is not registered for this client", requested)
}

func validateScopes(allowedSpaceSep, requested string) error {
	if requested == "" {
		return nil // caller may default to client scopes
	}
	allowed := make(map[string]bool)
	for _, s := range strings.Fields(allowedSpaceSep) {
		allowed[s] = true
	}
	for _, s := range strings.Fields(requested) {
		if !allowed[s] {
			return fmt.Errorf("scope %q is not allowed for this client", s)
		}
	}
	return nil
}

func sendOAuthError(c echo.Context, redirectURI, state, errCode string) error {
	if redirectURI == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errCode})
	}
	redir, err := url.Parse(redirectURI)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": errCode})
	}
	q := redir.Query()
	q.Set("error", errCode)
	if state != "" {
		q.Set("state", state)
	}
	redir.RawQuery = q.Encode()
	return c.JSON(http.StatusOK, map[string]string{"redirectTo": redir.String()})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
