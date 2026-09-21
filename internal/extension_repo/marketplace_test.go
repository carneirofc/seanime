package extension_repo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"seanime/internal/extension"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

// writeMarketplaceProvider writes a manifest + sibling payload under dir/sub, with the
// manifest referencing its payload by a relative path and declaring no manifestURI of its
// own — the monorepo layout the marketplace help text documents. It returns the manifest's
// absolute path.
func writeMarketplaceProvider(t *testing.T, dir, sub, id string) string {
	t.Helper()

	extDir := filepath.Join(dir, sub)
	require.NoError(t, os.MkdirAll(extDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(extDir, "payload.js"), []byte(asyncAnimeTorrentProviderPayload), 0o600))

	ext := extension.Extension{
		ID:          id,
		Name:        id,
		ManifestURI: "", // ignored on install: the URI it was fetched from is recorded instead
		PayloadURI:  "payload.js",
		Version:     "1.0.0",
		Language:    extension.LanguageJavascript,
		Type:        extension.TypeAnimeTorrentProvider,
		Description: "monorepo marketplace provider",
		Author:      "Test",
		Lang:        "en",
	}
	raw, err := json.Marshal(ext)
	require.NoError(t, err)

	manifestPath := filepath.Join(extDir, "manifest.json")
	require.NoError(t, os.WriteFile(manifestPath, raw, 0o600))
	return manifestPath
}

// writeMarketplaceListing marshals entries to dir/name and returns the file's path.
func writeMarketplaceListing(t *testing.T, path string, listing any) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	raw, err := json.Marshal(listing)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

// TestGetMarketplaceExtensionsResolvesRelativeManifestURI covers a local, monorepo-style
// marketplace: the marketplace JSON lives alongside the extensions it lists, and each
// entry's manifestURI is relative to the marketplace file's own directory. The returned
// extensions must have their manifestURI resolved to an absolute path so they can be
// installed directly, and the installed extension must record where it came from so it can
// later be reloaded and checked for updates.
func TestGetMarketplaceExtensionsResolvesRelativeManifestURI(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	repoDir := t.TempDir()
	manifestPath := writeMarketplaceProvider(t, repoDir, "a", "prov-a")

	marketplaceFile := writeMarketplaceListing(t, filepath.Join(repoDir, "marketplace.json"), []map[string]string{
		{"id": "prov-a", "manifestURI": "a/manifest.json"},
	})

	extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
	require.NoError(t, err)
	require.Len(t, extensions, 1)
	require.Equal(t, filepath.ToSlash(manifestPath), extensions[0].ManifestURI)

	// The resolved manifestURI must actually be installable.
	res, err := repo.InstallExternalExtension(extensions[0].ManifestURI)
	require.NoError(t, err)
	require.Contains(t, res.Message, "installed")

	loaded, found := repo.GetAnimeTorrentProviderExtensionByID("prov-a")
	require.True(t, found)

	// The manifest declared no manifestURI of its own; the URI it was fetched from is what
	// gets recorded, so reloading from source works.
	require.Equal(t, filepath.ToSlash(manifestPath), loaded.GetManifestURI())
	require.Equal(t, filepath.ToSlash(filepath.Join(repoDir, "a", "payload.js")), loaded.GetPayloadURI())

	_, err = repo.RefetchExternalExtension("prov-a")
	require.NoError(t, err)
}

// TestGetMarketplaceExtensionsResolutionEdgeCases exercises the resolution logic against a
// local marketplace file: an absolute entry passes through unchanged, a relative entry
// resolves against the marketplace's directory, an already-remote entry is left alone, and
// entries that can't be acted on — an empty manifestURI, a null element, an ID that is not a
// usable filename — are dropped.
func TestGetMarketplaceExtensionsResolutionEdgeCases(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	repoDir := t.TempDir()
	marketplaceDir := filepath.Join(repoDir, "sub")

	relativeManifest := writeMarketplaceProvider(t, marketplaceDir, "extensions", "relative-entry")
	elsewhereDir := t.TempDir()
	absoluteManifest := writeMarketplaceProvider(t, elsewhereDir, "abs", "absolute-entry")

	remoteManifest := marshalManifest(t, "remote-entry")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/remote.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(remoteManifest)
	}))
	defer server.Close()

	marketplaceFile := writeMarketplaceListing(t, filepath.Join(marketplaceDir, "marketplace.json"), []any{
		map[string]string{"id": "relative-entry", "manifestURI": "extensions/manifest.json"},
		map[string]string{"id": "absolute-entry", "manifestURI": absoluteManifest},
		map[string]string{"id": "remote-entry", "manifestURI": server.URL + "/remote.json"},
		map[string]string{"id": "no-manifest-entry", "manifestURI": ""},
		map[string]string{"id": "../escape", "manifestURI": "extensions/manifest.json"},
		nil,
	})

	extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
	require.NoError(t, err)
	require.Len(t, extensions, 3)

	byID := make(map[string]*extension.Extension, len(extensions))
	for _, ext := range extensions {
		byID[ext.ID] = ext
	}

	require.Equal(t, filepath.ToSlash(relativeManifest), byID["relative-entry"].ManifestURI,
		"a relative entry must resolve against the marketplace's own directory")
	require.Equal(t, absoluteManifest, byID["absolute-entry"].ManifestURI,
		"an already-absolute entry must be left unchanged")
	require.Equal(t, server.URL+"/remote.json", byID["remote-entry"].ManifestURI,
		"an already-absolute remote entry must be left unchanged")
	require.NotContains(t, byID, "no-manifest-entry")
	require.NotContains(t, byID, "../escape")
}

// TestGetMarketplaceExtensionsResolvesRelativeManifestURIAgainstRemoteBase covers a
// marketplace served over HTTP whose entries use paths relative to the marketplace URL,
// mirroring how a remote monorepo (e.g. GitHub raw content) would be laid out.
func TestGetMarketplaceExtensionsResolvesRelativeManifestURIAgainstRemoteBase(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	manifest := marshalManifest(t, "remote-provider")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repo/marketplace.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"remote-provider","manifestURI":"extensions/remote/manifest.json"}]`))
		case "/repo/extensions/remote/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(manifest)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	extensions, err := repo.getMarketplaceExtensions(server.URL + "/repo/marketplace.json")
	require.NoError(t, err)
	require.Len(t, extensions, 1)
	require.Equal(t, server.URL+"/repo/extensions/remote/manifest.json", extensions[0].ManifestURI)
}

// TestGetMarketplaceExtensionsHydratesIncompleteEntries covers the display metadata a
// minimal monorepo listing leaves out. Without it the entry has no type, which is what the
// UI groups by, so it is rendered nowhere at all.
func TestGetMarketplaceExtensionsHydratesIncompleteEntries(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	repoDir := t.TempDir()
	writeMarketplaceProvider(t, repoDir, "a", "prov-a")
	writeMarketplaceProvider(t, repoDir, "b", "prov-b")

	marketplaceFile := writeMarketplaceListing(t, filepath.Join(repoDir, "marketplace.json"), []map[string]string{
		// Nothing but an ID and a pointer: every displayed field comes from the manifest.
		{"id": "prov-a", "manifestURI": "a/manifest.json"},
		// The listing's own values win over the manifest's.
		{"id": "prov-b", "manifestURI": "b/manifest.json", "name": "Listing Name", "description": "Listing description"},
	})

	extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
	require.NoError(t, err)
	require.Len(t, extensions, 2)

	byID := make(map[string]*extension.Extension, len(extensions))
	for _, ext := range extensions {
		byID[ext.ID] = ext
	}

	hydrated := byID["prov-a"]
	require.Equal(t, "prov-a", hydrated.Name)
	require.Equal(t, extension.TypeAnimeTorrentProvider, hydrated.Type)
	require.Equal(t, extension.LanguageJavascript, hydrated.Language)
	require.Equal(t, "Test", hydrated.Author)
	require.Equal(t, "1.0.0", hydrated.Version)
	require.Equal(t, "monorepo marketplace provider", hydrated.Description)
	require.Empty(t, hydrated.Payload, "listing entries must not carry payloads")

	overridden := byID["prov-b"]
	require.Equal(t, "Listing Name", overridden.Name)
	require.Equal(t, "Listing description", overridden.Description)
	require.Equal(t, extension.TypeAnimeTorrentProvider, overridden.Type, "unset fields still come from the manifest")
}

// TestGetMarketplaceExtensionsFiltersUnusableEntries covers entries that survive resolution
// but could never be shown or installed: a manifest that isn't there, and an entry whose
// type the app doesn't know. Both used to be returned and rendered as blank, uninstallable
// cards.
func TestGetMarketplaceExtensionsFiltersUnusableEntries(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	repoDir := t.TempDir()
	writeMarketplaceProvider(t, repoDir, "good", "good-entry")

	marketplaceFile := writeMarketplaceListing(t, filepath.Join(repoDir, "marketplace.json"), []map[string]string{
		{"id": "good-entry", "manifestURI": "good/manifest.json"},
		{"id": "missing-entry", "manifestURI": "missing/manifest.json"},
		{
			"id": "unknown-type-entry", "manifestURI": "good/manifest.json",
			"name": "Unknown", "author": "Test", "language": "javascript", "type": "not-a-real-type",
		},
	})

	extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
	require.NoError(t, err)
	require.Len(t, extensions, 1)
	require.Equal(t, "good-entry", extensions[0].ID)
}

// TestGetMarketplaceExtensionsResolvesBrowserFacingAssets covers icon/readme/website, which
// the browser loads rather than the server. Relative against a remote marketplace they
// resolve to a URL the browser can reach; relative against a local one they would resolve to
// a server filesystem path, which renders as a broken image, so they are dropped.
func TestGetMarketplaceExtensionsResolvesBrowserFacingAssets(t *testing.T) {
	t.Run("remote base", func(t *testing.T) {
		repo, _ := newExternalExtensionTestRepository(t)

		manifest := marshalManifest(t, "remote-provider")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/repo/marketplace.json":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[{"id":"remote-provider","manifestURI":"ext/manifest.json","icon":"ext/icon.png"}]`))
			case "/repo/ext/manifest.json":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(manifest)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		extensions, err := repo.getMarketplaceExtensions(server.URL + "/repo/marketplace.json")
		require.NoError(t, err)
		require.Len(t, extensions, 1)
		require.Equal(t, server.URL+"/repo/ext/icon.png", extensions[0].Icon)
	})

	t.Run("local base", func(t *testing.T) {
		repo, _ := newExternalExtensionTestRepository(t)

		repoDir := t.TempDir()
		writeMarketplaceProvider(t, repoDir, "a", "prov-a")
		marketplaceFile := writeMarketplaceListing(t, filepath.Join(repoDir, "marketplace.json"), []map[string]string{
			{"id": "prov-a", "manifestURI": "a/manifest.json", "icon": "a/icon.png", "website": "a/readme.html"},
		})

		extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
		require.NoError(t, err)
		require.Len(t, extensions, 1)
		require.Empty(t, extensions[0].Icon, "a server filesystem path is not loadable by the browser")
		require.Empty(t, extensions[0].Website)
	})

	t.Run("absolute values pass through", func(t *testing.T) {
		repo, _ := newExternalExtensionTestRepository(t)

		repoDir := t.TempDir()
		writeMarketplaceProvider(t, repoDir, "a", "prov-a")
		marketplaceFile := writeMarketplaceListing(t, filepath.Join(repoDir, "marketplace.json"), []map[string]string{
			{"id": "prov-a", "manifestURI": "a/manifest.json", "icon": "https://cdn.example.com/icon.png"},
		})

		extensions, err := repo.getMarketplaceExtensions(marketplaceFile)
		require.NoError(t, err)
		require.Len(t, extensions, 1)
		require.Equal(t, "https://cdn.example.com/icon.png", extensions[0].Icon)
	})
}

// marshalManifest returns a servable manifest for id, with an inline payload so it needs no
// sibling file.
func marshalManifest(t *testing.T, id string) []byte {
	t.Helper()

	raw, err := json.Marshal(extension.Extension{
		ID:          id,
		Name:        id,
		Version:     "1.0.0",
		Language:    extension.LanguageJavascript,
		Type:        extension.TypeAnimeTorrentProvider,
		Description: "remote marketplace provider",
		Author:      "Test",
		Lang:        "en",
		Payload:     asyncAnimeTorrentProviderPayload,
	})
	require.NoError(t, err)
	return raw
}
