package manga

import (
	"os"
	"path/filepath"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepackLocalMangaSeriesRewritesZipAsCBZ(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	// A hand-assembled chapter: a plain .zip whose pages sit in a folder and are
	// numbered without padding — readable by nothing without normalization.
	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	page := pngBytes(t)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 3.zip"), buildZip(t, map[string][]byte{
		"scans/1.png":  page,
		"scans/2.png":  page,
		"scans/10.png": page,
		"Thumbs.db":    []byte("junk"),
	}), 0o644))

	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 21, "One Piece"))

	result, err := repository.RepackLocalMangaSeries("One Piece", newLocalMangaCollection(t, map[int]string{21: "One Piece"}))
	require.NoError(t, err)

	require.Len(t, result.Repacked, 1)
	require.Equal(t, "Chapter 3.zip", result.Repacked[0].From)
	require.Equal(t, []string{"Chapter 3.cbz"}, result.Repacked[0].To)
	require.Equal(t, 3, result.Repacked[0].PageCount)

	// The .zip is gone, replaced by a .cbz.
	require.NoFileExists(t, filepath.Join(seriesDir, "Chapter 3.zip"))
	repacked := filepath.Join(seriesDir, "Chapter 3.cbz")
	require.FileExists(t, repacked)

	// Pages are flattened, ordered and stripped of junk.
	require.Equal(t, []string{"001.png", "002.png", "003.png"}, readZipPageNames(t, repacked))

	info := readComicInfo(t, repacked)
	require.Equal(t, "One Piece", info.Series)
	require.Equal(t, "3", info.Number)
	require.Equal(t, 3, info.PageCount)
	require.Equal(t, chapter_downloader.MangaRightToLeft, info.Manga)

	requireNoStagingFiles(t, seriesDir)
}

func TestRepackLocalMangaSeriesRefreshesMetadataInPlace(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	writeLocalChapter(t, seriesDir, "Chapter 1.cbz", 2)
	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 21, "One Piece"))

	result, err := repository.RepackLocalMangaSeries("One Piece", newLocalMangaCollection(t, map[int]string{21: "One Piece"}))
	require.NoError(t, err)

	// Rewriting a chapter under its own name must replace it, not duplicate it.
	require.Len(t, result.Repacked, 1)
	require.Equal(t, []string{"Chapter 1.cbz"}, result.Repacked[0].To)

	entries, err := os.ReadDir(seriesDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	info := readComicInfo(t, filepath.Join(seriesDir, "Chapter 1.cbz"))
	require.Equal(t, "One Piece", info.Series)
	require.Equal(t, "1", info.Number)
}

func TestRepackLocalMangaSeriesSplitsMultiChapterArchives(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "Berserk")

	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	page := pngBytes(t)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Volume 1.zip"), buildZip(t, map[string][]byte{
		"Chapter 1/001.png": page,
		"Chapter 2/001.png": page,
	}), 0o644))

	result, err := repository.RepackLocalMangaSeries("Berserk", nil)
	require.NoError(t, err)

	require.Len(t, result.Repacked, 1)
	require.ElementsMatch(t, []string{"Chapter 1.cbz", "Chapter 2.cbz"}, result.Repacked[0].To)
	require.NoFileExists(t, filepath.Join(seriesDir, "Volume 1.zip"))
	require.FileExists(t, filepath.Join(seriesDir, "Chapter 1.cbz"))
	require.FileExists(t, filepath.Join(seriesDir, "Chapter 2.cbz"))
}

func TestRepackLocalMangaSeriesSkipsWhatItCannotRewrite(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 1.cbr"), []byte("Rar!"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 2.pdf"), []byte("%PDF-"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 3.cbz"), []byte("not a zip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 4.zip"), buildZip(t, map[string][]byte{
		"notes.txt": []byte("no pages here"),
	}), 0o644))

	result, err := repository.RepackLocalMangaSeries("One Piece", nil)
	require.NoError(t, err)
	require.Empty(t, result.Repacked)

	reasons := make(map[string]string, len(result.Skipped))
	for _, skipped := range result.Skipped {
		reasons[skipped.Name] = skipped.Reason
	}
	require.Equal(t, LocalMangaRepackUnsupported, reasons["Chapter 1.cbr"])
	require.Equal(t, LocalMangaRepackUnsupported, reasons["Chapter 2.pdf"])
	require.Equal(t, LocalMangaRepackUnsupported, reasons["Chapter 3.cbz"])
	require.Equal(t, LocalMangaRepackNoPages, reasons["Chapter 4.zip"])

	// Nothing it could not rewrite may have been touched.
	for _, name := range []string{"Chapter 1.cbr", "Chapter 2.pdf", "Chapter 3.cbz", "Chapter 4.zip"} {
		require.FileExists(t, filepath.Join(seriesDir, name))
	}
	requireNoStagingFiles(t, seriesDir)
}

func TestRepackLocalMangaSeriesRefusesToClobberAnotherChapter(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	// "Chapter 1.zip" would be rebuilt as "Chapter 1.cbz", which already exists
	// and holds a different chapter's pages.
	writeLocalChapter(t, seriesDir, "Chapter 1.cbz", 5)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 1.zip"), buildPageZip(t, "", 2), 0o644))

	result, err := repository.RepackLocalMangaSeries("One Piece", nil)
	require.NoError(t, err)

	require.Len(t, result.Skipped, 1)
	require.Equal(t, "Chapter 1.zip", result.Skipped[0].Name)
	require.Equal(t, LocalMangaRepackConflict, result.Skipped[0].Reason)

	// Both files survive untouched.
	require.FileExists(t, filepath.Join(seriesDir, "Chapter 1.zip"))
	require.Len(t, readZipPageNames(t, filepath.Join(seriesDir, "Chapter 1.cbz")), 5)
}

func TestRepackLocalMangaSeriesRejectsUnknownSeries(t *testing.T) {
	repository, _ := newLocalMangaTestRepository(t)

	_, err := repository.RepackLocalMangaSeries("Nonexistent", nil)
	require.ErrorIs(t, err, ErrLocalMangaSeriesNotFound)

	_, err = repository.RepackLocalMangaSeries("../escape", nil)
	require.ErrorIs(t, err, ErrInvalidLocalMangaName)
}

// TestRepackedChapterIsReadable is the check that matters: a repacked chapter
// has to still come back out of the provider as ordered, readable pages.
func TestRepackedChapterIsReadable(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 7.zip"), buildPageZip(t, "pages/", 3), 0o644))

	_, err := repository.RepackLocalMangaSeries("One Piece", nil)
	require.NoError(t, err)

	provider := manga_providers.NewLocal(root, repository.logger).(*manga_providers.Local)

	chapters, err := provider.FindChapters("One Piece")
	require.NoError(t, err)
	require.Len(t, chapters, 1)
	require.Equal(t, "7", chapters[0].Chapter)

	pages, err := provider.FindChapterPages(chapters[0].ID)
	require.NoError(t, err)
	require.Len(t, pages, 3, "the metadata document must not be served as a page")

	for _, page := range pages {
		reader, err := provider.ReadPage(page.URL)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
	}
}
