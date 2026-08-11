package chapter_downloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChapterFileNameRoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		chapterId     string
		chapterNumber string
	}{
		{"simple", "chapter-1", "1"},
		{"decimal chapter number", "chapter-13-5", "13.5"},
		{"underscore in id", "one_piece$chapter_1", "0001"},
		{"slash and dot in id", "series/ch.1", "2"},
		{"spaces and specials", "my chapter: *final*?", "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := FormatChapterFileName(tt.chapterId, tt.chapterNumber)
			require.NotContains(t, fileName, "/")

			chapterId, chapterNumber, ok := ParseChapterFileName(fileName)
			require.True(t, ok)
			require.Equal(t, tt.chapterId, chapterId)
			require.Equal(t, tt.chapterNumber, chapterNumber)
		})
	}

	_, _, ok := ParseChapterFileName("not-a-chapter.zip")
	require.False(t, ok)
	_, _, ok = ParseChapterFileName("noseparator.cbz")
	require.False(t, ok)
}

func TestSeriesDirNameRoundTrip(t *testing.T) {
	name := FormatSeriesDirName("comick", 101517)
	require.Equal(t, "comick_101517", name)

	provider, mediaId, ok := ParseSeriesDirName(name)
	require.True(t, ok)
	require.Equal(t, "comick", provider)
	require.Equal(t, 101517, mediaId)

	// Provider containing an underscore
	provider, mediaId, ok = ParseSeriesDirName("my_provider_42")
	require.True(t, ok)
	require.Equal(t, "my_provider", provider)
	require.Equal(t, 42, mediaId)

	_, _, ok = ParseSeriesDirName("nounderscore")
	require.False(t, ok)
	_, _, ok = ParseSeriesDirName("provider_notanumber")
	require.False(t, ok)
}

// writeTestChapterDir creates a staging-style directory with n 1x1 PNG pages
// and returns the matching registry.
func writeTestChapterDir(t *testing.T, dir string, n int) Registry {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, os.ModePerm))

	imageData := newTestPNG(t)
	registry := make(Registry, n)
	for i := range n {
		filename := filepath.Join(dir, pageFilename(i))
		require.NoError(t, os.WriteFile(filename, imageData, 0644))
		registry[i] = PageInfo{
			Index:    i,
			Filename: pageFilename(i),
			Size:     int64(len(imageData)),
			Width:    1,
			Height:   1,
		}
	}
	return registry
}

func pageFilename(index int) string {
	return []string{"001.png", "002.png", "003.png"}[index]
}

func TestWriteAndReadCBZ(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "staging")
	registry := writeTestChapterDir(t, srcDir, 2)

	id := DownloadID{Provider: "comick", MediaId: 1, ChapterId: "ch-1", ChapterNumber: "1"}
	info := buildComicInfo(id, "Series Title", "Chapter Title", registry)

	destPath := filepath.Join(tmp, "0001_ch-1.cbz")
	require.NoError(t, writeCBZ(destPath, srcDir, registry, info))

	// No temp file left behind
	_, err := os.Stat(destPath + ".tmp")
	require.True(t, os.IsNotExist(err))

	entries, readInfo, err := ReadCBZ(destPath)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "001.png", entries[0].Name)
	require.Equal(t, "002.png", entries[1].Name)
	require.Equal(t, 1, entries[0].Width)
	require.Equal(t, 1, entries[0].Height)

	require.NotNil(t, readInfo)
	require.Equal(t, "Series Title", readInfo.Series)
	require.Equal(t, "Chapter Title", readInfo.Title)
	require.Equal(t, "1", readInfo.Number)
	require.Equal(t, 2, readInfo.PageCount)
	require.NotNil(t, readInfo.Pages)
	require.Len(t, readInfo.Pages.Pages, 2)
}

func TestWriteCBZMissingPageFails(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "staging")
	registry := writeTestChapterDir(t, srcDir, 1)
	registry[1] = PageInfo{Index: 1, Filename: "missing.png"}

	destPath := filepath.Join(tmp, "0001_ch-1.cbz")
	require.Error(t, writeCBZ(destPath, srcDir, registry, nil))

	// Neither the archive nor its temp file may exist after a failure
	_, err := os.Stat(destPath)
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(destPath + ".tmp")
	require.True(t, os.IsNotExist(err))
}

func TestScanDownloadDirMixedLayouts(t *testing.T) {
	tmp := t.TempDir()

	// New layout: comick_1/0001_ch-1.cbz
	seriesDir := filepath.Join(tmp, FormatSeriesDirName("comick", 1))
	stagingDir := filepath.Join(tmp, "cbz-staging")
	registry := writeTestChapterDir(t, stagingDir, 1)
	require.NoError(t, os.MkdirAll(seriesDir, os.ModePerm))
	require.NoError(t, writeCBZ(filepath.Join(seriesDir, FormatChapterFileName("ch-1", "0001")), stagingDir, registry, nil))
	require.NoError(t, os.RemoveAll(stagingDir))

	// Legacy layout: mangapill_2_ch$UNDERSCORE$2_3/
	legacyName := FormatChapterDirName("mangapill", 2, "ch_2", "3")
	writeTestChapterDir(t, filepath.Join(tmp, legacyName), 1)

	// Ignored: staging dir, temp file, unrelated dir/file
	writeTestChapterDir(t, filepath.Join(tmp, stagingDirPrefix+"comick_1_x_1"), 1)
	require.NoError(t, os.WriteFile(filepath.Join(seriesDir, "0002_ch-2.cbz.tmp"), []byte("partial"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmp, "unrelated"), os.ModePerm))

	result := ScanDownloadDir(tmp)
	require.Len(t, result, 2)

	require.Equal(t, "comick_1/"+FormatChapterFileName("ch-1", "0001"), result[DownloadID{
		Provider: "comick", MediaId: 1, ChapterId: "ch-1", ChapterNumber: "0001",
	}])
	require.Equal(t, legacyName, result[DownloadID{
		Provider: "mangapill", MediaId: 2, ChapterId: "ch_2", ChapterNumber: "3",
	}])
}
