package core

import (
	"seanime/internal/util"
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

// GetServerHMACAuth returns the HMAC authenticator used for server-minted query
// tokens (media URLs for external players, proxied streams, ...).
//   - OIDC mode: derived from the per-boot random MediaTokenSecret. The password
//     is ignored entirely and leaked tokens die on restart.
//   - Password mode: derived from the hashed server password (legacy behavior,
//     tokens survive restarts).
func (a *App) GetServerHMACAuth() *util.HMACAuth {
	var secret string
	switch {
	case a.IsOidcMode():
		secret = a.MediaTokenSecret
	case a.Config != nil && a.Config.Server.Password != "":
		secret = a.ServerPasswordHash
	default:
		secret = "seanime-default-secret"
	}

	return util.NewHMACAuth(secret, 24*time.Hour)
}

func (a *App) GetClientIdentityHMACAuth() *util.HMACAuth {
	if a.ClientIdentitySecret == "" {
		a.ClientIdentitySecret = util.GenerateCryptoID()
	}

	return util.NewHMACAuth(a.ClientIdentitySecret, 24*time.Hour)
}
