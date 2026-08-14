package core

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

// Middlewares that harden the server while OIDC login is active:
//   - securityHeadersMiddleware sets browser security headers on every response.
//   - webBundleGateMiddleware keeps the SPA bundle and static assets away from
//     unauthenticated clients; they only ever receive the minimal login shell.

func securityHeadersMiddleware(app *App) echo.MiddlewareFunc {
	hstsEligible := strings.HasPrefix(app.Config.Server.ExternalURL, "https://")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			headers := c.Response().Header()
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("Content-Security-Policy", "frame-ancestors 'none'")
			headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			// Browsers ignore HSTS on plaintext responses, so this is safe to set
			// whenever the canonical URL is https (TLS usually terminates upstream)
			if hstsEligible {
				headers.Set("Strict-Transport-Security", "max-age=15552000")
			}

			return next(c)
		}
	}
}

// isDocumentRequest reports whether the request looks like a browser navigation.
func isDocumentRequest(req *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(req.Header.Get("Sec-Fetch-Dest")), "document") {
		return true
	}
	return strings.Contains(req.Header.Get("Accept"), "text/html")
}

// gatedStaticPathPrefixes are directories served by e.Static that must not be
// readable without a session (or a valid server-minted HMAC query token).
var gatedStaticPathPrefixes = []string{"/assets", "/manga-downloads", "/offline-assets"}

func isGatedStaticPath(p string) bool {
	for _, prefix := range gatedStaticPathPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// webBundleGateMiddleware serves, to unauthenticated clients, only the login
// shell (an isolated bundle under shell/ that contains no app code); every
// other document, chunk or static asset requires a valid session. API and
// websocket paths pass through untouched — their auth lives in the handlers.
func webBundleGateMiddleware(app *App, shellFS fs.FS) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			p := req.URL.Path

			if strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/events") {
				return next(c)
			}

			if _, ok := app.ResolveServerSession(req); ok {
				if p == "/login" || p == "/login/" {
					// Already signed in
					return c.Redirect(http.StatusFound, "/")
				}
				// The app bundle is per-session content now; keep shared caches out
				c.Response().Header().Set("Cache-Control", "private")
				return next(c)
			}

			// Unauthenticated: the login shell is the only reachable content
			if p == "/login" || p == "/login/" {
				return serveShellFile(c, shellFS, "index.html")
			}
			if strings.HasPrefix(p, "/shell/") {
				return serveShellFile(c, shellFS, strings.TrimPrefix(p, "/shell/"))
			}

			if isGatedStaticPath(p) {
				// Server-minted media URLs carry an HMAC query token (external players)
				if token := req.URL.Query().Get("token"); token != "" {
					if app.ValidateMediaToken(token, p) {
						return next(c)
					}
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			if isDocumentRequest(req) {
				return c.Redirect(http.StatusFound, "/login")
			}

			return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		}
	}
}

// serveShellFile serves a file from the login-shell bundle, falling back to a
// minimal built-in login page when the web build does not include a shell.
func serveShellFile(c echo.Context, shellFS fs.FS, name string) error {
	name = path.Clean(strings.TrimPrefix(name, "/"))
	if name == "" || name == "." {
		name = "index.html"
	}

	if shellFS != nil {
		if data, err := fs.ReadFile(shellFS, name); err == nil {
			contentType := mime.TypeByExtension(path.Ext(name))
			if contentType == "" {
				contentType = http.DetectContentType(data)
			}
			return c.Blob(http.StatusOK, contentType, data)
		}
	}

	if strings.HasSuffix(name, ".html") || name == "index.html" {
		return c.HTML(http.StatusOK, fallbackLoginShellHTML)
	}

	return echo.NewHTTPError(http.StatusNotFound)
}

// fallbackLoginShellHTML keeps the login gate functional when the embedded web
// build does not ship a shell bundle (e.g. API-only builds).
const fallbackLoginShellHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Seanime — Sign in</title>
<style>
body{margin:0;display:flex;align-items:center;justify-content:center;min-height:100vh;background:#0c0c0f;color:#eee;font-family:system-ui,sans-serif}
main{text-align:center}
a{display:inline-block;margin-top:1.5rem;padding:.75rem 2rem;border-radius:.5rem;background:#6152df;color:#fff;text-decoration:none;font-weight:600}
p{color:#f87171;margin-top:1rem}
</style>
</head>
<body>
<main>
<h1>Seanime</h1>
<a href="/api/v1/auth/oidc/login">Sign in</a>
<p id="err" hidden></p>
<script>
var e=new URLSearchParams(location.search).get("error");
if(e){var m={idp_denied:"The identity provider denied the request.",not_allowed:"This account is not allowed on this server.",oidc_failed:"Sign-in failed. Please try again."};
var el=document.getElementById("err");el.textContent=m[e]||"Sign-in failed.";el.hidden=false}
</script>
</main>
</body>
</html>
`
