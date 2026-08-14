package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateExecutablePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits and /bin layout are POSIX-specific")
	}

	dataDir := t.TempDir()
	t.Cleanup(func() { SetUntrustedExecutableRoots(nil) })
	SetUntrustedExecutableRoots([]string{dataDir})

	writeExecutable := func(dir string, name string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755))
		return path
	}

	t.Run("an empty path means use the default", func(t *testing.T) {
		resolved, err := ValidateExecutablePath("")
		assert.NoError(t, err)
		assert.Empty(t, resolved)
	})

	t.Run("resolves a bare name through PATH", func(t *testing.T) {
		resolved, err := ValidateExecutablePath("sh")
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(resolved), "expected an absolute path, got %q", resolved)
	})

	t.Run("refuses a binary inside a writable directory", func(t *testing.T) {
		// This is the escalation that matters in a container: /data is the writable
		// volume, so anything an attacker can download or extract into it would
		// otherwise become an executable transcoder.
		planted := writeExecutable(dataDir, "ffmpeg")

		_, err := ValidateExecutablePath(planted)
		assert.Error(t, err)
	})

	t.Run("refuses a binary in a subdirectory of a writable directory", func(t *testing.T) {
		nested := filepath.Join(dataDir, "cache", "bin")
		require.NoError(t, os.MkdirAll(nested, 0o755))
		planted := writeExecutable(nested, "ffprobe")

		_, err := ValidateExecutablePath(planted)
		assert.Error(t, err)
	})

	t.Run("refuses a symlink that escapes into a writable directory", func(t *testing.T) {
		planted := writeExecutable(dataDir, "payload")
		linkDir := t.TempDir()
		link := filepath.Join(linkDir, "ffmpeg")
		require.NoError(t, os.Symlink(planted, link))

		_, err := ValidateExecutablePath(link)
		assert.Error(t, err)
	})

	t.Run("refuses a missing binary", func(t *testing.T) {
		_, err := ValidateExecutablePath(filepath.Join(t.TempDir(), "nope"))
		assert.Error(t, err)
	})

	t.Run("refuses a non-executable file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "notes.txt")
		require.NoError(t, os.WriteFile(path, []byte("hello"), 0o644))

		_, err := ValidateExecutablePath(path)
		assert.Error(t, err)
	})

	t.Run("allows a binary outside every writable directory", func(t *testing.T) {
		dir := t.TempDir()
		path := writeExecutable(dir, "ffmpeg")

		resolved, err := ValidateExecutablePath(path)
		require.NoError(t, err)
		assert.Equal(t, path, resolved)
	})
}
