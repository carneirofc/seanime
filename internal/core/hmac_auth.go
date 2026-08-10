package core

import (
	"seanime/internal/util"
	"time"
)

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
