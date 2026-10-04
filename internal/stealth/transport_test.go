package stealth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGateway struct {
	t        *testing.T
	got      []proxyRequest
	tokens   []string
	respond  func(p proxyRequest) (int, any)
	srv      *httptest.Server
	blockFor time.Duration
}

func newFakeGateway(t *testing.T, respond func(p proxyRequest) (int, any)) *fakeGateway {
	g := &fakeGateway{t: t, respond: respond}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if g.blockFor > 0 {
			select {
			case <-time.After(g.blockFor):
			case <-r.Context().Done():
				return
			}
		}
		var p proxyRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&p))
		g.got = append(g.got, p)
		g.tokens = append(g.tokens, r.Header.Get("X-Cloak-Token"))
		status, body := g.respond(p)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func configureFor(t *testing.T, cfg Config) {
	prev := current.Load()
	Configure(cfg)
	t.Cleanup(func() { current.Store(prev) })
}

func okResponse(p proxyRequest, body []byte, headers [][2]string) (int, any) {
	return http.StatusOK, proxyResponse{
		Status:  200,
		URL:     p.URL,
		Headers: headers,
		BodyB64: base64.StdEncoding.EncodeToString(body),
	}
}

func TestDo_RoundTripsBinaryBodyAndHeaders(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x00, 0xff, 0x10}
	g := newFakeGateway(t, func(p proxyRequest) (int, any) {
		return okResponse(p, png, [][2]string{
			{"Content-Type", "image/png"},
			{"Set-Cookie", "a=1"},
			{"Set-Cookie", "b=2"},
			{"Content-Encoding", "br"},
		})
	})
	configureFor(t, Config{Enabled: true, URL: g.srv.URL + "/", Token: "secret"})

	r, err := http.NewRequest(http.MethodPost, "https://example.com/x?q=1", strings.NewReader("payload"))
	require.NoError(t, err)
	r.Header.Add("Accept", "a")
	r.Header.Add("Accept", "b")
	r.Header.Add("Cookie", "x=1")
	r.Header.Add("Cookie", "y=2")
	r.Header.Set("Accept-Encoding", "gzip")

	resp, err := Do(r)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, png, body)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, []string{"a=1", "b=2"}, resp.Header.Values("Set-Cookie"))
	assert.Empty(t, resp.Header.Get("Content-Encoding"), "the gateway returns a decoded body")
	assert.Equal(t, int64(len(png)), resp.ContentLength)

	require.Len(t, g.got, 1)
	sent := g.got[0]
	assert.Equal(t, "POST", sent.Method)
	assert.Equal(t, "https://example.com/x?q=1", sent.URL)
	assert.Equal(t, "a, b", sent.Headers["Accept"])
	assert.Equal(t, "x=1; y=2", sent.Headers["Cookie"])
	assert.NotContains(t, sent.Headers, "Accept-Encoding")
	assert.False(t, sent.FollowRedirects)
	decoded, _ := base64.StdEncoding.DecodeString(sent.BodyB64)
	assert.Equal(t, "payload", string(decoded))
	assert.Equal(t, "secret", g.tokens[0])
}

func TestDo_GatewayErrorIsWrapped(t *testing.T) {
	g := newFakeGateway(t, func(proxyRequest) (int, any) {
		return http.StatusBadGateway, map[string]string{"detail": "upstream reset"}
	})
	configureFor(t, Config{Enabled: true, URL: g.srv.URL})

	r, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, err := Do(r)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrGateway))
	assert.Contains(t, err.Error(), "upstream reset")
}

func TestDo_UnreachableGateway(t *testing.T) {
	configureFor(t, Config{Enabled: true, URL: "http://127.0.0.1:1"})
	r, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, err := Do(r)
	assert.True(t, errors.Is(err, ErrGateway))
}

func TestDo_HonoursCancellation(t *testing.T) {
	g := newFakeGateway(t, func(p proxyRequest) (int, any) { return okResponse(p, nil, nil) })
	g.blockFor = time.Second
	configureFor(t, Config{Enabled: true, URL: g.srv.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	start := time.Now()
	_, err := Do(r)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestTransport_RoutesOnlyEnabledCategory(t *testing.T) {
	g := newFakeGateway(t, func(p proxyRequest) (int, any) { return okResponse(p, []byte("via gateway"), nil) })
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "direct")
	}))
	defer direct.Close()

	get := func(c *http.Client) string {
		resp, err := c.Get(direct.URL)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	official := Client(nil, CategoryOfficialAPI)

	configureFor(t, Config{Enabled: true, URL: g.srv.URL, OfficialAPIs: false})
	assert.Equal(t, "direct", get(official))

	Configure(Config{Enabled: true, URL: g.srv.URL, OfficialAPIs: true})
	assert.Equal(t, "via gateway", get(official))

	Configure(Config{Enabled: false, URL: g.srv.URL, OfficialAPIs: true})
	assert.Equal(t, "direct", get(official))
}

func TestWrapReq_RoutesThroughGateway(t *testing.T) {
	g := newFakeGateway(t, func(p proxyRequest) (int, any) {
		return okResponse(p, []byte(`{"ok":true}`), [][2]string{{"Content-Type", "application/json"}})
	})
	configureFor(t, Config{Enabled: true, URL: g.srv.URL, OfficialAPIs: true})

	c := WrapReq(req.C(), CategoryOfficialAPI)
	var out struct{ Ok bool }
	resp, err := c.R().SetSuccessResult(&out).Get("https://api.example.com/v1")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, out.Ok)
	require.Len(t, g.got, 1)
	assert.Equal(t, "https://api.example.com/v1", g.got[0].URL)
}

func TestClient_FollowsRedirectsHopByHop(t *testing.T) {
	g := newFakeGateway(t, func(p proxyRequest) (int, any) {
		if strings.HasSuffix(p.URL, "/start") {
			return http.StatusOK, proxyResponse{Status: 302, URL: p.URL, Headers: [][2]string{{"Location", "/end"}}}
		}
		return okResponse(p, []byte("done"), nil)
	})
	configureFor(t, Config{Enabled: true, URL: g.srv.URL, OfficialAPIs: true})

	resp, err := Client(nil, CategoryOfficialAPI).Get("https://example.com/start")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "done", string(b))
	assert.Equal(t, "https://example.com/end", resp.Request.URL.String())
	require.Len(t, g.got, 2)
}

func TestConfigureDefaults(t *testing.T) {
	configureFor(t, Config{Enabled: true, ExtensionMode: "bogus"})
	cfg := Current()
	assert.Equal(t, DefaultURL, cfg.URL)
	assert.Equal(t, ModeFallback, cfg.ExtensionMode)
	assert.True(t, FallbackEnabled())
	assert.False(t, Routed(CategoryExtension))

	Configure(Config{Enabled: true, ExtensionMode: ModeAlways})
	assert.True(t, Routed(CategoryExtension))
	assert.False(t, FallbackEnabled())
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"": ModeFallback, "Always": ModeAlways, " off ": ModeOff} {
		got, ok := ParseMode(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := ParseMode("sometimes")
	assert.False(t, ok)
}

func TestIsChallenge(t *testing.T) {
	cf := http.Header{"Server": {"cloudflare"}}
	assert.True(t, IsChallenge(403, http.Header{"Cf-Mitigated": {"challenge"}}, nil))
	assert.True(t, IsChallenge(503, cf, []byte("<title>Just a moment...</title>")))
	assert.True(t, IsChallenge(403, http.Header{"Server": {"ddos-guard"}}, []byte("DDoS-Guard check")))
	assert.False(t, IsChallenge(403, cf, []byte(`{"error":"forbidden"}`)), "API 403 behind Cloudflare")
	assert.False(t, IsChallenge(200, http.Header{"Cf-Mitigated": {"challenge"}}, nil))
	assert.False(t, IsChallenge(503, http.Header{"Server": {"nginx"}}, []byte("Just a moment...")))
}
