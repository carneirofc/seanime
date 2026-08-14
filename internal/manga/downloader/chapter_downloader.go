package chapter_downloader

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"seanime/internal/database/db"
	"seanime/internal/events"
	hibikemanga "seanime/internal/extension/hibike/manga"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	_ "golang.org/x/image/bmp"  // Register BMP format
	_ "golang.org/x/image/tiff" // Register Tiff format
	_ "golang.org/x/image/webp" // Register WebP format (common on modern manga providers)
)

// pageDownloadRetries is the number of times a single page download is retried
// before the page (and therefore the chapter) is considered failed.
const pageDownloadRetries = 3

// 📁 cache/manga
// └── 📁 {provider}_{mediaId}                                  <- Series directory
//     └── 📄 {chapterNumber}_{chapterId}.cbz                   <- One CBZ archive per chapter
//         ├── 📄 001.jpg
//         ├── 📄 002.jpg
//         ├── 📄 ...
//         └── 📄 ComicInfo.xml                                 <- Standard CBZ metadata
//
// Legacy layout (pre-CBZ), still readable and migrated on startup:
// └── 📁 {provider}_{mediaId}_{chapterId}_{chapterNumber}
//     ├── 📄 registry.json
//     ├── 📄 1.jpg
//     └── 📄 ...
//

type (
	// Downloader is used to download chapters from various manga providers.
	Downloader struct {
		logger         *zerolog.Logger
		wsEventManager events.WSEventManagerInterface
		database       *db.Database
		downloadDir    string
		mu             sync.Mutex
		downloadMu     sync.Mutex
		// ctx/cancel control cancellation of in-flight downloads.
		// They are guarded by mu. cancel() is called by Stop() and a fresh
		// context is created by Run() once the previous one has been cancelled.
		ctx                 context.Context
		cancel              context.CancelFunc
		queue               *Queue
		runCh               chan *QueueInfo // Receives a signal to download the next item
		chapterDownloadedCh chan DownloadID // Sends a signal when a chapter has been downloaded
	}

	//+-------------------------------------------------------------------------------------------------------------------+

	DownloadID struct {
		Provider      string `json:"provider"`
		MediaId       int    `json:"mediaId"`
		ChapterId     string `json:"chapterId"`
		ChapterNumber string `json:"chapterNumber"`
	}

	//+-------------------------------------------------------------------------------------------------------------------+

	// Registry stored in 📄 registry.json for each chapter download.
	Registry map[int]PageInfo

	PageInfo struct {
		Index       int    `json:"index"`
		Filename    string `json:"filename"`
		OriginalURL string `json:"original_url"`
		Size        int64  `json:"size"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
	}
)

type (
	NewDownloaderOptions struct {
		Logger         *zerolog.Logger
		WSEventManager events.WSEventManagerInterface
		DownloadDir    string
		Database       *db.Database
	}

	DownloadOptions struct {
		DownloadID
		Pages []*hibikemanga.ChapterPage
		// MediaTitle and ChapterTitle feed the ComicInfo.xml metadata.
		// They may be empty.
		MediaTitle   string
		ChapterTitle string
		StartNow     bool
	}
)

func NewDownloader(opts *NewDownloaderOptions) *Downloader {
	runCh := make(chan *QueueInfo, 1)
	ctx, cancel := context.WithCancel(context.Background())

	d := &Downloader{
		logger:              opts.Logger,
		wsEventManager:      opts.WSEventManager,
		downloadDir:         opts.DownloadDir,
		ctx:                 ctx,
		cancel:              cancel,
		runCh:               runCh,
		queue:               NewQueue(opts.Database, opts.Logger, opts.WSEventManager, runCh),
		chapterDownloadedCh: make(chan DownloadID, 100),
	}

	return d
}

// currentCtx returns the active cancellation context for in-flight downloads.
func (cd *Downloader) currentCtx() context.Context {
	cd.mu.Lock()
	defer cd.mu.Unlock()
	if cd.ctx == nil {
		return context.Background()
	}
	return cd.ctx
}

// Start spins up a goroutine that will listen to queue events.
func (cd *Downloader) Start() {
	go func() {
		for {
			select {
			// Listen for new queue items
			case queueInfo := <-cd.runCh:
				cd.logger.Debug().Msgf("chapter downloader: Received queue item to download: %s", queueInfo.ChapterId)
				cd.run(queueInfo)
			}
		}
	}()
}

func (cd *Downloader) ChapterDownloaded() <-chan DownloadID {
	return cd.chapterDownloadedCh
}

// AddToQueue adds a chapter to the download queue.
// If the chapter is already downloaded (CBZ archive or legacy folder), it will delete the previous data and re-download it.
func (cd *Downloader) AddToQueue(opts DownloadOptions) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	downloadId := opts.DownloadID

	// Check if chapter is already downloaded
	cbzPath := cd.getChapterCBZPath(downloadId)
	if _, err := os.Stat(cbzPath); err == nil {
		cd.logger.Warn().Msg("chapter downloader: chapter archive already exists, deleting")
		_ = os.Remove(cbzPath)
	}
	legacyDir := cd.getChapterDownloadDir(downloadId)
	if _, err := os.Stat(legacyDir); err == nil {
		cd.logger.Warn().Msg("chapter downloader: legacy chapter directory already exists, deleting")
		_ = os.RemoveAll(legacyDir)
	}

	// Start download
	cd.logger.Debug().Msgf("chapter downloader: Adding chapter to download queue: %s", opts.ChapterId)
	// Add to queue
	return cd.queue.Add(downloadId, opts.Pages, opts.MediaTitle, opts.ChapterTitle, opts.StartNow)
}

// DeleteChapter deletes a downloaded chapter (CBZ archive or legacy folder) from the download directory.
func (cd *Downloader) DeleteChapter(id DownloadID) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.logger.Debug().Msgf("chapter downloader: Deleting chapter %s", id.ChapterId)

	_ = os.Remove(cd.getChapterCBZPath(id))
	_ = os.RemoveAll(cd.getChapterDownloadDir(id))

	// Remove the series directory once its last chapter is gone
	seriesDir := filepath.Join(cd.downloadDir, FormatSeriesDirName(id.Provider, id.MediaId))
	if entries, err := os.ReadDir(seriesDir); err == nil && len(entries) == 0 {
		_ = os.Remove(seriesDir)
	}

	cd.logger.Debug().Msgf("chapter downloader: Removed chapter %s", id.ChapterId)
	return nil
}

// Run starts the downloader if it's not already running.
func (cd *Downloader) Run() {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.logger.Debug().Msg("chapter downloader: Starting queue")

	// Ensure a live (non-cancelled) context exists. A previous Stop() may have
	// cancelled it, in which case we create a fresh one for upcoming downloads.
	if cd.ctx == nil || cd.ctx.Err() != nil {
		cd.ctx, cd.cancel = context.WithCancel(context.Background())
	}

	cd.queue.Run()
}

// Stop cancels in-flight downloads and stops the queue from running.
func (cd *Downloader) Stop() {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	if cd.cancel != nil {
		cd.cancel() // Cancel in-flight downloads
	}

	cd.queue.Stop()
}

// run downloads the chapter based on the QueueInfo provided.
// This is called successively for each current item being processed.
// It invokes downloadChapterImages to download the chapter pages.
func (cd *Downloader) run(queueInfo *QueueInfo) {

	defer util.HandlePanicInModuleThen("internal/manga/downloader/runNext", func() {
		cd.logger.Error().Msg("chapter downloader: Panic in 'run'")
	})

	// Download chapter images
	if err := cd.downloadChapterImages(queueInfo); err != nil {
		return
	}

	cd.chapterDownloadedCh <- queueInfo.DownloadID
}

// downloadChapterImages downloads each page image to a hidden staging directory,
// then packs them into a CBZ archive (pages + ComicInfo.xml).
//
//	e.g.,
//	📁 {provider}_{mediaId}
//	   └── 📄 {chapterNumber}_{chapterId}.cbz
func (cd *Downloader) downloadChapterImages(queueInfo *QueueInfo) (err error) {

	// Create staging directory
	// 📁 .downloading_{provider}_{mediaId}_{chapterId}_{chapterNumber}
	destination := cd.getChapterStagingDir(queueInfo.DownloadID)
	_ = os.RemoveAll(destination) // clear leftovers from a previous interrupted run
	if err = os.MkdirAll(destination, os.ModePerm); err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create staging directory for chapter %s", queueInfo.ChapterId)
		return err
	}

	cd.logger.Debug().Msgf("chapter downloader: Downloading chapter %s images to %s", queueInfo.ChapterId, destination)

	registry := make(Registry)

	// calculateBatchSize calculates the batch size based on the number of URLs.
	calculateBatchSize := func(numURLs int) int {
		maxBatchSize := 5
		batchSize := numURLs / 10
		if batchSize < 1 {
			return 1
		} else if batchSize > maxBatchSize {
			return maxBatchSize
		}
		return batchSize
	}

	// Download images
	batchSize := calculateBatchSize(len(queueInfo.Pages))

	// Snapshot the cancellation context once so all page goroutines observe the
	// same channel (avoids a data race on the downloader's ctx field).
	ctx := cd.currentCtx()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, batchSize) // Semaphore to control concurrency
	for _, page := range queueInfo.Pages {
		semaphore <- struct{}{} // Acquire semaphore
		wg.Add(1)
		go func(page *hibikemanga.ChapterPage, registry *Registry) {
			defer func() {
				<-semaphore // Release semaphore
				wg.Done()
			}()
			select {
			case <-ctx.Done():
				//cd.logger.Warn().Msg("chapter downloader: Download goroutine canceled")
				return
			default:
				cd.downloadPage(page, destination, registry)
			}
		}(page, &registry)
	}
	wg.Wait()

	// Pack the staged pages into the chapter CBZ archive
	_ = cd.finalizeChapter(queueInfo, destination, registry)

	cd.queue.HasCompleted(queueInfo)

	if queueInfo.Status != QueueStatusErrored {
		cd.logger.Info().Msgf("chapter downloader: Finished downloading chapter %s", queueInfo.ChapterId)
	}

	if queueInfo.Status == QueueStatusErrored {
		return fmt.Errorf("chapter downloader: Failed to download chapter %s", queueInfo.ChapterId)
	}

	return
}

// downloadPage downloads a single page from the URL and saves it to the destination directory.
// It also updates the Registry with the page information.
func (cd *Downloader) downloadPage(page *hibikemanga.ChapterPage, destination string, registry *Registry) {

	defer util.HandlePanicInModuleThen("manga/downloader/downloadImage", func() {
	})

	// Download image from URL

	imgID := fmt.Sprintf("%03d", page.Index+1)

	// Retry transient failures so a single network blip doesn't fail the whole chapter.
	buf, err := manga_providers.GetImageByProxyWithRetry(page.URL, page.Headers, pageDownloadRetries)
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to get image from URL %s", page.URL)
		return
	}

	// Get the image format
	config, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to decode image format from URL %s", page.URL)
		return
	}

	filename := imgID + "." + format

	// Create the file
	filePath := filepath.Join(destination, filename)
	file, err := os.Create(filePath)
	if err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create file for image %s", imgID)
		return
	}
	defer file.Close()

	// Copy the image data to the file
	_, err = io.Copy(file, bytes.NewReader(buf))
	if err != nil {
		cd.logger.Error().Err(err).Msgf("image downloader: Failed to write image data to file for image from %s", page.URL)
		return
	}

	// Update registry
	cd.downloadMu.Lock()
	(*registry)[page.Index] = PageInfo{
		Index:       page.Index,
		Width:       config.Width,
		Height:      config.Height,
		Filename:    filename,
		OriginalURL: page.URL,
		Size:        int64(len(buf)),
	}
	cd.downloadMu.Unlock()

	return
}

////////////////////////

// finalizeChapter validates that every page was downloaded to the staging
// directory and packs the pages into the chapter's CBZ archive.
// The staging directory is always removed; on failure the queue item is marked errored.
func (cd *Downloader) finalizeChapter(queueInfo *QueueInfo, stagingDir string, registry Registry) (err error) {

	defer util.HandlePanicInModuleThen("manga/downloader/finalizeChapter", func() {
		err = fmt.Errorf("chapter downloader: Failed to create chapter archive")
		queueInfo.Status = QueueStatusErrored
		_ = os.RemoveAll(stagingDir)
	})

	// Verify all images have been downloaded
	allDownloaded := true
	for _, page := range queueInfo.Pages {
		if _, ok := registry[page.Index]; !ok {
			allDownloaded = false
			break
		}
	}

	if !allDownloaded {
		cd.logger.Error().Msg("chapter downloader: Not all images have been downloaded, aborting")
		queueInfo.Status = QueueStatusErrored
		_ = os.RemoveAll(stagingDir)
		return fmt.Errorf("chapter downloader: Not all images have been downloaded, operation aborted")
	}

	cbzPath := cd.getChapterCBZPath(queueInfo.DownloadID)
	if err = os.MkdirAll(filepath.Dir(cbzPath), os.ModePerm); err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to create series directory for chapter %s", queueInfo.ChapterId)
		queueInfo.Status = QueueStatusErrored
		_ = os.RemoveAll(stagingDir)
		return err
	}

	info := buildComicInfo(queueInfo.DownloadID, queueInfo.MediaTitle, queueInfo.ChapterTitle, registry)
	if err = writeCBZ(cbzPath, stagingDir, registry, info); err != nil {
		cd.logger.Error().Err(err).Msgf("chapter downloader: Failed to write chapter archive for chapter %s", queueInfo.ChapterId)
		queueInfo.Status = QueueStatusErrored
		_ = os.RemoveAll(stagingDir)
		return err
	}

	_ = os.RemoveAll(stagingDir)
	return nil
}

func (cd *Downloader) getChapterDownloadDir(downloadId DownloadID) string {
	return filepath.Join(cd.downloadDir, FormatChapterDirName(downloadId.Provider, downloadId.MediaId, downloadId.ChapterId, downloadId.ChapterNumber))
}

func FormatChapterDirName(provider string, mediaId int, chapterId string, chapterNumber string) string {
	return fmt.Sprintf("%s_%d_%s_%s", provider, mediaId, EscapeChapterID(chapterId), chapterNumber)
}

// ParseChapterDirName parses a chapter directory name and returns the DownloadID.
// e.g. comick_1234_chapter$UNDERSCORE$id_13.5 -> {Provider: "comick", MediaId: 1234, ChapterId: "chapter_id", ChapterNumber: "13.5"}
func ParseChapterDirName(dirName string) (id DownloadID, ok bool) {
	parts := strings.Split(dirName, "_")
	if len(parts) != 4 {
		return id, false
	}

	id.Provider = parts[0]
	var err error
	id.MediaId, err = strconv.Atoi(parts[1])
	if err != nil {
		return id, false
	}
	id.ChapterId = UnescapeChapterID(parts[2])
	id.ChapterNumber = parts[3]

	ok = true
	return
}

func EscapeChapterID(id string) string {
	id = strings.ReplaceAll(id, "/", "$SLASH$")
	id = strings.ReplaceAll(id, "\\", "$BSLASH$")
	id = strings.ReplaceAll(id, ":", "$COLON$")
	id = strings.ReplaceAll(id, "*", "$ASTERISK$")
	id = strings.ReplaceAll(id, "?", "$QUESTION$")
	id = strings.ReplaceAll(id, "\"", "$QUOTE$")
	id = strings.ReplaceAll(id, "<", "$LT$")
	id = strings.ReplaceAll(id, ">", "$GT$")
	id = strings.ReplaceAll(id, "|", "$PIPE$")
	id = strings.ReplaceAll(id, ".", "$DOT$")
	id = strings.ReplaceAll(id, " ", "$SPACE$")
	id = strings.ReplaceAll(id, "_", "$UNDERSCORE$")
	return id
}

func UnescapeChapterID(id string) string {
	id = strings.ReplaceAll(id, "$SLASH$", "/")
	id = strings.ReplaceAll(id, "$BSLASH$", "\\")
	id = strings.ReplaceAll(id, "$COLON$", ":")
	id = strings.ReplaceAll(id, "$ASTERISK$", "*")
	id = strings.ReplaceAll(id, "$QUESTION$", "?")
	id = strings.ReplaceAll(id, "$QUOTE$", "\"")
	id = strings.ReplaceAll(id, "$LT$", "<")
	id = strings.ReplaceAll(id, "$GT$", ">")
	id = strings.ReplaceAll(id, "$PIPE$", "|")
	id = strings.ReplaceAll(id, "$DOT$", ".")
	id = strings.ReplaceAll(id, "$SPACE$", " ")
	id = strings.ReplaceAll(id, "$UNDERSCORE$", "_")
	return id
}

