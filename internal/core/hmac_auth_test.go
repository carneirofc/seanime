package core

import "testing"

func TestMediaTokenSessionSubjectRoundTrip(t *testing.T) {
	// A subject that does not parse back is a subject the validator would reject,
	// which would break every media URL minted by a logged-in browser.
	subject := MediaTokenSessionSubject(42)
	if subject != "session:42" {
		t.Fatalf("MediaTokenSessionSubject(42) = %q, want session:42", subject)
	}
}

func TestUnboundMediaTokenSubjectIsLive(t *testing.T) {
	// Tokens minted without a request context — the URLs handed to external
	// players — carry no binding and must keep working.
	app := &App{}

	if !app.isMediaTokenSubjectLive("") {
		t.Fatal("an unbound media token was rejected")
	}
}

func TestUnparseableMediaTokenSubjectIsDenied(t *testing.T) {
	app := &App{}

	for _, subject := range []string{
		"garbage",
		"session:",
		"session:not-a-number",
		"session:-1",
		"user:42",
		// A binding we do not understand is a binding we cannot check, so it has to
		// fail closed rather than fall through to the unbound case.
		"session:1:2",
	} {
		if app.isMediaTokenSubjectLive(subject) {
			t.Errorf("subject %q was accepted", subject)
		}
	}
}

func TestSessionBoundMediaTokenIsDeniedOutsideOidcMode(t *testing.T) {
	// There are no login sessions without OIDC, so nothing can vouch for the
	// binding — including a token a client minted for itself with the password
	// hash it already holds.
	app := &App{}

	if app.isMediaTokenSubjectLive("session:1") {
		t.Fatal("a session-bound token was accepted with no OIDC session store")
	}
}

func TestZeroSessionIDIsDenied(t *testing.T) {
	// Zero is what an unset id looks like; it must never resolve to a session.
	app := &App{}

	if app.IsServerSessionLive(0) {
		t.Fatal("session id 0 was treated as live")
	}
}
