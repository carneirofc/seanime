package chapter_downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func writeLegacyChapterDir(t *testing.T, downloadDir string, id DownloadID, withRegistry bool) string {
	t.Helper()

	dir := filepath.Join(downloadDir, FormatChapterDirName(id.Provider, id.MediaId, id.ChapterId, id.ChapterNumber))
	require.NoError(t, os.MkdirAll(dir, os.ModePerm))

	imageData := newTestPNG(t)
	registry := make(Registry)
	for i, name := range []string{"01.png", "02.png"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), imageData, 0644))
		registry[i] = PageInfo{Index: i, Filename: name, Size: int64(len(imageData)), Width: 1, Height: 1}
	}

	if withRegistry {
		data, err := json.Marshal(registry)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "registry.json"), data, 0644))
	}

	return dir
}

func migrationTestID() DownloadID {
	return DownloadID{Provider: "comick", MediaId: 1234, ChapterId: "ch_1", ChapterNumber: "13.5"}
}

func expectedCBZPath(downloadDir string, id DownloadID) string {
	return filepath.Join(downloadDir, FormatSeriesDirName(id.Provider, id.MediaId), FormatChapterFileName(id.ChapterId, id.ChapterNumber))
}

func TestMigrateLegacyChapterDir(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()
	id := migrationTestID()
	srcDir := writeLegacyChapterDir(t, tmp, id, true)

	MigrateLegacyChapterDirs(tmp, &logger)

	// Source directory is gone, CBZ exists with re-padded entries and dimensions
	_, err := os.Stat(srcDir)
	require.True(t, os.IsNotExist(err))

	entries, info, err := ReadCBZ(expectedCBZPath(tmp, id))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "001.png", entries[0].Name)
	require.Equal(t, "002.png", entries[1].Name)
	require.Equal(t, 1, entries[0].Width)
	require.NotNil(t, info)
	require.Equal(t, id.ChapterNumber, info.Number)
	require.Equal(t, 2, info.PageCount)

	// The migrated chapter is discoverable by the scanner
	scanned := ScanDownloadDir(tmp)
	require.Contains(t, scanned, id)
}

func TestMigrateLegacyChapterDirWithoutRegistry(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()
	id := migrationTestID()
	srcDir := writeLegacyChapterDir(t, tmp, id, false)

	MigrateLegacyChapterDirs(tmp, &logger)

	_, err := os.Stat(srcDir)
	require.True(t, os.IsNotExist(err))

	entries, _, err := ReadCBZ(expectedCBZPath(tmp, id))
	require.NoError(t, err)
	require.Len(t, entries, 2)
	// Dimensions decoded from the images since the registry was synthesized
	require.Equal(t, 1, entries[0].Width)
	require.Equal(t, 1, entries[0].Height)
}

func TestMigrateLegacyChapterDirMissingPagePreservesDir(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()
	id := migrationTestID()
	srcDir := writeLegacyChapterDir(t, tmp, id, true)
	require.NoError(t, os.Remove(filepath.Join(srcDir, "02.png")))

	MigrateLegacyChapterDirs(tmp, &logger)

	// Directory untouched, no CBZ written
	_, err := os.Stat(srcDir)
	require.NoError(t, err)
	_, err = os.Stat(expectedCBZPath(tmp, id))
	require.True(t, os.IsNotExist(err))

	// The chapter remains readable through the legacy scan branch
	scanned := ScanDownloadDir(tmp)
	require.Contains(t, scanned, id)
}

func TestMigrateLegacyChapterDirExistingCBZWins(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()
	id := migrationTestID()
	srcDir := writeLegacyChapterDir(t, tmp, id, true)

	// Pre-existing archive from a previous run that crashed before cleanup
	destPath := expectedCBZPath(tmp, id)
	require.NoError(t, os.MkdirAll(filepath.Dir(destPath), os.ModePerm))
	require.NoError(t, os.WriteFile(destPath, []byte("existing"), 0644))

	MigrateLegacyChapterDirs(tmp, &logger)

	// Legacy dir removed, existing archive untouched
	_, err := os.Stat(srcDir)
	require.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(destPath)
	require.NoError(t, err)
	require.Equal(t, []byte("existing"), data)
}

func TestMigrateLegacyChapterDirsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()
	id := migrationTestID()
	writeLegacyChapterDir(t, tmp, id, true)

	MigrateLegacyChapterDirs(tmp, &logger)
	firstInfo, err := os.Stat(expectedCBZPath(tmp, id))
	require.NoError(t, err)

	MigrateLegacyChapterDirs(tmp, &logger)
	secondInfo, err := os.Stat(expectedCBZPath(tmp, id))
	require.NoError(t, err)
	require.Equal(t, firstInfo.ModTime(), secondInfo.ModTime())

	scanned := ScanDownloadDir(tmp)
	require.Len(t, scanned, 1)
}

func TestMigrateSweepsTempArchives(t *testing.T) {
	tmp := t.TempDir()
	logger := zerolog.Nop()

	seriesDir := filepath.Join(tmp, FormatSeriesDirName("comick", 1))
	require.NoError(t, os.MkdirAll(seriesDir, os.ModePerm))
	tmpArchive := filepath.Join(seriesDir, "0001_ch-1.cbz.tmp")
	require.NoError(t, os.WriteFile(tmpArchive, []byte("partial"), 0644))

	MigrateLegacyChapterDirs(tmp, &logger)

	_, err := os.Stat(tmpArchive)
	require.True(t, os.IsNotExist(err))
}
