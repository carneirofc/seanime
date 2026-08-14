package extension_repo

import (
	"context"
	"errors"
	"fmt"
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

// normalizeGitRepoPattern canonicalizes a user-supplied repository pattern to
// "host", "host/owner" or "host/owner/repo" form: lowercased, no scheme, no
// userinfo, no trailing slash, no ".git" suffix. It accepts URLs
// ("https://github.com/owner/repo"), bare "host/path" strings and scp-like git
// remotes ("git@github.com:owner/repo.git").
func normalizeGitRepoPattern(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("repository is empty")
	}

	// scp-like git remote: git@host:owner/repo(.git)
	if at := strings.Index(s, "@"); at != -1 && !strings.Contains(s, "://") {
		if colon := strings.Index(s[at:], ":"); colon != -1 {
			s = s[at+1:at+colon] + "/" + s[at+colon+1:]
		} else {
			s = s[at+1:]
		}
	}

	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("invalid repository %q: expected a host, host/owner or host/owner/repo", s)
	}

	host := strings.ToLower(u.Hostname())
	p := strings.Trim(u.Path, "/")
	p = strings.TrimSuffix(p, ".git")
	if p == "" {
		return host, nil
	}
	return host + "/" + strings.ToLower(p), nil
}

// gitRepoRequestKeys returns the canonical "host/path" keys that configured
// token patterns are matched against for the given request URL. Besides the
// URL's own host and path, GitHub content hosts are mapped back to the
// canonical "github.com/{owner}/{repo}" form so a single configured pattern
// covers github.com, raw.githubusercontent.com, codeload.github.com and the
// contents API.
func gitRepoRequestKeys(u *url.URL) []string {
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(strings.Trim(u.Path, "/"))

	keys := []string{host}
	if path != "" {
		keys[0] = host + "/" + path
	}

	segs := strings.Split(path, "/")
	switch {
	case (host == "raw.githubusercontent.com" || host == "codeload.github.com") && len(segs) >= 2:
		keys = append(keys, "github.com/"+segs[0]+"/"+segs[1])
	case host == "api.github.com" && len(segs) >= 3 && segs[0] == "repos":
		keys = append(keys, "github.com/"+segs[1]+"/"+segs[2])
	case (host == "gist.github.com" || host == "gist.githubusercontent.com") && len(segs) >= 1 && segs[0] != "":
		keys = append(keys, "github.com/"+segs[0])
	}

	return keys
}

// gitTokenForURL returns the token configured for the most specific repository
// pattern matching the request URL, or "" when none matches. A pattern matches
// a key when it equals the key or is a parent path of it.
func gitTokenForURL(u *url.URL, gitTokens map[string]string) string {
	if len(gitTokens) == 0 {
		return ""
	}

	keys := gitRepoRequestKeys(u)
	best, bestLen := "", -1
	for pattern, token := range gitTokens {
		for _, k := range keys {
			if k == pattern || strings.HasPrefix(k, pattern+"/") {
				if len(pattern) > bestLen {
					best, bestLen = token, len(pattern)
				}
			}
		}
	}
	return best
}

// newExtensionRequest builds a GET request for an extension-related resource
// (marketplace listing, manifest, payload) with support for private git repositories.
//
// Credentials are resolved in order:
//  1. URL userinfo with a password (https://user:token@host/...) -> standard basic auth,
//     which GitHub accepts with a personal access token as the password.
//  2. URL userinfo with a single component (https://token@host/...) -> the component is
//     treated as a token and sent as a bearer token; the userinfo is stripped from the URL.
//  3. A token configured for a repository pattern matching the URL (gitTokens)
//     -> sent as a bearer token.
//  4. For GitHub hosts only, the SEANIME_GITHUB_TOKEN or GITHUB_TOKEN environment
//     variable -> sent as a bearer token.
//
// Requests to the GitHub contents API (api.github.com/repos/{owner}/{repo}/contents/...)
// are sent with the raw media type so the URL can be used directly as a marketplace or
// manifest URL for files in private repositories.
func newExtensionRequest(ctx context.Context, rawURL string, gitTokens map[string]string) (*http.Request, error) {
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

	if bearerToken == "" && u.User == nil {
		bearerToken = gitTokenForURL(u, gitTokens)
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
