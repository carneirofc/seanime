package stealth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// Wire shapes of the gateway's POST /proxy endpoint.
type proxyRequest struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	BodyB64         string            `json:"bodyB64,omitempty"`
	TimeoutMs       int               `json:"timeoutMs,omitempty"`
	FollowRedirects bool              `json:"followRedirects"`
}

type proxyResponse struct {
	Status     int         `json:"status"`
	StatusText string      `json:"statusText"`
	URL        string      `json:"url"`
	Headers    [][2]string `json:"headers"`
	BodyB64    string      `json:"bodyB64"`
	Challenged bool        `json:"challenged"`
}

type proxyError struct {
	Detail any `json:"detail"`
}

// ErrGateway wraps every failure to reach or use the gateway, so callers can tell a
// gateway outage apart from an upstream HTTP error status.
var ErrGateway = errors.New("stealth gateway")

// Headers the gateway sets itself, or that would describe the wrapped request rather
// than the one being replayed.
var droppedRequestHeaders = map[string]bool{
	"Host":              true,
	"Content-Length":    true,
	"Connection":        true,
	"Keep-Alive":        true,
	"Proxy-Connection":  true,
	"Transfer-Encoding": true,
	"Upgrade":           true,
	"Te":                true,
	"Accept-Encoding":   true, // curl_cffi negotiates and decodes compression itself
}

// gatewayClient talks to the gateway only. It deliberately does not use
// security.HardenedTransport: the gateway usually sits on loopback, its address comes
// from config.toml rather than from an extension, and it applies its own SSRF guard to
// the URLs it is asked to fetch.
var gatewayClient = &http.Client{
	Transport: &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 32,
		IdleConnTimeout:     90 * time.Second,
	},
	// Redirects of the target URL come back as 3xx responses; the gateway itself
	// never redirects.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// defaultGatewayTimeout bounds a request whose context has no deadline. A challenge
// solve in Camoufox can take tens of seconds.
const defaultGatewayTimeout = 90 * time.Second

// Do replays r through the gateway unconditionally and returns the upstream response.
// Redirects are not followed, so the caller's http.Client applies its own redirect
// policy and every hop goes through the gateway again.
func Do(r *http.Request) (*http.Response, error) {
	cfg := Current()
	if !cfg.Enabled {
		return nil, fmt.Errorf("%w: not enabled", ErrGateway)
	}

	payload := proxyRequest{
		Method:  r.Method,
		URL:     r.URL.String(),
		Headers: make(map[string]string, len(r.Header)),
	}
	if payload.Method == "" {
		payload.Method = http.MethodGet
	}
	for k, vs := range r.Header {
		ck := http.CanonicalHeaderKey(k)
		if droppedRequestHeaders[ck] || len(vs) == 0 {
			continue
		}
		sep := ", "
		if ck == "Cookie" {
			sep = "; "
		}
		payload.Headers[ck] = strings.Join(vs, sep)
	}
	if r.Body != nil && r.Body != http.NoBody {
		body, err := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("stealth: read request body: %w", err)
		}
		if len(body) > 0 {
			payload.BodyB64 = base64.StdEncoding.EncodeToString(body)
		}
	}
	timeout := defaultGatewayTimeout
	if deadline, ok := r.Context().Deadline(); ok {
		timeout = time.Until(deadline)
	}
	if timeout <= 0 {
		return nil, r.Context().Err()
	}
	payload.TimeoutMs = int(timeout.Milliseconds())

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	gwReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, cfg.URL+"/proxy", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGateway, err)
	}
	gwReq.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		gwReq.Header.Set("X-Cloak-Token", cfg.Token)
	}

	gwResp, err := gatewayClient.Do(gwReq)
	if err != nil {
		if ctxErr := r.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: %v", ErrGateway, err)
	}
	defer func() { _ = gwResp.Body.Close() }()
	gwBody, err := io.ReadAll(gwResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrGateway, err)
	}
	if gwResp.StatusCode != http.StatusOK {
		var pe proxyError
		_ = json.Unmarshal(gwBody, &pe)
		detail := fmt.Sprint(pe.Detail)
		if pe.Detail == nil {
			detail = strings.TrimSpace(string(gwBody))
		}
		return nil, fmt.Errorf("%w: %s: %s", ErrGateway, gwResp.Status, detail)
	}

	var pr proxyResponse
	if err := json.Unmarshal(gwBody, &pr); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrGateway, err)
	}
	body, err := base64.StdEncoding.DecodeString(pr.BodyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: decode body: %v", ErrGateway, err)
	}

	header := make(http.Header, len(pr.Headers))
	for _, kv := range pr.Headers {
		header.Add(kv[0], kv[1])
	}
	// The gateway hands back a decoded body; the original framing no longer applies.
	header.Del("Content-Encoding")
	header.Del("Transfer-Encoding")
	header.Set("Content-Length", strconv.Itoa(len(body)))

	status := pr.StatusText
	if status == "" {
		status = http.StatusText(pr.Status)
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", pr.Status, status),
		StatusCode:    pr.Status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}, nil
}

// Transport sends requests of Category through the gateway when that category is
// routed, and through Base otherwise. The decision is made per request, so a
// configuration change applies without rebuilding clients.
type Transport struct {
	Base     http.RoundTripper
	Category Category
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if Routed(t.Category) {
		return Do(r)
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

// gatewayOnly always uses the gateway; used for explicit per-request overrides.
type gatewayOnly struct{}

func (gatewayOnly) RoundTrip(r *http.Request) (*http.Response, error) { return Do(r) }

// GatewayTransport returns a RoundTripper that always replays through the gateway.
func GatewayTransport() http.RoundTripper { return gatewayOnly{} }

// Client returns a copy of base (or a fresh client when nil) whose transport honours the
// category's routing.
func Client(base *http.Client, cat Category) *http.Client {
	var c http.Client
	if base != nil {
		c = *base
	}
	c.Transport = &Transport{Base: c.Transport, Category: cat}
	return &c
}

// WrapReq makes a req client honour the category's routing. It returns c for chaining.
func WrapReq(c *req.Client, cat Category) *req.Client {
	c.GetTransport().WrapRoundTrip(func(rt http.RoundTripper) http.RoundTripper {
		return &Transport{Base: rt, Category: cat}
	})
	return c
}

// ForceReq makes a req client always replay through the gateway.
func ForceReq(c *req.Client) *req.Client {
	c.GetTransport().WrapRoundTrip(func(http.RoundTripper) http.RoundTripper {
		return gatewayOnly{}
	})
	return c
}
