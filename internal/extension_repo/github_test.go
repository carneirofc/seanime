package extension_repo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewExtensionRequestTokenOnlyUserinfo(t *testing.T) {
	req, err := newExtensionRequest(context.Background(), "https://ghp_secret@raw.githubusercontent.com/owner/private/main/marketplace.json", nil)
	require.NoError(t, err)

	require.Equal(t, "Bearer ghp_secret", req.Header.Get("Authorization"))
	require.Nil(t, req.URL.User, "userinfo should be stripped from the request URL")
	require.Equal(t, "https://raw.githubusercontent.com/owner/private/main/marketplace.json", req.URL.String())
}

func TestNewExtensionRequestBasicAuthUserinfoIsPreserved(t *testing.T) {
	req, err := newExtensionRequest(context.Background(), "https://user:ghp_secret@raw.githubusercontent.com/owner/private/main/marketplace.json", nil)
	require.NoError(t, err)

	// net/http applies basic auth from the URL userinfo when sending the request
	require.Empty(t, req.Header.Get("Authorization"))
	require.NotNil(t, req.URL.User)
	password, hasPassword := req.URL.User.Password()
	require.True(t, hasPassword)
	require.Equal(t, "ghp_secret", password)
}

func TestNewExtensionRequestEnvTokenForGitHubHost(t *testing.T) {
	t.Setenv("SEANIME_GITHUB_TOKEN", "ghp_env_token")

	req, err := newExtensionRequest(context.Background(), "https://raw.githubusercontent.com/owner/private/main/marketplace.json", nil)
	require.NoError(t, err)
	require.Equal(t, "Bearer ghp_env_token", req.Header.Get("Authorization"))
}

func TestNewExtensionRequestEnvTokenFallback(t *testing.T) {
	t.Setenv("SEANIME_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")

	req, err := newExtensionRequest(context.Background(), "https://api.github.com/repos/owner/private/contents/marketplace.json", nil)
	require.NoError(t, err)
	require.Equal(t, "Bearer ghp_fallback", req.Header.Get("Authorization"))
}

func TestNewExtensionRequestEnvTokenNotSentToOtherHosts(t *testing.T) {
	t.Setenv("SEANIME_GITHUB_TOKEN", "ghp_env_token")

	req, err := newExtensionRequest(context.Background(), "https://example.com/marketplace.json", nil)
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestNewExtensionRequestGitHubContentsAPIAcceptHeader(t *testing.T) {
	req, err := newExtensionRequest(context.Background(), "https://api.github.com/repos/owner/private/contents/marketplace.json?ref=main", nil)
	require.NoError(t, err)
	require.Equal(t, "application/vnd.github.raw+json", req.Header.Get("Accept"))

	req, err = newExtensionRequest(context.Background(), "https://raw.githubusercontent.com/owner/private/main/marketplace.json", nil)
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("Accept"))
}

func TestGetMarketplaceExtensionsWithToken(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"dummy-provider","manifestURI":"https://example.com/dummy.json"}]`))
	}))
	defer server.Close()

	// Without a token the private endpoint responds 404
	_, err := repo.GetMarketplaceExtensions(server.URL)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 404")

	// With the token embedded in the URL userinfo the request is authenticated
	authedURL := strings.Replace(server.URL, "http://", "http://secret-token@", 1)
	extensions, err := repo.GetMarketplaceExtensions(authedURL)
	require.NoError(t, err)
	require.Len(t, extensions, 1)
	require.Equal(t, "dummy-provider", extensions[0].ID)
}

func TestGetMarketplaceExtensionsWithBasicAuth(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, ok := r.BasicAuth()
		if !ok || password != "secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"id":"dummy-provider","manifestURI":"https://example.com/dummy.json"}]`))
	}))
	defer server.Close()

	authedURL := strings.Replace(server.URL, "http://", "http://user:secret-token@", 1)
	extensions, err := repo.GetMarketplaceExtensions(authedURL)
	require.NoError(t, err)
	require.Len(t, extensions, 1)
}

func TestFetchExternalExtensionDataWithToken(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)
	ext := testExternalExtension()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprintf(w, `{
			"id": %q, "name": %q, "version": %q, "manifestURI": %q,
			"language": %q, "type": %q, "description": %q, "author": %q,
			"lang": %q, "payload": %q
		}`, ext.ID, ext.Name, ext.Version, ext.ManifestURI,
			ext.Language, ext.Type, ext.Description, ext.Author,
			ext.Lang, ext.Payload)
	}))
	defer server.Close()

	authedURL := strings.Replace(server.URL, "http://", "http://secret-token@", 1) + "/manifest.json"
	fetched, err := repo.FetchExternalExtensionData(authedURL)
	require.NoError(t, err)
	require.Equal(t, ext.ID, fetched.ID)
}

func TestNormalizeGitRepoPattern(t *testing.T) {
	cases := map[string]string{
		"https://github.com/Owner/Repo":  "github.com/owner/repo",
		"github.com/owner/repo":          "github.com/owner/repo",
		"github.com/owner/repo.git":      "github.com/owner/repo",
		"git@github.com:owner/repo.git":  "github.com/owner/repo",
		"GitHub.com/Owner":               "github.com/owner",
		"https://gitea.example.com":      "gitea.example.com",
		"https://gitea.example.com/o/r/": "gitea.example.com/o/r",
	}
	for in, want := range cases {
		got, err := normalizeGitRepoPattern(in)
		require.NoError(t, err, in)
		require.Equal(t, want, got, in)
	}

	_, err := normalizeGitRepoPattern("")
	require.Error(t, err)
	_, err = normalizeGitRepoPattern("   ")
	require.Error(t, err)
}

func TestGitTokenForURLMatchesGitHubContentHosts(t *testing.T) {
	tokens := map[string]string{"github.com/owner/private": "tok_repo"}

	for _, rawURL := range []string{
		"https://github.com/owner/private/raw/main/manifest.json",
		"https://raw.githubusercontent.com/owner/private/main/manifest.json",
		"https://api.github.com/repos/owner/private/contents/manifest.json",
		"https://codeload.github.com/owner/private/tar.gz/main",
	} {
		req, err := newExtensionRequest(context.Background(), rawURL, tokens)
		require.NoError(t, err, rawURL)
		require.Equal(t, "Bearer tok_repo", req.Header.Get("Authorization"), rawURL)
	}

	// Other repositories from the same owner are not matched
	req, err := newExtensionRequest(context.Background(), "https://raw.githubusercontent.com/owner/other/main/manifest.json", tokens)
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestGitTokenForURLMostSpecificPatternWins(t *testing.T) {
	tokens := map[string]string{
		"gitea.example.com":            "tok_host",
		"gitea.example.com/owner":      "tok_owner",
		"gitea.example.com/owner/repo": "tok_repo",
	}

	req, err := newExtensionRequest(context.Background(), "https://gitea.example.com/owner/repo/raw/branch/main/manifest.json", tokens)
	require.NoError(t, err)
	require.Equal(t, "Bearer tok_repo", req.Header.Get("Authorization"))

	req, err = newExtensionRequest(context.Background(), "https://gitea.example.com/owner/other/raw/x.json", tokens)
	require.NoError(t, err)
	require.Equal(t, "Bearer tok_owner", req.Header.Get("Authorization"))

	req, err = newExtensionRequest(context.Background(), "https://gitea.example.com/someone/else.json", tokens)
	require.NoError(t, err)
	require.Equal(t, "Bearer tok_host", req.Header.Get("Authorization"))
}

func TestGitTokenForURLUserinfoTakesPrecedence(t *testing.T) {
	tokens := map[string]string{"example.com": "tok_stored"}

	req, err := newExtensionRequest(context.Background(), "https://tok_url@example.com/manifest.json", tokens)
	require.NoError(t, err)
	require.Equal(t, "Bearer tok_url", req.Header.Get("Authorization"))
}

func TestStoredGitTokenUsedForFetch(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)
	ext := testExternalExtension()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprintf(w, `{
			"id": %q, "name": %q, "version": %q, "manifestURI": %q,
			"language": %q, "type": %q, "description": %q, "author": %q,
			"lang": %q, "payload": %q
		}`, ext.ID, ext.Name, ext.Version, ext.ManifestURI,
			ext.Language, ext.Type, ext.Description, ext.Author,
			ext.Lang, ext.Payload)
	}))
	defer server.Close()

	// Without a stored token the fetch fails
	_, err := repo.FetchExternalExtensionData(server.URL + "/manifest.json")
	require.Error(t, err)

	// Store a token for the server's host and the fetch succeeds
	host := strings.TrimPrefix(server.URL, "http://")
	require.NoError(t, repo.SetGitToken(host, "secret-token"))

	fetched, err := repo.FetchExternalExtensionData(server.URL + "/manifest.json")
	require.NoError(t, err)
	require.Equal(t, ext.ID, fetched.ID)

	// Masked listing never exposes the full token
	list := repo.ListGitTokens()
	require.Len(t, list, 1)
	require.NotContains(t, list[0].MaskedToken, "secret-tok")
	require.Contains(t, list[0].MaskedToken, "oken")

	// Removing the token disables access again
	require.NoError(t, repo.RemoveGitToken(host))
	_, err = repo.FetchExternalExtensionData(server.URL + "/manifest.json")
	require.Error(t, err)
}
