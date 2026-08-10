package core

import (
	"net/http"
	"seanime/internal/database/models"
	"seanime/internal/util"
	"sync"
	"time"
)

// Server-side OIDC session resolution. Lives in core so both the API layer
// (handlers) and the embedded web-bundle gate (echo.go) can share it.

const (
	sessionCacheTTL      = 60 * time.Second
	sessionTouchInterval = 5 * time.Minute
)

type sessionCacheEntry struct {
	session   *models.ServerSession
	fetchedAt time.Time
}

var (
	serverSessionCache   = make(map[string]sessionCacheEntry)
	serverSessionCacheMu sync.Mutex
)

// SessionCookieName is the OIDC login session cookie. The __Host- prefix
// (requires Secure, Path=/, no Domain) is dropped only for insecure local development.
func (a *App) SessionCookieName() string {
	if a.Config.Server.Oidc.AllowInsecure {
		return "seanime-session"
	}
	return "__Host-seanime-session"
}

// InvalidateServerSessionCache evicts a session from the lookup cache (e.g. on logout).
func InvalidateServerSessionCache(tokenHash string) {
	serverSessionCacheMu.Lock()
	delete(serverSessionCache, tokenHash)
	serverSessionCacheMu.Unlock()
}

// ResolveServerSession validates the OIDC session cookie on the request.
// Lookups are cached briefly to avoid a database hit on every request.
func (a *App) ResolveServerSession(req *http.Request) (*models.ServerSession, bool) {
	if !a.IsOidcMode() || req == nil {
		return nil, false
	}

	cookie, err := req.Cookie(a.SessionCookieName())
	if err != nil || cookie.Value == "" {
		return nil, false
	}

	tokenHash := util.HashSHA256Hex(cookie.Value)
	now := time.Now()

	serverSessionCacheMu.Lock()
	if entry, ok := serverSessionCache[tokenHash]; ok && now.Sub(entry.fetchedAt) < sessionCacheTTL {
		session := entry.session
		serverSessionCacheMu.Unlock()
		if session == nil || now.After(session.ExpiresAt) || a.sessionPastAbsoluteCap(session, now) {
			return nil, false
		}
		a.touchServerSessionThrottled(session, now)
		return session, true
	}
	serverSessionCacheMu.Unlock()

	session, err := a.Database.GetValidServerSession(tokenHash)
	if err != nil {
		session = nil
	} else if a.sessionPastAbsoluteCap(session, now) {
		_ = a.Database.DeleteServerSession(tokenHash)
		session = nil
	}

	serverSessionCacheMu.Lock()
	serverSessionCache[tokenHash] = sessionCacheEntry{session: session, fetchedAt: now}
	// Opportunistically evict stale entries so the map cannot grow unbounded
	for key, entry := range serverSessionCache {
		if now.Sub(entry.fetchedAt) >= sessionCacheTTL {
			delete(serverSessionCache, key)
		}
	}
	serverSessionCacheMu.Unlock()

	if session == nil {
		return nil, false
	}

	a.touchServerSessionThrottled(session, now)
	return session, true
}

// touchServerSessionThrottled advances the sliding expiry at most once per touch interval.
func (a *App) touchServerSessionThrottled(session *models.ServerSession, now time.Time) {
	if now.Sub(session.LastSeenAt) <= sessionTouchInterval {
		return
	}

	newExpiry := now.Add(time.Duration(a.Config.Server.Oidc.SessionTTLDays) * 24 * time.Hour)
	if maxExpiry := a.sessionAbsoluteCap(session); newExpiry.After(maxExpiry) {
		newExpiry = maxExpiry
	}
	_ = a.Database.TouchServerSession(session.ID, now, newExpiry)
	session.LastSeenAt = now
	session.ExpiresAt = newExpiry
}

func (a *App) sessionAbsoluteCap(session *models.ServerSession) time.Time {
	return session.CreatedAt.Add(time.Duration(a.Config.Server.Oidc.SessionMaxDays) * 24 * time.Hour)
}

func (a *App) sessionPastAbsoluteCap(session *models.ServerSession, now time.Time) bool {
	return now.After(a.sessionAbsoluteCap(session))
}
