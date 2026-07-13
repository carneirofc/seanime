package extension_repo

import (
	"os"
	"path/filepath"
	"runtime"
	"seanime/internal/extension"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

func TestLocalFileFromURI(t *testing.T) {
	type uriCase struct {
		name    string
		uri     string
		wantOK  bool
		wantHas string // substring the resolved path must contain (skipped when empty)
	}

	cases := []uriCase{
		{name: "http", uri: "https://example.com/ext.json", wantOK: false},
		{name: "relative", uri: "some/relative/path.json", wantOK: false},
		{name: "empty", uri: "", wantOK: false},
	}

	if runtime.GOOS != "windows" {
		cases = append(cases,
			uriCase{name: "abs posix", uri: "/home/me/ext.json", wantOK: true, wantHas: "/home/me/ext.json"},
			uriCase{name: "file url", uri: "file:///home/me/ext.json", wantOK: true, wantHas: "/home/me/ext.json"},
		)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, ok := localFileFromURI(tc.uri)
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantOK, IsLocalFileURI(tc.uri))
			if tc.wantOK && tc.wantHas != "" {
				require.Contains(t, path, tc.wantHas)
			}
		})
	}
}

// writeLocalExtensionSource writes a manifest + separate payload file to srcDir and
// returns the manifest path and the file:// URI of the payload.
func writeLocalExtensionSource(t *testing.T, srcDir string, manifestURI string, payload string) (manifestPath string) {
	t.Helper()

	payloadPath := filepath.Join(srcDir, "payload.js")
	require.NoError(t, os.WriteFile(payloadPath, []byte(payload), 0600))

	ext := extension.Extension{
		ID:          "local-torrent-provider",
		Name:        "Local Torrent Provider",
		Version:     "1.0.0",
		ManifestURI: manifestURI,
		PayloadURI:  "file://" + filepath.ToSlash(payloadPath),
		Language:    extension.LanguageJavascript,
		Type:        extension.TypeAnimeTorrentProvider,
		Description: "Local test provider",
		Author:      "Test",
		Lang:        "en",
	}

	manifestPath = filepath.Join(srcDir, "manifest.json")
	raw, err := json.Marshal(ext)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, raw, 0600))
	return manifestPath
}

func TestResolveExtensionURI(t *testing.T) {
	// Absolute references are returned unchanged.
	require.Equal(t, "https://example.com/x.json", resolveExtensionURI("file:///repo/repo.json", "https://example.com/x.json"))
	require.Equal(t, "file:///other/x.json", resolveExtensionURI("file:///repo/repo.json", "file:///other/x.json"))

	if runtime.GOOS != "windows" {
		require.Equal(t, "/abs/x.json", resolveExtensionURI("/repo/repo.json", "/abs/x.json"))
		// Relative refs resolve against a local base's directory.
		require.Equal(t, "/repo/sub/x.json", resolveExtensionURI("/repo/repo.json", "sub/x.json"))
		require.Equal(t, "/repo/sub/x.json", resolveExtensionURI("file:///repo/repo.json", "sub/x.json"))
	}

	// Relative refs resolve against a remote base URL.
	require.Equal(t, "https://host/a/x.json", resolveExtensionURI("https://host/a/repo.json", "x.json"))
	// With no base (JSON-literal repository), a relative ref is left as-is.
	require.Equal(t, "x.json", resolveExtensionURI("", "x.json"))
}

func TestInstallExternalExtensionFromLocalFile(t *testing.T) {
	repo, extensionDir := newExternalExtensionTestRepository(t)

	srcDir := t.TempDir()
	// The stored manifestURI (used later for reload-from-source) is the absolute path.
	manifestPath := writeLocalExtensionSource(t, srcDir, "", asyncAnimeTorrentProviderPayload)

	// Install using a plain absolute filesystem path (exercises the IsAbs branch),
	// with the manifest self-declaring the absolute path as its manifestURI.
	rewriteManifestURI(t, manifestPath, manifestPath)

	res, err := repo.InstallExternalExtension(manifestPath)
	require.NoError(t, err)
	require.Contains(t, res.Message, "installed")

	// The extension file was written to the extension dir.
	_, err = os.Stat(filepath.Join(extensionDir, "local-torrent-provider.json"))
	require.NoError(t, err)

	// It loaded successfully (payload came from the local payloadUri) and is available.
	all := repo.GetAllExtensions(false)
	require.Empty(t, all.InvalidExtensions)
	require.Len(t, all.Extensions, 1)
	require.Equal(t, "local-torrent-provider", all.Extensions[0].ID)

	_, found := repo.GetAnimeTorrentProviderExtensionByID("local-torrent-provider")
	require.True(t, found)
}

func TestRefetchExternalExtensionReloadsFromSourceWithoutVersionBump(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	srcDir := t.TempDir()
	manifestPath := filepath.Join(srcDir, "manifest.json")
	// Install from a file:// manifest URI, self-declaring the same URI so it can be reloaded.
	fileURI := "file://" + filepath.ToSlash(manifestPath)
	writeLocalExtensionSource(t, srcDir, fileURI, asyncAnimeTorrentProviderPayload)

	_, err := repo.InstallExternalExtension(fileURI)
	require.NoError(t, err)
	require.NotContains(t, repo.GetExtensionPayload("local-torrent-provider"), "EDITED")

	// Edit the payload in place WITHOUT bumping the manifest version.
	editedPayload := "// EDITED\n" + asyncAnimeTorrentProviderPayload
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "payload.js"), []byte(editedPayload), 0600))

	// checkForUpdates must NOT report anything (version is unchanged)...
	require.Empty(t, repo.checkForUpdates())

	// ...but reload-from-source must pick up the new payload.
	res, err := repo.RefetchExternalExtension("local-torrent-provider")
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Contains(t, repo.GetExtensionPayload("local-torrent-provider"), "EDITED")

	// RefetchAll should report it as reloaded.
	all := repo.RefetchAllExternalExtensions()
	require.Contains(t, all.Reloaded, "local-torrent-provider")
	require.Empty(t, all.Failed)
}

// TestInstallExternalExtensionsFromLocalRepositoryRelative covers installing from
// a local repository file whose entries are relative paths, whose manifests carry
// relative payload URIs, and whose self-declared manifestURI points elsewhere
// (a remote URL that is never fetched). All references must resolve locally and
// every extension must actually install.
func TestInstallExternalExtensionsFromLocalRepositoryRelative(t *testing.T) {
	repo, _ := newExternalExtensionTestRepository(t)

	repoDir := t.TempDir()

	writeProvider := func(sub, id string) {
		dir := filepath.Join(repoDir, sub)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "payload.js"), []byte(asyncAnimeTorrentProviderPayload), 0o600))
		ext := extension.Extension{
			ID:   id,
			Name: id,
			// A self-declared manifestURI that would 404 if re-fetched; install
			// must use the fetched data, not this URI.
			ManifestURI: "https://example.com/does-not-exist/" + id + ".json",
			PayloadURI:  "payload.js", // relative to this manifest's directory
			Version:     "1.0.0",
			Language:    extension.LanguageJavascript,
			Type:        extension.TypeAnimeTorrentProvider,
			Description: "local repo provider",
			Author:      "Test",
			Lang:        "en",
		}
		raw, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600))
	}

	writeProvider("a", "prov-a")
	writeProvider("b", "prov-b")

	// Repository file at the repo root with relative entries.
	repoFile := filepath.Join(repoDir, "repo.json")
	raw, err := json.Marshal(map[string][]string{"urls": {"a/manifest.json", "b/manifest.json"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(repoFile, raw, 0o600))

	res, err := repo.InstallExternalExtensions("file://"+filepath.ToSlash(repoFile), true)
	require.NoError(t, err)
	require.Contains(t, res.Message, "installed 2 of 2")

	all := repo.GetAllExtensions(false)
	require.Empty(t, all.InvalidExtensions)
	require.Len(t, all.Extensions, 2)

	for _, id := range []string{"prov-a", "prov-b"} {
		_, found := repo.GetAnimeTorrentProviderExtensionByID(id)
		require.Truef(t, found, "expected %s to be installed and loaded", id)
	}
}

// rewriteManifestURI rewrites the manifestURI field of an on-disk manifest file.
func rewriteManifestURI(t *testing.T, manifestPath string, manifestURI string) {
	t.Helper()

	raw, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var ext extension.Extension
	require.NoError(t, json.Unmarshal(raw, &ext))
	ext.ManifestURI = manifestURI

	out, err := json.Marshal(ext)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, out, 0600))
}
