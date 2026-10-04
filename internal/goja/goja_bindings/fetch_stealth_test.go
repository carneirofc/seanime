package goja_bindings

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"seanime/internal/stealth"
	"sync/atomic"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

// fakeStealthGateway answers POST /proxy with a fixed body and counts calls.
func fakeStealthGateway(t *testing.T, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var in struct{ URL string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  200,
			"url":     in.URL,
			"headers": [][2]string{{"Content-Type", "text/plain"}},
			"bodyB64": base64.StdEncoding.EncodeToString([]byte(body)),
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestFetchStealthModes(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/challenge" {
			w.Header().Set("Server", "cloudflare")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<title>Just a moment...</title>"))
			return
		}
		_, _ = w.Write([]byte("direct"))
	}))
	defer origin.Close()
	gateway, calls := fakeStealthGateway(t, "via gateway")
	t.Cleanup(func() { stealth.Configure(stealth.Config{}) })

	tests := []struct {
		name      string
		mode      stealth.Mode
		enabled   bool
		path      string
		opts      string
		wantBody  string
		wantCalls int32
	}{
		{name: "disabled goes direct", enabled: false, mode: stealth.ModeAlways, path: "/ok", wantBody: "direct"},
		{name: "always routes", enabled: true, mode: stealth.ModeAlways, path: "/ok", wantBody: "via gateway", wantCalls: 1},
		{name: "fallback passes clean responses", enabled: true, mode: stealth.ModeFallback, path: "/ok", wantBody: "direct"},
		{name: "fallback retries challenges", enabled: true, mode: stealth.ModeFallback, path: "/challenge", wantBody: "via gateway", wantCalls: 1},
		{name: "off leaves challenges", enabled: true, mode: stealth.ModeOff, path: "/challenge", wantBody: "<title>Just a moment...</title>"},
		{name: "per-call opt in", enabled: true, mode: stealth.ModeOff, path: "/ok", opts: `{stealth: true}`, wantBody: "via gateway", wantCalls: 1},
		{name: "per-call opt out", enabled: true, mode: stealth.ModeAlways, path: "/challenge", opts: `{stealth: false}`, wantBody: "<title>Just a moment...</title>"},
		{name: "opt in without gateway", enabled: false, mode: stealth.ModeOff, path: "/ok", opts: `{stealth: true}`, wantBody: "direct"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stealth.Configure(stealth.Config{Enabled: tt.enabled, URL: gateway.URL, ExtensionMode: tt.mode})
			calls.Store(0)

			vm := goja.New()
			fetch := BindFetch("", vm, []string{"*"})
			defer fetch.Close()

			opts := tt.opts
			if opts == "" {
				opts = "{}"
			}
			val, err := vm.RunString(fmt.Sprintf(`fetch(%q, %s)`, origin.URL+tt.path, opts))
			require.NoError(t, err)
			promise := requirePromise(t, val)
			waitForPromiseState(t, promise, goja.PromiseStateFulfilled)

			text, ok := goja.AssertFunction(promise.Result().ToObject(vm).Get("text"))
			require.True(t, ok)
			got, err := text(goja.Undefined())
			require.NoError(t, err)
			require.Equal(t, tt.wantBody, got.String())
			require.Equal(t, tt.wantCalls, calls.Load())
		})
	}
}
