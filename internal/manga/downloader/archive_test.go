package chapter_downloader

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func readZipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	entries := make(map[string][]byte)
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		entries[f.Name] = content
	}
	return entries
}

func TestWriteLegacyDirAsCBZ(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "02.png"), []byte("page-two"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "01.png"), []byte("page-one"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "registry.json"), []byte("{}"), 0644))

	var buf bytes.Buffer
	require.NoError(t, WriteLegacyDirAsCBZ(&buf, dir))

	entries := readZipEntries(t, buf.Bytes())
	require.Len(t, entries, 2, "registry.json must not be included")
	require.Equal(t, []byte("page-one"), entries["01.png"])
	require.Equal(t, []byte("page-two"), entries["02.png"])
}

func TestWriteLegacyDirAsCBZNoImages(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "registry.json"), []byte("{}"), 0644))

	var buf bytes.Buffer
	require.Error(t, WriteLegacyDirAsCBZ(&buf, dir))
}

func TestWriteMediaArchive(t *testing.T) {
	downloadDir := t.TempDir()

	// New-layout chapter: comick_1/0001_ch-1.cbz
	seriesDir := filepath.Join(downloadDir, FormatSeriesDirName("comick", 1))
	require.NoError(t, os.MkdirAll(seriesDir, os.ModePerm))
	var cbzBuf bytes.Buffer
	zw := zip.NewWriter(&cbzBuf)
	w, err := zw.Create("001.png")
	require.NoError(t, err)
	_, err = w.Write([]byte("cbz-page"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	cbzName := FormatChapterFileName("ch-1", "0001")
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, cbzName), cbzBuf.Bytes(), 0644))

	// Legacy-layout chapter for the same media: comick_1_ch-2_0002/
	legacyDir := filepath.Join(downloadDir, FormatChapterDirName("comick", 1, "ch-2", "0002"))
	require.NoError(t, os.MkdirAll(legacyDir, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "01.png"), []byte("legacy-page"), 0644))

	// Chapter of a different media that must not be included
	otherSeriesDir := filepath.Join(downloadDir, FormatSeriesDirName("comick", 2))
	require.NoError(t, os.MkdirAll(otherSeriesDir, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(otherSeriesDir, FormatChapterFileName("ch-9", "0009")), cbzBuf.Bytes(), 0644))

	var buf bytes.Buffer
	written, err := WriteMediaArchive(&buf, downloadDir, "comick", 1)
	require.NoError(t, err)
	require.Equal(t, 2, written)

	entries := readZipEntries(t, buf.Bytes())
	require.Len(t, entries, 2)

	// CBZ chapter is copied verbatim
	require.Equal(t, cbzBuf.Bytes(), entries[cbzName])

	// Legacy chapter is wrapped into a CBZ entry named like the new layout
	legacyEntryName := FormatChapterFileName("ch-2", "0002")
	innerEntries := readZipEntries(t, entries[legacyEntryName])
	require.Equal(t, []byte("legacy-page"), innerEntries["01.png"])
}

func TestWriteMediaArchiveNoChapters(t *testing.T) {
	var buf bytes.Buffer
	_, err := WriteMediaArchive(&buf, t.TempDir(), "comick", 1)
	require.Error(t, err)
}
