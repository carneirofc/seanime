package chapter_downloader

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"seanime/internal/database/db"
	"seanime/internal/events"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"seanime/internal/testutil"

	"github.com/stretchr/testify/require"
)

func newTestDownloader(t *testing.T) (*Downloader, *db.Database, string) {
	t.Helper()

	env := testutil.NewTestEnv(t)
	logger := env.Logger()
	database := env.MustNewDatabase(logger)
	downloadDir := env.MustMkdir("downloads")

	downloader := NewDownloader(&NewDownloaderOptions{
		Logger:         logger,
		WSEventManager: events.NewMockWSEventManager(logger),
		Database:       database,
		DownloadDir:    downloadDir,
	})

	return downloader, database, downloadDir
}

func newTestPNG(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 128, B: 64, A: 255})
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

func TestQueueAddsItemToDatabase(t *testing.T) {
	downloader, database, _ := newTestDownloader(t)

	pages := []*hibikemanga.ChapterPage{{
		Index: 0,
		URL:   "https://example.com/01.png",
	}}
	id := DownloadID{
		Provider:      "test-provider",
		MediaId:       101517,
		ChapterId:     "chapter-1",
		ChapterNumber: "1",
	}

	err := downloader.AddToQueue(DownloadOptions{
		DownloadID:   id,
		Pages:        pages,
		MediaTitle:   "Test Manga",
		ChapterTitle: "Chapter 1 - Beginnings",
		StartNow:     false,
	})
	require.NoError(t, err)

	next, err := database.GetNextChapterDownloadQueueItem()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Equal(t, id.Provider, next.Provider)
	require.Equal(t, id.MediaId, next.MediaID)
	require.Equal(t, id.ChapterId, next.ChapterID)
	require.Equal(t, id.ChapterNumber, next.ChapterNumber)
	require.Equal(t, "Test Manga", next.MediaTitle)
	require.Equal(t, "Chapter 1 - Beginnings", next.ChapterTitle)
	require.Equal(t, string(QueueStatusNotStarted), next.Status)
	require.NotEmpty(t, next.PageData)
}

func TestDownloadChapterImagesWritesCBZ(t *testing.T) {
	downloader, database, downloadDir := newTestDownloader(t)

	imageData := newTestPNG(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(imageData)
	}))
	defer server.Close()

	pages := []*hibikemanga.ChapterPage{{
		Index: 0,
		URL:   server.URL + "/page.png",
	}}
	id := DownloadID{
		Provider:      "test-provider",
		MediaId:       101517,
		ChapterId:     "chapter-1",
		ChapterNumber: "1",
	}

	err := downloader.AddToQueue(DownloadOptions{
		DownloadID:   id,
		Pages:        pages,
		MediaTitle:   "Test Manga",
		ChapterTitle: "Chapter 1",
		StartNow:     false,
	})
	require.NoError(t, err)

	require.NoError(t, database.UpdateChapterDownloadQueueItemStatus(id.Provider, id.MediaId, id.ChapterId, string(QueueStatusDownloading)))
	downloader.queue.current = &QueueInfo{
		DownloadID:   id,
		Pages:        pages,
		Status:       QueueStatusDownloading,
		MediaTitle:   "Test Manga",
		ChapterTitle: "Chapter 1",
	}

	err = downloader.downloadChapterImages(downloader.queue.current)
	require.NoError(t, err)

	cbzPath := filepath.Join(downloadDir, FormatSeriesDirName(id.Provider, id.MediaId), FormatChapterFileName(id.ChapterId, id.ChapterNumber))
	entries, info, err := ReadCBZ(cbzPath)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "001.png", entries[0].Name)
	require.Equal(t, 1, entries[0].Width)
	require.Equal(t, 1, entries[0].Height)

	require.NotNil(t, info)
	require.Equal(t, "Test Manga", info.Series)
	require.Equal(t, "Chapter 1", info.Title)
	require.Equal(t, id.ChapterNumber, info.Number)
	require.Equal(t, 1, info.PageCount)

	// Staging directory must be gone
	_, err = os.Stat(downloader.getChapterStagingDir(id))
	require.True(t, os.IsNotExist(err))

	queueItems, err := database.GetChapterDownloadQueue()
	require.NoError(t, err)
	require.Empty(t, queueItems)
}
