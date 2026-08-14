package security

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestValidateOutboundURL(t *testing.T) {
	t.Cleanup(func() {
		SetSecureMode("")
	})

	t.Run("allows localhost outside strict mode", func(t *testing.T) {
		SetSecureMode("")
		if err := ValidateOutboundUrl("http://127.0.0.1:8080"); err != nil {
			t.Fatalf("expected localhost to be allowed outside strict mode: %v", err)
		}
	})

	t.Run("blocks localhost in strict mode", func(t *testing.T) {
		SetSecureMode(SecureModeStrict)
		if err := ValidateOutboundUrl("http://localhost:8080"); err == nil {
			t.Fatal("expected localhost to be blocked in strict mode")
		}
	})

	t.Run("blocks loopback ip in strict mode", func(t *testing.T) {
		SetSecureMode(SecureModeStrict)
		if err := ValidateOutboundUrl("http://127.0.0.1:8080"); err == nil {
			t.Fatal("expected loopback ip to be blocked in strict mode")
		}
	})

	t.Run("blocks private ip in strict mode", func(t *testing.T) {
		SetSecureMode(SecureModeStrict)
		if err := ValidateOutboundUrl("http://192.168.1.10:8080"); err == nil {
			t.Fatal("expected private ip to be blocked in strict mode")
		}
	})

	t.Run("allows public ip in strict mode", func(t *testing.T) {
		SetSecureMode(SecureModeStrict)
		if err := ValidateOutboundUrl("https://1.1.1.1"); err != nil {
			t.Fatalf("expected public ip to be allowed in strict mode: %v", err)
		}
	})

	t.Run("blocks loopback ip in hardened mode", func(t *testing.T) {
		SetSecureMode(SecureModeHardened)
		if err := ValidateOutboundUrl("http://127.0.0.1:8080"); err == nil {
			t.Fatal("expected loopback ip to be blocked in hardened mode")
		}
	})

	t.Run("blocks private ip in hardened mode", func(t *testing.T) {
		SetSecureMode(SecureModeHardened)
		if err := ValidateOutboundUrl("http://192.168.1.10:8080"); err == nil {
			t.Fatal("expected private ip to be blocked in hardened mode")
		}
	})

	t.Run("allows public ip in hardened mode", func(t *testing.T) {
		SetSecureMode(SecureModeHardened)
		if err := ValidateOutboundUrl("https://1.1.1.1"); err != nil {
			t.Fatalf("expected public ip to be allowed in hardened mode: %v", err)
		}
	})

	t.Run("allows loopback in lax mode", func(t *testing.T) {
		SetSecureMode(SecureModeLax)
		if err := ValidateOutboundUrl("http://127.0.0.1:8080"); err != nil {
			t.Fatalf("expected loopback to be allowed in lax mode: %v", err)
		}
	})
}

func TestValidateOutboundURLFailsClosed(t *testing.T) {
	t.Cleanup(func() {
		SetSecureMode("")
	})

	SetSecureMode(SecureModeStrict)

	// A name that cannot be resolved used to be allowed through, which turned any
	// transient DNS failure — or a name that only resolves inside the cluster's
	// resolver — into a bypass of the whole check.
	if err := ValidateOutboundUrl("http://this-host-does-not-exist.invalid/asset.png"); err == nil {
		t.Fatal("expected an unresolvable host to be refused, not allowed")
	}
}

func TestPrivateNetworkRanges(t *testing.T) {
	blocked := []string{
		"127.0.0.1",       // loopback
		"10.0.0.1",        // RFC 1918
		"192.168.1.1",     // RFC 1918
		"172.16.0.1",      // RFC 1918
		"169.254.169.254", // link-local: cloud metadata
		"100.64.0.1",      // RFC 6598 carrier-grade NAT
		"0.0.0.0",         // unspecified
		"::1",             // IPv6 loopback
		"fc00::1",         // IPv6 unique-local
		"fe80::1",         // IPv6 link-local
		"ff02::1",         // IPv6 multicast
	}

	allowed := []string{
		"1.1.1.1",
		"8.8.8.8",
		"203.0.113.10",
		"2606:4700:4700::1111",
	}

	for _, raw := range blocked {
		addr := netip.MustParseAddr(raw)
		if !isPrivateNetworkAddr(addr) {
			t.Errorf("expected %s to be treated as a non-public address", raw)
		}
	}

	for _, raw := range allowed {
		addr := netip.MustParseAddr(raw)
		if isPrivateNetworkAddr(addr) {
			t.Errorf("expected %s to be treated as publicly routable", raw)
		}
	}
}

func TestHardenedDialControlBlocksResolvedAddress(t *testing.T) {
	t.Cleanup(func() {
		SetSecureMode("")
	})

	SetSecureMode(SecureModeStrict)

	// This is the check that survives DNS rebinding: whatever the URL said, the
	// address actually being connected to is what gets inspected.
	if err := hardenedDialControl("tcp4", "169.254.169.254:80", nil); err == nil {
		t.Fatal("expected the metadata endpoint to be refused at dial time")
	}

	if err := hardenedDialControl("tcp4", "10.0.0.5:8080", nil); err == nil {
		t.Fatal("expected a private address to be refused at dial time")
	}

	if err := hardenedDialControl("tcp4", "1.1.1.1:443", nil); err != nil {
		t.Fatalf("expected a public address to be allowed at dial time: %v", err)
	}

	SetSecureMode("")
	if err := hardenedDialControl("tcp4", "10.0.0.5:8080", nil); err != nil {
		t.Fatalf("expected a local install to keep reaching its LAN: %v", err)
	}
}

func TestHardenedTransportBlocksRedirectToPrivateAddress(t *testing.T) {
	t.Cleanup(func() {
		SetSecureMode("")
	})

	// A redirect is a fresh dial, so the dialer sees it even though the original URL
	// was public.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(redirector.Close)

	// Each case gets its own transport: the shared one pools connections, and a
	// connection opened under one policy would otherwise be reused under another.
	newClient := func() *http.Client {
		return &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext:       HardenedDialContext(5*time.Second, 0),
				DisableKeepAlives: true,
			},
		}
	}

	SetSecureMode("")
	resp, err := newClient().Get(redirector.URL)
	if err != nil {
		t.Fatalf("expected the redirect to be followed for a local install: %v", err)
	}
	_ = resp.Body.Close()

	SetSecureMode(SecureModeStrict)
	if _, err := newClient().Get(redirector.URL); err == nil {
		t.Fatal("expected a loopback destination to be refused")
	}
}
