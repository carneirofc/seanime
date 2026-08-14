package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Outbound request filtering.
//
// Outbound targets here are user-content image/stream/marketplace URLs and whatever
// an extension decides to fetch. They are supposed to be public, so refusing to
// reach private, loopback, link-local and carrier-grade-NAT destinations costs
// nothing legitimate — and inside a cluster those destinations are the API server,
// the kubelet, the cloud metadata endpoint, and every unauthenticated neighbour.
//
// Enforcement happens at DIAL time, on the address actually being connected to.
// Validating the URL string up front cannot work on its own: between the check and
// the connection the name resolves a second time, so a hostname with a short TTL can
// answer publicly for the check and privately for the connection. Dial-time
// filtering also covers redirects for free, since every hop dials again.

// BlocksPrivateEgress reports whether outbound requests are confined to publicly
// routable destinations.
//
// A plain local install is often pointed at a NAS or a LAN source on purpose, so the
// confinement would break legitimate use there. Every server posture — hardened and
// strict, which is what OIDC forces — is confined, because in that posture the
// private addresses within reach are infrastructure, not media.
func BlocksPrivateEgress() bool {
	return IsHardened()
}

// ValidateOutboundUrl is a cheap pre-check that rejects obviously-internal targets
// before a connection is attempted, so callers get a clear error instead of a dial
// failure. It is NOT the enforcement point — HardenedTransport is. Because it can be
// defeated by a rebinding DNS answer, it must never be the only line of defence.
func ValidateOutboundUrl(rawURL string) error {
	if !BlocksPrivateEgress() {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("missing host")
	}

	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("private network access denied: host '%s' resolves to localhost", host)
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if isPrivateNetworkAddr(addr) {
			return fmt.Errorf("private network access denied: host '%s' is not publicly routable", host)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		// Fail closed. A name that will not resolve now is not a name worth
		// connecting to, and treating resolution failure as permission turns any
		// transient DNS error into a bypass.
		return fmt.Errorf("could not resolve host '%s': %w", host, err)
	}

	if len(addrs) == 0 {
		return fmt.Errorf("could not resolve host '%s'", host)
	}

	for _, addr := range addrs {
		if isPrivateNetworkAddr(addr) {
			return fmt.Errorf("private network access denied: host '%s' resolves to a private address", host)
		}
	}

	return nil
}

// cgnatPrefix is RFC 6598 carrier-grade NAT space, routinely used for cluster and
// VPN networks (Tailscale, EKS pods) and not publicly routable.
var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// uniqueLocalPrefix is the IPv6 equivalent of RFC 1918.
var uniqueLocalPrefix = netip.MustParsePrefix("fc00::/7")

func isPrivateNetworkAddr(addr netip.Addr) bool {
	addr = addr.Unmap()

	if !addr.IsValid() {
		return true
	}

	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() ||
		addr.IsInterfaceLocalMulticast() ||
		cgnatPrefix.Contains(addr) ||
		uniqueLocalPrefix.Contains(addr)
}

// ErrBlockedOutboundAddress is returned by the hardened dialer when a connection
// resolves to a non-public address.
type ErrBlockedOutboundAddress struct {
	Address string
}

func (e *ErrBlockedOutboundAddress) Error() string {
	return fmt.Sprintf("outbound connection to '%s' denied: not a publicly routable address", e.Address)
}

// hardenedDialControl is the last word on where an outbound connection may go. The
// Go runtime calls it with the concrete address it is about to connect to, after
// resolution, on every dial — including each redirect hop and each connection a
// pooled transport opens.
func hardenedDialControl(network string, address string, _ syscall.RawConn) error {
	if !BlocksPrivateEgress() {
		return nil
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		// The dialer hands us a literal address; anything else means we cannot tell
		// where this connection is going, so we do not allow it.
		return &ErrBlockedOutboundAddress{Address: address}
	}

	if isPrivateNetworkAddr(addr) {
		return &ErrBlockedOutboundAddress{Address: addr.String()}
	}

	return nil
}

// HardenedDialContext returns a DialContext that enforces the egress policy. Use it
// when a caller needs its own transport (custom TLS or HTTP/2 settings) but must
// still be confined — attaching this is what makes that transport safe.
func HardenedDialContext(timeout time.Duration, keepAlive time.Duration) func(ctx context.Context, network string, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: keepAlive,
		Control:   hardenedDialControl,
	}

	return dialer.DialContext
}

var (
	hardenedTransportOnce sync.Once
	hardenedTransport     *http.Transport
)

// HardenedTransport returns a shared http.Transport that refuses to connect to
// anything that is not publicly routable. Use it for every outbound request built
// from user, extension or remote-content input.
func HardenedTransport() *http.Transport {
	hardenedTransportOnce.Do(func() {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Control:   hardenedDialControl,
		}

		hardenedTransport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	})

	return hardenedTransport
}

// HardenedClient returns an http.Client using HardenedTransport.
func HardenedClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: HardenedTransport(),
		Timeout:   timeout,
	}
}
