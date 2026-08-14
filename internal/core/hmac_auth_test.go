package core

import (
	"testing"
	"time"

	"seanime/internal/util"
)

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

func TestWebsocketTicketRoundTrip(t *testing.T) {
	app := &App{MediaTokenSecret: "test-secret"}

	ticket, err := app.GenerateWebsocketTicket("")
	if err != nil {
		t.Fatalf("GenerateWebsocketTicket: %v", err)
	}

	if !app.ValidateWebsocketTicket(ticket) {
		t.Fatal("a freshly minted websocket ticket was rejected")
	}
}

func TestEmptyWebsocketTicketIsDenied(t *testing.T) {
	app := &App{MediaTokenSecret: "test-secret"}

	if app.ValidateWebsocketTicket("") {
		t.Fatal("an empty websocket ticket was accepted")
	}
}

func TestExpiredWebsocketTicketIsDenied(t *testing.T) {
	app := &App{MediaTokenSecret: "test-secret"}

	// Mint through an instance whose TTL has already elapsed, which is what a
	// ticket left sitting in a proxy log looks like by the time someone finds it.
	stale := util.NewHMACAuth("test-secret", -time.Minute)
	ticket, err := stale.GenerateToken(websocketTicketEndpoint)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if app.ValidateWebsocketTicket(ticket) {
		t.Fatal("an expired websocket ticket was accepted")
	}
}

func TestLongLivedWebsocketTicketIsCappedByServerTTL(t *testing.T) {
	// In password mode the client holds the signing secret, so it can claim any
	// "exp" it likes. The server's TTL has to be the authority, or the ticket would
	// be no better than handing out the credential.
	app := &App{MediaTokenSecret: "test-secret"}

	generous := util.NewHMACAuth("test-secret", 24*time.Hour)
	ticket, err := generous.GenerateTokenForSubject(websocketTicketEndpoint, "")
	if err != nil {
		t.Fatalf("GenerateTokenForSubject: %v", err)
	}

	// Still inside the server's own 60s window, so it is accepted for now...
	if !app.ValidateWebsocketTicket(ticket) {
		t.Fatal("a fresh ticket claiming a long life was rejected outright")
	}

	// ...but the claimed expiry is not what bounds it: validating through an
	// instance whose TTL has passed rejects it well before the claimed 24 hours.
	expired := util.NewHMACAuth("test-secret", -time.Minute)
	if _, err := expired.ValidateToken(ticket, websocketTicketEndpoint); err == nil {
		t.Fatal("the claimed expiry outlived the server-enforced TTL")
	}
}

func TestMediaTokenIsNotAcceptedAsWebsocketTicket(t *testing.T) {
	// The two are signed with the same secret, so only the endpoint claim keeps a
	// leaked media URL from being replayed into an event stream.
	app := &App{MediaTokenSecret: "test-secret"}

	mediaToken, err := app.GetServerHMACAuth().GenerateToken("/api/v1/image-proxy")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if app.ValidateWebsocketTicket(mediaToken) {
		t.Fatal("a media token was accepted as a websocket ticket")
	}
}

func TestWebsocketTicketIsNotAcceptedAtMediaEndpoint(t *testing.T) {
	app := &App{MediaTokenSecret: "test-secret"}

	ticket, err := app.GenerateWebsocketTicket("")
	if err != nil {
		t.Fatalf("GenerateWebsocketTicket: %v", err)
	}

	if app.ValidateMediaToken(ticket, "/api/v1/image-proxy") {
		t.Fatal("a websocket ticket was accepted at a media endpoint")
	}
}

func TestWildcardTicketIsDenied(t *testing.T) {
	// A client holding the signing secret could mint a "*" token, which
	// ValidateToken's endpoint matching would happily accept for any path.
	app := &App{MediaTokenSecret: "test-secret"}

	wildcard, err := app.GetWebsocketTicketAuth().GenerateToken("*")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if app.ValidateWebsocketTicket(wildcard) {
		t.Fatal("a wildcard token was accepted as a websocket ticket")
	}
}

func TestSessionBoundWebsocketTicketIsDeniedWithoutSession(t *testing.T) {
	app := &App{MediaTokenSecret: "test-secret"}

	ticket, err := app.GenerateWebsocketTicket(MediaTokenSessionSubject(1))
	if err != nil {
		t.Fatalf("GenerateWebsocketTicket: %v", err)
	}

	if app.ValidateWebsocketTicket(ticket) {
		t.Fatal("a session-bound ticket was accepted with no live session")
	}
}

func TestZeroSessionIDIsDenied(t *testing.T) {
	// Zero is what an unset id looks like; it must never resolve to a session.
	app := &App{}

	if app.IsServerSessionLive(0) {
		t.Fatal("session id 0 was treated as live")
	}
}
