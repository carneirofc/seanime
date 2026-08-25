package manga

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	chapter_downloader "seanime/internal/manga/downloader"
	"seanime/internal/testmocks"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// pngBytes returns a tiny but genuinely decodable PNG, so tests exercise the
// same code paths a real page image would.
func pngBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// buildZip builds an in-memory zip archive from entry name -> content.
func buildZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, name := range names {
		w, err := writer.Create(name)
		require.NoError(t, err)
		_, err = w.Write(entries[name])
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	return buf.Bytes()
}

// buildPageZip builds an archive of `count` page images under the given prefix.
func buildPageZip(t *testing.T, prefix string, count int) []byte {
	t.Helper()

	page := pngBytes(t)
	entries := make(map[string][]byte, count)
	for i := 1; i <= count; i++ {
		entries[fmt.Sprintf("%s%d.png", prefix, i)] = page
	}

	return buildZip(t, entries)
}

// writeLocalChapter writes a readable chapter archive into a series directory.
func writeLocalChapter(t *testing.T, seriesDir string, filename string, pages int) string {
	t.Helper()

	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	path := filepath.Join(seriesDir, filename)
	require.NoError(t, os.WriteFile(path, buildPageZip(t, "", pages), 0o644))

	return path
}

// readZipEntryNames lists the entry names of an archive on disk, in order.
func readZipEntryNames(t *testing.T, path string) []string {
	t.Helper()

	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer reader.Close()

	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}

	return names
}

// readZipPageNames lists only the page images of an archive, leaving out the
// metadata document.
func readZipPageNames(t *testing.T, path string) []string {
	t.Helper()

	names := make([]string, 0)
	for _, name := range readZipEntryNames(t, path) {
		if name == chapter_downloader.ComicInfoFilename {
			continue
		}
		names = append(names, name)
	}

	return names
}

// readComicInfo reads the metadata document out of a CBZ, failing if it has none.
func readComicInfo(t *testing.T, path string) *chapter_downloader.ComicInfo {
	t.Helper()

	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != chapter_downloader.ComicInfoFilename {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		defer rc.Close()

		info, err := chapter_downloader.ParseComicInfo(rc)
		require.NoError(t, err)
		return info
	}

	t.Fatalf("%s has no %s", path, chapter_downloader.ComicInfoFilename)
	return nil
}

// newLocalMangaCollection builds a manga collection out of media ID -> title.
func newLocalMangaCollection(t *testing.T, entries map[int]string) *anilist.MangaCollection {
	t.Helper()

	mediaIds := make([]int, 0, len(entries))
	for mediaId := range entries {
		mediaIds = append(mediaIds, mediaId)
	}
	sort.Ints(mediaIds)

	listEntries := make([]*anilist.MangaListEntry, 0, len(mediaIds))
	for _, mediaId := range mediaIds {
		listEntries = append(listEntries, &anilist.MangaListEntry{
			Media:  testmocks.NewBaseManga(mediaId, entries[mediaId]),
			Status: new(anilist.MediaListStatusCurrent),
		})
	}

	return &anilist.MangaCollection{MediaListCollection: &anilist.MangaCollection_MediaListCollection{
		Lists: []*anilist.MangaList{{Entries: listEntries}},
	}}
}
