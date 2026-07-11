package extension_repo

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// isGitHubHost reports whether the host belongs to GitHub, i.e. a host that
// accepts a GitHub token for authenticated requests to private repositories.
func isGitHubHost(host string) bool {
	host = strings.ToLower(host)
	return host == "github.com" ||
		host == "api.github.com" ||
		host == "gist.github.com" ||
		strings.HasSuffix(host, ".githubusercontent.com")
}

// githubTokenFromEnv returns the GitHub token configured through the environment, if any.
func githubTokenFromEnv() string {
	if token := os.Getenv("SEANIME_GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GITHUB_TOKEN")
}

// newExtensionRequest builds a GET request for an extension-related resource
// (marketplace listing, manifest, payload) with support for private GitHub repositories.
//
// Credentials are resolved in order:
//  1. URL userinfo with a password (https://user:token@host/...) -> standard basic auth,
//     which GitHub accepts with a personal access token as the password.
//  2. URL userinfo with a single component (https://token@host/...) -> the component is
//     treated as a token and sent as a bearer token; the userinfo is stripped from the URL.
//  3. For GitHub hosts only, the SEANIME_GITHUB_TOKEN or GITHUB_TOKEN environment
//     variable -> sent as a bearer token.
//
// Requests to the GitHub contents API (api.github.com/repos/{owner}/{repo}/contents/...)
// are sent with the raw media type so the URL can be used directly as a marketplace or
// manifest URL for files in private repositories.
func newExtensionRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	bearerToken := ""
	if u.User != nil {
		if _, hasPassword := u.User.Password(); !hasPassword {
			bearerToken = u.User.Username()
			u.User = nil
		}
		// With a password present, keep the userinfo intact: net/http applies it
		// as basic auth when the request is sent.
	}

	if bearerToken == "" && u.User == nil && isGitHubHost(u.Hostname()) {
		bearerToken = githubTokenFromEnv()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	if strings.EqualFold(u.Hostname(), "api.github.com") && strings.Contains(u.Path, "/contents/") {
		req.Header.Set("Accept", "application/vnd.github.raw+json")
	}

	return req, nil
}
