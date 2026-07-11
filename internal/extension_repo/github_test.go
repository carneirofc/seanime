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
	req, err := newExtensionRequest(context.Background(), "https://ghp_secret@raw.githubusercontent.com/owner/private/main/marketplace.json")
	require.NoError(t, err)

	require.Equal(t, "Bearer ghp_secret", req.Header.Get("Authorization"))
	require.Nil(t, req.URL.User, "userinfo should be stripped from the request URL")
	require.Equal(t, "https://raw.githubusercontent.com/owner/private/main/marketplace.json", req.URL.String())
}

func TestNewExtensionRequestBasicAuthUserinfoIsPreserved(t *testing.T) {
	req, err := newExtensionRequest(context.Background(), "https://user:ghp_secret@raw.githubusercontent.com/owner/private/main/marketplace.json")
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

	req, err := newExtensionRequest(context.Background(), "https://raw.githubusercontent.com/owner/private/main/marketplace.json")
	require.NoError(t, err)
	require.Equal(t, "Bearer ghp_env_token", req.Header.Get("Authorization"))
}

func TestNewExtensionRequestEnvTokenFallback(t *testing.T) {
	t.Setenv("SEANIME_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "ghp_fallback")

	req, err := newExtensionRequest(context.Background(), "https://api.github.com/repos/owner/private/contents/marketplace.json")
	require.NoError(t, err)
	require.Equal(t, "Bearer ghp_fallback", req.Header.Get("Authorization"))
}

func TestNewExtensionRequestEnvTokenNotSentToOtherHosts(t *testing.T) {
	t.Setenv("SEANIME_GITHUB_TOKEN", "ghp_env_token")

	req, err := newExtensionRequest(context.Background(), "https://example.com/marketplace.json")
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestNewExtensionRequestGitHubContentsAPIAcceptHeader(t *testing.T) {
	req, err := newExtensionRequest(context.Background(), "https://api.github.com/repos/owner/private/contents/marketplace.json?ref=main")
	require.NoError(t, err)
	require.Equal(t, "application/vnd.github.raw+json", req.Header.Get("Accept"))

	req, err = newExtensionRequest(context.Background(), "https://raw.githubusercontent.com/owner/private/main/marketplace.json")
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
