package core

import (
	"seanime/internal/util"
	"strconv"
	"strings"
	"time"
)

// mediaTokenEndpointPrefixes lists the only path prefixes on which server HMAC
// query tokens ("?token=") are minted and accepted. Tokens are meant for media
// URLs handed to external players and image loaders; scoping them here keeps a
// leaked URL from doubling as a general-purpose API credential.
var mediaTokenEndpointPrefixes = []string{
	"/api/v1/mediastream/",
	"/api/v1/directstream/",
	"/api/v1/torrentstream/",
	"/api/v1/nakama/stream",
	"/api/v1/image-proxy",
	"/api/v1/manga/local-page",
	"/api/v1/manga/downloads/chapter-archive",
	"/api/v1/manga/downloads/media-archive",
	"/api/v1/proxy",
	"/api/v1/report/issue/download",
	"/assets/",
	"/manga-downloads/",
	"/offline-assets/",
}

// IsMediaTokenEndpoint reports whether HMAC query-token auth applies to the path.
func IsMediaTokenEndpoint(path string) bool {
	for _, prefix := range mediaTokenEndpointPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// websocketTicketEndpoint is the endpoint claim carried by a websocket ticket.
//
// It is deliberately not one of the mediaTokenEndpointPrefixes: ValidateToken
// matches the claim against the path being requested, so a media URL token cannot
// be replayed to open an event stream, and a ticket cannot be replayed at a media
// endpoint.
const websocketTicketEndpoint = "/events"

// WebsocketTicketTTL bounds how long a websocket ticket stays usable.
//
// A ticket only has to survive the gap between being minted and the upgrade
// request that spends it, so it expires far sooner than a media token. That is
// the point of it: browsers cannot set headers on a websocket upgrade, so
// something has to travel in the URL, and a URL is logged by every proxy in
// front of us. A minute-old signed ticket is worth much less there than the
// credential it replaces.
const WebsocketTicketTTL = 60 * time.Second

// MediaTokenTTL bounds how long a server query token stays usable.
//
// These tokens travel in URLs, so they end up in player logs, proxy access logs
// and Referer headers, and unlike the session cookie they cannot be signed out.
// The window therefore needs to be long enough to cover a single viewing session
// and no longer. It is enforced on validation as well as on minting, so a token
// minted client-side in password mode cannot claim a longer life than this.
const MediaTokenTTL = 6 * time.Hour

// mediaTokenSessionSubjectPrefix marks a media token as bound to a login session.
const mediaTokenSessionSubjectPrefix = "session:"

// MediaTokenSessionSubject is the claim that binds a media token to the session
// that requested it. The session's numeric id is enough to look it up and is not
// itself a credential, so putting it in a URL leaks nothing the token does not.
func MediaTokenSessionSubject(sessionID uint) string {
	return mediaTokenSessionSubjectPrefix + strconv.FormatUint(uint64(sessionID), 10)
}

// ValidateMediaToken checks a server-minted query token against the requested path
// and, when the token names a login session, that the session is still live.
//
// Binding is what gives these tokens a revocation story: signing out, or letting a
// session lapse, kills every media URL minted under it instead of leaving them
// usable for the rest of their TTL.
func (a *App) ValidateMediaToken(token string, path string) bool {
	claims, err := a.GetServerHMACAuth().ValidateToken(token, path)
	if err != nil {
		return false
	}

	return a.isMediaTokenSubjectLive(claims.Subject)
}

func (a *App) isMediaTokenSubjectLive(subject string) bool {
	if subject == "" {
		// Unbound: minted without a request context (media URLs handed to external
		// players) or client-side in password mode, where there is no session to
		// bind to and the client holds the signing secret regardless.
		return true
	}

	rawID, found := strings.CutPrefix(subject, mediaTokenSessionSubjectPrefix)
	if !found {
		// An unrecognised binding is a binding we cannot check.
		return false
	}

	// 32 bits, not 64: uint is platform-sized, so parsing wider than the narrowest
	// uint would let a forged id truncate onto a real session on a 32-bit build.
	sessionID, err := strconv.ParseUint(rawID, 10, 32)
	if err != nil {
		return false
	}

	return a.IsServerSessionLive(uint(sessionID))
}

// serverHMACSecret is the signing secret behind every server query token (media
// URLs, websocket tickets).
//   - OIDC mode: the per-boot random MediaTokenSecret. The password is ignored
//     entirely and leaked tokens die on restart.
//   - Password mode: the hashed server password (legacy behavior, tokens survive
//     restarts).
//   - Passwordless, non-OIDC mode: the per-boot random secret rather than a fixed
//     constant, so signatures are never predictable. Tokens die on restart, which
//     is acceptable in this mode.
func (a *App) serverHMACSecret() string {
	switch {
	case a.IsOidcMode():
		return a.MediaTokenSecret
	case a.Config != nil && a.Config.Server.Password != "":
		return a.ServerPasswordHash
	default:
		return a.MediaTokenSecret
	}
}

// GetServerHMACAuth returns the HMAC authenticator used for server-minted query
// tokens (media URLs for external players, proxied streams, ...).
func (a *App) GetServerHMACAuth() *util.HMACAuth {
	return util.NewHMACAuth(a.serverHMACSecret(), MediaTokenTTL)
}

// GetWebsocketTicketAuth returns the authenticator for websocket tickets: the same
// signing secret as media tokens, a much shorter lifetime.
//
// The TTL lives on the validating instance, not just the claim, so it is the
// server that decides how long a ticket lasts. That matters in password mode,
// where the client holds the signing secret and picks its own "exp" — validating
// through this instance caps the effective expiry at issue time plus the TTL.
func (a *App) GetWebsocketTicketAuth() *util.HMACAuth {
	return util.NewHMACAuth(a.serverHMACSecret(), WebsocketTicketTTL)
}

// GenerateWebsocketTicket mints a ticket for opening the /events stream, bound to
// the given subject (empty for an unbound ticket).
func (a *App) GenerateWebsocketTicket(subject string) (string, error) {
	return a.GetWebsocketTicketAuth().GenerateTokenForSubject(websocketTicketEndpoint, subject)
}

// ValidateWebsocketTicket checks a ticket presented at the websocket upgrade and,
// when it names a login session, that the session is still live.
func (a *App) ValidateWebsocketTicket(ticket string) bool {
	if ticket == "" {
		return false
	}

	claims, err := a.GetWebsocketTicketAuth().ValidateToken(ticket, websocketTicketEndpoint)
	if err != nil {
		return false
	}

	// Exact match, not the prefix/wildcard matching ValidateToken is willing to do.
	// In password mode the client holds the signing secret, so it could otherwise
	// mint itself a "*" token and turn a ticket into a general-purpose credential.
	if claims.Endpoint != websocketTicketEndpoint {
		return false
	}

	return a.isMediaTokenSubjectLive(claims.Subject)
}

func (a *App) GetClientIdentityHMACAuth() *util.HMACAuth {
	if a.ClientIdentitySecret == "" {
		a.ClientIdentitySecret = util.GenerateCryptoID()
	}

	return util.NewHMACAuth(a.ClientIdentitySecret, 24*time.Hour)
}
