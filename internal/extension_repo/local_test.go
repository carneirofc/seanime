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
