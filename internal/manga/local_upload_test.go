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

func TestUploadLocalMangaArchiveStoresFlatChapter(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	archive := buildPageZip(t, "page_", 3)
	result, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "One Piece",
		Filename: "Chapter 12.cbz",
		Source:   bytes.NewReader(archive),
	})
	require.NoError(t, err)

	require.Equal(t, "One Piece", result.Series)
	require.Equal(t, []string{"Chapter 12.cbz"}, result.Chapters)
	require.Equal(t, 3, result.PageCount)

	stored := filepath.Join(root, "One Piece", "Chapter 12.cbz")
	require.FileExists(t, stored)
	// Pages are renamed to padded positions so any reader walks them in order.
	require.Equal(t, []string{"001.png", "002.png", "003.png"}, readZipPageNames(t, stored))
	// The staging file must not survive a successful upload.
	requireNoStagingFiles(t, root)
}

func TestUploadLocalMangaArchiveFlattensWrapperFolder(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	// A single wrapper folder is the usual shape of a downloaded chapter, and it
	// names the chapter better than the uploaded filename does.
	archive := buildPageZip(t, "Chapter 5/", 2)
	result, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "Berserk",
		Filename: "download.zip",
		Source:   bytes.NewReader(archive),
	})
	require.NoError(t, err)

	require.Equal(t, []string{"Chapter 5.cbz"}, result.Chapters)
	require.Equal(t, []string{"001.png", "002.png"}, readZipPageNames(t, filepath.Join(root, "Berserk", "Chapter 5.cbz")))
}

func TestUploadLocalMangaArchiveSplitsChapterFolders(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	page := pngBytes(t)
	archive := buildZip(t, map[string][]byte{
		"Berserk/Chapter 1/001.png": page,
		"Berserk/Chapter 1/002.png": page,
		"Berserk/Chapter 2/001.png": page,
		"Berserk/ComicInfo.xml":     []byte("<ComicInfo/>"),
		"__MACOSX/._001.png":        page,
	})

	result, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "Berserk",
		Filename: "Berserk.zip",
		Source:   bytes.NewReader(archive),
	})
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"Chapter 1.cbz", "Chapter 2.cbz"}, result.Chapters)
	require.Equal(t, 3, result.PageCount)
	require.Equal(t, []string{"001.png", "002.png"}, readZipPageNames(t, filepath.Join(root, "Berserk", "Chapter 1.cbz")))
	require.Equal(t, []string{"001.png"}, readZipPageNames(t, filepath.Join(root, "Berserk", "Chapter 2.cbz")))
}

func TestUploadLocalMangaArchiveOrdersPagesNaturally(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	// Distinct page bytes so the stored order can be verified, not just the names.
	pages := map[string][]byte{
		"2.png":  []byte("\x89PNG\r\n\x1a\nTWO"),
		"10.png": []byte("\x89PNG\r\n\x1a\nTEN"),
		"1.png":  []byte("\x89PNG\r\n\x1a\nONE"),
	}

	_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "Naruto",
		Filename: "Chapter 1.cbz",
		Source:   bytes.NewReader(buildZip(t, pages)),
	})
	require.NoError(t, err)

	contents := readZipEntryContents(t, filepath.Join(root, "Naruto", "Chapter 1.cbz"))
	require.Equal(t, []string{"001.png", "002.png", "003.png"}, contents.names)
	require.Equal(t, [][]byte{pages["1.png"], pages["2.png"], pages["10.png"]}, contents.data)
}

func TestUploadLocalMangaArchiveWritesComicInfo(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:     "One Piece",
		Filename:   "Chapter 12.cbz",
		Source:     bytes.NewReader(buildPageZip(t, "", 2)),
		MediaId:    21,
		Collection: newLocalMangaCollection(t, map[int]string{21: "One Piece"}),
	})
	require.NoError(t, err)

	info := readComicInfo(t, filepath.Join(root, "One Piece", "Chapter 12.cbz"))

	// The series name comes from the mapped AniList entry, not the folder.
	require.Equal(t, "One Piece", info.Series)
	require.Equal(t, "12", info.Number)
	require.Equal(t, "Chapter 12", info.Title)
	require.Equal(t, chapter_downloader.MangaRightToLeft, info.Manga)
	require.Equal(t, 2, info.PageCount)
	require.NotNil(t, info.Pages)
	require.Len(t, info.Pages.Pages, 2)

	// Page geometry is measured from what was written, so it has to be real.
	for index, page := range info.Pages.Pages {
		require.Equal(t, index, page.Image)
		require.Equal(t, 2, page.ImageWidth)
		require.Equal(t, 2, page.ImageHeight)
		require.Positive(t, page.ImageSize)
	}
}

func TestUploadLocalMangaArchiveWritesComicInfoWithoutAMapping(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "[Group] Vinland.Saga",
		Filename: "Vol 2 Chapter 7 - The Journey Begins.cbz",
		Source:   bytes.NewReader(buildPageZip(t, "", 1)),
	})
	require.NoError(t, err)

	info := readComicInfo(t, filepath.Join(root, "[Group] Vinland.Saga", "Vol 2 Chapter 7 - The Journey Begins.cbz"))

	// With nothing mapped, the folder name still has to yield a usable series.
	require.Equal(t, "Vinland Saga", info.Series)
	require.Equal(t, "7", info.Number)
	require.Equal(t, "2", info.Volume)
	require.Equal(t, "The Journey Begins", info.Title)
}

func TestUploadLocalMangaArchiveRejectsBadArchives(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	t.Run("not a zip", func(t *testing.T) {
		_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
			Series:   "One Piece",
			Filename: "chapter.cbz",
			Source:   bytes.NewReader([]byte("this is not a zip file")),
		})
		require.ErrorIs(t, err, ErrLocalMangaUploadNotAnArchive)
	})

	t.Run("no page images", func(t *testing.T) {
		archive := buildZip(t, map[string][]byte{"notes.txt": []byte("hello")})
		_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
			Series:   "One Piece",
			Filename: "chapter.cbz",
			Source:   bytes.NewReader(archive),
		})
		require.ErrorIs(t, err, ErrLocalMangaUploadNoPages)
	})

	t.Run("traversal entry", func(t *testing.T) {
		archive := buildZip(t, map[string][]byte{"../../escaped.png": pngBytes(t)})
		_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
			Series:   "One Piece",
			Filename: "chapter.cbz",
			Source:   bytes.NewReader(archive),
		})
		require.ErrorIs(t, err, ErrLocalMangaUploadUnsafeEntry)
	})

	t.Run("invalid series name", func(t *testing.T) {
		_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
			Series:   "../escape",
			Filename: "chapter.cbz",
			Source:   bytes.NewReader(buildPageZip(t, "", 1)),
		})
		require.ErrorIs(t, err, ErrInvalidLocalMangaName)
	})

	// A rejected upload must leave nothing behind.
	requireNoStagingFiles(t, root)
	require.NoDirExists(t, filepath.Join(root, "One Piece"))
}

func TestUploadLocalMangaArchiveHonoursOverwrite(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	upload := func(pages int, overwrite bool) error {
		_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
			Series:    "One Piece",
			Filename:  "Chapter 1.cbz",
			Source:    bytes.NewReader(buildPageZip(t, "", pages)),
			Overwrite: overwrite,
		})
		return err
	}

	require.NoError(t, upload(2, false))
	require.ErrorIs(t, upload(4, false), ErrLocalMangaChapterExists)
	// The refused upload must not have touched the stored chapter.
	require.Len(t, readZipPageNames(t, filepath.Join(root, "One Piece", "Chapter 1.cbz")), 2)

	require.NoError(t, upload(4, true))
	require.Len(t, readZipPageNames(t, filepath.Join(root, "One Piece", "Chapter 1.cbz")), 4)
}

func TestUploadLocalMangaArchiveMapsMedia(t *testing.T) {
	repository, _ := newLocalMangaTestRepository(t)

	result, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "One Piece",
		Filename: "Chapter 1.cbz",
		Source:   bytes.NewReader(buildPageZip(t, "", 1)),
		MediaId:  21,
	})
	require.NoError(t, err)
	require.NotNil(t, result.MediaId)
	require.Equal(t, 21, *result.MediaId)

	mapping, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, 21)
	require.True(t, found)
	require.Equal(t, "One Piece", mapping.MangaID)
}

// TestUploadedChapterIsReadable is the end-to-end check that matters: an
// uploaded archive has to come back out of the provider as ordered pages.
func TestUploadedChapterIsReadable(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	_, err := repository.UploadLocalMangaArchive(&LocalMangaUploadOptions{
		Series:   "One Piece",
		Filename: "Chapter 3.cbz",
		Source:   bytes.NewReader(buildPageZip(t, "scan_", 3)),
	})
	require.NoError(t, err)

	provider := manga_providers.NewLocal(root, repository.logger).(*manga_providers.Local)

	chapters, err := provider.FindChapters("One Piece")
	require.NoError(t, err)
	require.Len(t, chapters, 1)
	require.Equal(t, "3", chapters[0].Chapter)

	pages, err := provider.FindChapterPages(chapters[0].ID)
	require.NoError(t, err)
	require.Len(t, pages, 3)

	for index, page := range pages {
		require.Equal(t, index, page.Index)
		reader, err := provider.ReadPage(page.URL)
		require.NoError(t, err, "page %d should be readable", index)
		require.NoError(t, reader.Close())
	}
}

func TestComparePageNames(t *testing.T) {
	require.Negative(t, comparePageNames("2.png", "10.png"))
	require.Negative(t, comparePageNames("001.png", "002.png"))
	require.Zero(t, comparePageNames("01.png", "1.png"))
	require.Negative(t, comparePageNames("page_9.jpg", "page_10.jpg"))
	require.Positive(t, comparePageNames("b.png", "a.png"))
}

func TestNormalizeArchiveEntryName(t *testing.T) {
	valid := map[string]string{
		"001.png":             "001.png",
		"Chapter 1/001.png":   "Chapter 1/001.png",
		`Chapter 1\001.png`:   "Chapter 1/001.png",
		"./Chapter 1/001.png": "Chapter 1/001.png",
		"Chapter 1//001.png":  "Chapter 1/001.png",
		"Chapter 1/./001.png": "Chapter 1/001.png",
		"Vol 1/Ch 1/001.png":  "Vol 1/Ch 1/001.png",
	}
	for input, expected := range valid {
		got, ok := normalizeArchiveEntryName(input)
		require.True(t, ok, "input %q", input)
		require.Equal(t, expected, got, "input %q", input)
	}

	invalid := []string{"", "/abs/001.png", "../001.png", "a/../../b.png", `C:/001.png`, "a\x00b.png"}
	for _, input := range invalid {
		_, ok := normalizeArchiveEntryName(input)
		require.False(t, ok, "input %q should be rejected", input)
	}
}

func TestCommonArchivePrefix(t *testing.T) {
	require.Equal(t, "", commonArchivePrefix(nil))
	require.Equal(t, "Series", commonArchivePrefix([]string{"Series"}))
	require.Equal(t, "Series", commonArchivePrefix([]string{"Series/Ch 1", "Series/Ch 2"}))
	require.Equal(t, "", commonArchivePrefix([]string{"Ch 1", "Ch 2"}))
	require.Equal(t, "", commonArchivePrefix([]string{".", "Ch 1"}))
}

type zipContents struct {
	names []string
	data  [][]byte
}

func readZipEntryContents(t *testing.T, path string) zipContents {
	t.Helper()

	zipReader, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer zipReader.Close()

	contents := zipContents{}
	for _, file := range zipReader.File {
		if file.Name == chapter_downloader.ComicInfoFilename {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		data := new(bytes.Buffer)
		_, err = data.ReadFrom(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())

		contents.names = append(contents.names, file.Name)
		contents.data = append(contents.data, data.Bytes())
	}

	return contents
}

// requireNoStagingFiles asserts that no partial upload was left in the library
// root.
func requireNoStagingFiles(t *testing.T, root string) {
	t.Helper()

	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	for _, entry := range entries {
		require.NotContains(t, entry.Name(), localMangaUploadStagingPrefix,
			"upload staging file was left behind: %s", entry.Name())
	}
}
