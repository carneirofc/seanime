package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetFileSelectorContent(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "another"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Other.JSON"), []byte("{}"), 0o600))

	// Filter to .json (case-insensitive), directories always included.
	content, err := getFileSelectorContent(dir, []string{"json"})
	require.NoError(t, err)

	names := make([]string, 0, len(content))
	for _, e := range content {
		names = append(names, e.Name)
	}

	// Directories first (alphabetical), then matching files (alphabetical), .txt excluded.
	require.Equal(t, []string{"another", "sub", "manifest.json", "Other.JSON"}, names)
	require.True(t, content[0].IsDir)
	require.True(t, content[1].IsDir)
	require.False(t, content[2].IsDir)

	// No filter includes every file.
	all, err := getFileSelectorContent(dir, nil)
	require.NoError(t, err)
	require.Len(t, all, 5)
}
