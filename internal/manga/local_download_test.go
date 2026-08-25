package manga

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLocalMangaChapters(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	writeLocalChapter(t, seriesDir, "Chapter 2.cbz", 2)
	writeLocalChapter(t, seriesDir, "Chapter 10.cbz", 1)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 1.zip"), buildPageZip(t, "", 3), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 3.cbr"), []byte("Rar!"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "cover.jpg"), []byte("not a chapter"), 0o644))

	chapters, err := repository.GetLocalMangaChapters("One Piece")
	require.NoError(t, err)

	// Ordered by chapter number, not by filename: chapter 10 comes last.
	names := make([]string, 0, len(chapters))
	for _, chapter := range chapters {
		names = append(names, chapter.Filename)
	}
	require.Equal(t, []string{"Chapter 1.zip", "Chapter 2.cbz", "Chapter 3.cbr", "Chapter 10.cbz"}, names)

	byName := make(map[string]*LocalMangaChapter, len(chapters))
	for _, chapter := range chapters {
		byName[chapter.Filename] = chapter
	}

	require.Equal(t, "zip", byName["Chapter 1.zip"].Format)
	require.True(t, byName["Chapter 1.zip"].Downloadable)
	require.False(t, byName["Chapter 1.zip"].HasMetadata)

	require.Equal(t, "cbz", byName["Chapter 2.cbz"].Format)
	require.Equal(t, "2", byName["Chapter 2.cbz"].Number)
	require.Positive(t, byName["Chapter 2.cbz"].Size)

	// A format Seanime cannot read is listed but not offered.
	require.Equal(t, "cbr", byName["Chapter 3.cbr"].Format)
	require.False(t, byName["Chapter 3.cbr"].Downloadable)
}

func TestGetLocalMangaChaptersReportsExistingMetadata(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "One Piece",
		Filename: "Chapter 1.cbz",
		Source:   bytes.NewReader(buildPageZip(t, "", 1)),
	})
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(root, "One Piece"))

	chapters, err := repository.GetLocalMangaChapters("One Piece")
	require.NoError(t, err)
	require.Len(t, chapters, 1)
	require.True(t, chapters[0].HasMetadata, "an uploaded chapter is written with metadata")
}

func TestGetLocalMangaChaptersRejectsBadInput(t *testing.T) {
	repository, _ := newLocalMangaTestRepository(t)

	_, err := repository.GetLocalMangaChapters("Nonexistent")
	require.ErrorIs(t, err, ErrLocalMangaSeriesNotFound)

	_, err = repository.GetLocalMangaChapters("../escape")
	require.ErrorIs(t, err, ErrInvalidLocalMangaName)
}

func TestWriteLocalMangaChapterCBZNormalizesOnTheWayOut(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	// An untouched hand-made chapter: plain .zip, nested, unpadded, with junk.
	require.NoError(t, os.MkdirAll(seriesDir, 0o755))
	page := pngBytes(t)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 4.zip"), buildZip(t, map[string][]byte{
		"scan/2.png":  page,
		"scan/10.png": page,
		"scan/1.png":  page,
		"readme.txt":  []byte("junk"),
	}), 0o644))
	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 21, "One Piece"))

	buffer := new(bytes.Buffer)
	require.NoError(t, repository.WriteLocalMangaChapterCBZ(
		buffer, "One Piece", "Chapter 4.zip",
		newLocalMangaCollection(t, map[int]string{21: "One Piece"}),
	))

	names, info := readCBZBuffer(t, buffer)
	require.Equal(t, []string{"001.png", "002.png", "003.png"}, names)

	require.NotNil(t, info)
	require.Equal(t, "One Piece", info.Series)
	require.Equal(t, "4", info.Number)
	require.Equal(t, 3, info.PageCount)

	// The stored file must be exactly as it was: the download normalizes a copy.
	require.FileExists(t, filepath.Join(seriesDir, "Chapter 4.zip"))
	require.Equal(t, []string{"readme.txt", "scan/1.png", "scan/10.png", "scan/2.png"},
		readZipEntryNames(t, filepath.Join(seriesDir, "Chapter 4.zip")))
}

func TestWriteLocalMangaChapterCBZFromImageDirectory(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	chapterDir := filepath.Join(root, "One Piece", "Chapter 5")

	require.NoError(t, os.MkdirAll(chapterDir, 0o755))
	page := pngBytes(t)
	for _, name := range []string{"1.png", "2.png", "10.png"} {
		require.NoError(t, os.WriteFile(filepath.Join(chapterDir, name), page, 0o644))
	}

	buffer := new(bytes.Buffer)
	require.NoError(t, repository.WriteLocalMangaChapterCBZ(buffer, "One Piece", "Chapter 5", nil))

	names, info := readCBZBuffer(t, buffer)
	require.Equal(t, []string{"001.png", "002.png", "003.png"}, names)
	require.NotNil(t, info)
	require.Equal(t, "5", info.Number)
}

func TestWriteLocalMangaChapterCBZRejectsBadInput(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 1)

	buffer := new(bytes.Buffer)

	require.ErrorIs(t,
		repository.WriteLocalMangaChapterCBZ(buffer, "One Piece", "Missing.cbz", nil),
		ErrLocalMangaChapterNotFound)

	// A chapter name is a single path element, never a way out of the series.
	require.ErrorIs(t,
		repository.WriteLocalMangaChapterCBZ(buffer, "One Piece", "../One Piece/Chapter 1.cbz", nil),
		ErrInvalidLocalMangaName)
	require.ErrorIs(t,
		repository.WriteLocalMangaChapterCBZ(buffer, "One Piece", "sub/Chapter 1.cbz", nil),
		ErrInvalidLocalMangaName)
}

func TestWriteLocalMangaSeriesArchive(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	writeLocalChapter(t, seriesDir, "Chapter 1.cbz", 2)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 2.zip"), buildPageZip(t, "", 1), 0o644))
	// Neither of these can be served, and neither may break the export.
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 3.cbr"), []byte("Rar!"), 0o644))

	buffer := new(bytes.Buffer)
	count, err := repository.WriteLocalMangaSeriesArchive(buffer, "One Piece", nil)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	require.NoError(t, err)

	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	// Every entry is offered as a CBZ, whatever it was stored as.
	require.Equal(t, []string{"Chapter 1.cbz", "Chapter 2.cbz"}, names)

	// The entries must be real CBZ archives, not just renamed sources.
	inner, err := reader.File[1].Open()
	require.NoError(t, err)
	defer inner.Close()
	data := new(bytes.Buffer)
	_, err = data.ReadFrom(inner)
	require.NoError(t, err)

	innerNames, innerInfo := readCBZBuffer(t, data)
	require.Equal(t, []string{"001.png"}, innerNames)
	require.NotNil(t, innerInfo)
	require.Equal(t, "2", innerInfo.Number)
}

func TestLocalMangaChapterDownloadName(t *testing.T) {
	require.Equal(t, "Chapter 1.cbz", LocalMangaChapterDownloadName("Chapter 1.zip"))
	require.Equal(t, "Chapter 1.cbz", LocalMangaChapterDownloadName("Chapter 1.cbz"))
	require.Equal(t, "Chapter 1.cbz", LocalMangaChapterDownloadName("Chapter 1"))
	require.Equal(t, "chapter.cbz", LocalMangaChapterDownloadName(""))
}

// readCBZBuffer splits a generated CBZ into its page names and its metadata.
func readCBZBuffer(t *testing.T, buffer *bytes.Buffer) ([]string, *chapter_downloader.ComicInfo) {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	require.NoError(t, err)

	names := make([]string, 0, len(reader.File))
	var info *chapter_downloader.ComicInfo

	for _, file := range reader.File {
		if file.Name == chapter_downloader.ComicInfoFilename {
			rc, err := file.Open()
			require.NoError(t, err)
			info, err = chapter_downloader.ParseComicInfo(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			continue
		}
		names = append(names, file.Name)
	}

	return names, info
}

func TestCheckLocalMangaChapterDownload(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)
	seriesDir := filepath.Join(root, "One Piece")

	writeLocalChapter(t, seriesDir, "Chapter 1.cbz", 1)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 2.cbz"), []byte("not a zip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "Chapter 3.zip"), buildZip(t, map[string][]byte{
		"notes.txt": []byte("no pages"),
	}), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(seriesDir, "Chapter 4"), 0o755))

	// Everything the download would reject has to be rejected here instead, while
	// a status code can still be sent.
	require.NoError(t, repository.CheckLocalMangaChapterDownload("One Piece", "Chapter 1.cbz"))
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("One Piece", "Chapter 2.cbz"), ErrLocalMangaUploadNotAnArchive)
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("One Piece", "Chapter 3.zip"), ErrLocalMangaUploadNoPages)
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("One Piece", "Chapter 4"), ErrLocalMangaUploadNoPages)
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("One Piece", "Missing.cbz"), ErrLocalMangaChapterNotFound)
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("One Piece", "../escape"), ErrInvalidLocalMangaName)
	// A chapter of a series that does not exist is simply a chapter that is not
	// there; both answers are a 404 either way.
	require.ErrorIs(t, repository.CheckLocalMangaChapterDownload("Nonexistent", "Chapter 1.cbz"), ErrLocalMangaChapterNotFound)
}
