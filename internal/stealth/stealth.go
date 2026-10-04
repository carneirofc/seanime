// Package stealth routes outbound HTTP requests through the cloak-backend gateway
// (seanime-extensions/cloak-backend), which replays them with a real browser's TLS and
// HTTP/2 fingerprint (curl_cffi impersonating Firefox) and solves anti-bot challenges in
// Camoufox, reusing the clearance cookies for later requests.
//
// The gateway is optional. Every helper here falls back to the wrapped transport when
// stealth is disabled for the request's category, so wiring a client through this package
// costs nothing until a user turns it on in config.toml:
//
//	[stealth]
//	enabled = true
//	url = "http://127.0.0.1:47541"
//	extensionMode = "fallback" # off | fallback | always
//	officialAPIs = false
//
// Configuration is process-wide and read on every request, so it can be swapped at runtime
// with Configure.
package stealth

import (
	"strings"
	"sync/atomic"
)

const DefaultURL = "http://127.0.0.1:47541"

// Mode decides how extension fetch() uses the gateway.
type Mode string

const (
	// ModeOff never routes extension requests through the gateway.
	ModeOff Mode = "off"
	// ModeFallback sends the request directly first and retries once through the gateway
	// when the response is an anti-bot challenge.
	ModeFallback Mode = "fallback"
	// ModeAlways sends every extension request through the gateway.
	ModeAlways Mode = "always"
)

// Category groups outbound clients so each group can be opted in separately.
type Category int

const (
	// CategoryExtension is goja fetch() from extensions and plugins.
	CategoryExtension Category = iota
	// CategoryOfficialAPI is first-party metadata APIs: AniList, MyAnimeList, animap,
	// Jikan, the filler list and the image proxy.
	CategoryOfficialAPI
)

type Config struct {
	Enabled bool
	// URL is the gateway base URL. Defaults to DefaultURL.
	URL string
	// Token is sent as X-Cloak-Token when the gateway requires one.
	Token         string
	ExtensionMode Mode
	// OfficialAPIs routes CategoryOfficialAPI clients through the gateway.
	OfficialAPIs bool
}

var current atomic.Pointer[Config]

// Configure replaces the process-wide configuration. Unknown modes fall back to
// ModeFallback and an empty URL to DefaultURL.
func Configure(cfg Config) {
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	switch cfg.ExtensionMode {
	case ModeOff, ModeFallback, ModeAlways:
	default:
		cfg.ExtensionMode = ModeFallback
	}
	current.Store(&cfg)
}

// Current returns the active configuration (the zero value when never configured).
func Current() Config {
	if cfg := current.Load(); cfg != nil {
		return *cfg
	}
	return Config{URL: DefaultURL, ExtensionMode: ModeOff}
}

// ParseMode normalises a user-supplied mode string.
func ParseMode(s string) (Mode, bool) {
	switch m := Mode(strings.ToLower(strings.TrimSpace(s))); m {
	case ModeOff, ModeFallback, ModeAlways:
		return m, true
	case "":
		return ModeFallback, true
	}
	return "", false
}

// Routed reports whether every request of this category goes through the gateway.
func Routed(cat Category) bool {
	cfg := Current()
	if !cfg.Enabled {
		return false
	}
	switch cat {
	case CategoryExtension:
		return cfg.ExtensionMode == ModeAlways
	case CategoryOfficialAPI:
		return cfg.OfficialAPIs
	}
	return false
}

// FallbackEnabled reports whether extension requests that hit a challenge are retried
// through the gateway.
func FallbackEnabled() bool {
	cfg := Current()
	return cfg.Enabled && cfg.ExtensionMode == ModeFallback
}

// Available reports whether a gateway is configured at all, regardless of category.
// Used for the per-request `stealth: true` override in extension fetch().
func Available() bool {
	return Current().Enabled
}
