package stealth

import (
	"bytes"
	"net/http"
	"strings"
)

// Body markers of anti-bot interstitials. Matched case-insensitively against the start
// of the body only, so a large page that merely mentions one of them is not mistaken
// for a challenge.
var challengeBodyMarkers = [][]byte{
	[]byte("just a moment..."),
	[]byte("checking your browser"),
	[]byte("cf-browser-verification"),
	[]byte("challenge-platform"),
	[]byte("attention required! | cloudflare"),
	[]byte("ddos-guard"),
	[]byte("verify you are human"),
}

const challengeSniffBytes = 16 * 1024

// IsChallenge reports whether a response is an anti-bot challenge page rather than the
// real resource. It only fires on the status codes challenges use, so an API's own 403
// (e.g. a JSON error behind Cloudflare) is not retried.
func IsChallenge(status int, header http.Header, body []byte) bool {
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable:
	default:
		return false
	}
	if header.Get("Cf-Mitigated") == "challenge" {
		return true
	}
	server := strings.ToLower(header.Get("Server"))
	if server != "cloudflare" && !strings.Contains(server, "ddos-guard") {
		return false
	}
	if len(body) > challengeSniffBytes {
		body = body[:challengeSniffBytes]
	}
	lower := bytes.ToLower(body)
	for _, m := range challengeBodyMarkers {
		if bytes.Contains(lower, m) {
			return true
		}
	}
	return false
}
