package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"
)

// forgeToken signs arbitrary claims with the given secret, standing in for a
// minter that is not this package — the browser, which holds the signing secret in
// password mode and therefore picks its own claims.
func forgeToken(t *testing.T, secret string, claims TokenClaims) string {
	t.Helper()

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	claimsB64 := base64URLEncode(claimsJSON)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(claimsB64))

	return claimsB64 + "." + base64URLEncode(mac.Sum(nil))
}

func TestValidateTokenAcceptsAServerMintedToken(t *testing.T) {
	auth := NewHMACAuth("secret", time.Hour)

	token, err := auth.GenerateToken("/api/v1/mediastream/")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
}

func TestValidateTokenCapsAClaimedLifetimeAtTheServerTTL(t *testing.T) {
	// The signing secret is the server password hash in password mode, so the
	// client can claim any expiry it likes. The server's TTL has to be the
	// authority, or a leaked media URL is good forever.
	const secret = "secret"
	auth := NewHMACAuth(secret, time.Hour)

	issued := time.Now().Add(-2 * time.Hour).Unix()
	token := forgeToken(t, secret, TokenClaims{
		Endpoint:  "/api/v1/mediastream/",
		IssuedAt:  issued,
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
	})

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err == nil {
		t.Fatal("a token claiming a year of life was accepted two hours past a one-hour TTL")
	}
}

func TestValidateTokenRejectsAFutureIssueTime(t *testing.T) {
	// Without this, back-dating the TTL window is as simple as claiming a future
	// "iat" — the cap would slide forward with it.
	const secret = "secret"
	auth := NewHMACAuth(secret, time.Hour)

	future := time.Now().Add(48 * time.Hour).Unix()
	token := forgeToken(t, secret, TokenClaims{
		Endpoint:  "/api/v1/mediastream/",
		IssuedAt:  future,
		ExpiresAt: future + 3600,
	})

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err == nil {
		t.Fatal("a token issued 48 hours in the future was accepted")
	}
}

func TestValidateTokenToleratesModestClockSkew(t *testing.T) {
	// Tokens are minted in the browser too; a slightly fast clock must not lock a
	// user out of playback.
	const secret = "secret"
	auth := NewHMACAuth(secret, time.Hour)

	skewed := time.Now().Add(time.Minute).Unix()
	token := forgeToken(t, secret, TokenClaims{
		Endpoint:  "/api/v1/mediastream/",
		IssuedAt:  skewed,
		ExpiresAt: skewed + 3600,
	})

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err != nil {
		t.Fatalf("a token one minute ahead of the server was rejected: %v", err)
	}
}

func TestValidateTokenRejectsAMissingIssueTime(t *testing.T) {
	// "iat" is load-bearing now: it is what the TTL cap is measured from.
	const secret = "secret"
	auth := NewHMACAuth(secret, time.Hour)

	token := forgeToken(t, secret, TokenClaims{
		Endpoint:  "/api/v1/mediastream/",
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
	})

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err == nil {
		t.Fatal("a token with no issue time was accepted")
	}
}

func TestValidateTokenStillHonoursAShorterClaimedExpiry(t *testing.T) {
	// The cap is a ceiling, not a replacement: a token that says it expired stays
	// expired even though the TTL would still allow it.
	const secret = "secret"
	auth := NewHMACAuth(secret, 24*time.Hour)

	issued := time.Now().Add(-time.Hour).Unix()
	token := forgeToken(t, secret, TokenClaims{
		Endpoint:  "/api/v1/mediastream/",
		IssuedAt:  issued,
		ExpiresAt: issued + 60,
	})

	if _, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err == nil {
		t.Fatal("a token past its own expiry was accepted")
	}
}

func TestGenerateTokenForSubjectCarriesTheBinding(t *testing.T) {
	auth := NewHMACAuth("secret", time.Hour)

	token, err := auth.GenerateTokenForSubject("/api/v1/mediastream/", "session:42")
	if err != nil {
		t.Fatalf("GenerateTokenForSubject: %v", err)
	}

	claims, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4")
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Subject != "session:42" {
		t.Fatalf("subject = %q, want session:42", claims.Subject)
	}
}

func TestGenerateTokenLeavesTheSubjectUnbound(t *testing.T) {
	// Tokens minted without a request context must stay unbound, or the media URLs
	// handed to external players would have nothing to bind to and always fail.
	auth := NewHMACAuth("secret", time.Hour)

	token, err := auth.GenerateToken("/api/v1/mediastream/")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := auth.ValidateToken(token, "/api/v1/mediastream/file.mp4")
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Subject != "" {
		t.Fatalf("subject = %q, want it empty", claims.Subject)
	}
}

func TestValidateTokenRejectsAnotherEndpoint(t *testing.T) {
	// Path scoping is what keeps a leaked media URL from being a general-purpose
	// API credential; the lifetime work must not have loosened it.
	auth := NewHMACAuth("secret", time.Hour)

	token, err := auth.GenerateToken("/api/v1/mediastream/")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := auth.ValidateToken(token, "/api/v1/settings"); err == nil {
		t.Fatal("a mediastream token was accepted on /api/v1/settings")
	}
}

func TestValidateTokenRejectsAForeignSignature(t *testing.T) {
	issuer := NewHMACAuth("secret", time.Hour)
	other := NewHMACAuth("a-different-secret", time.Hour)

	token, err := issuer.GenerateToken("/api/v1/mediastream/")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if _, err := other.ValidateToken(token, "/api/v1/mediastream/file.mp4"); err == nil {
		t.Fatal("a token signed with another secret was accepted")
	}
}
