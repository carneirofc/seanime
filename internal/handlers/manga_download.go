package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"seanime/internal/events"
	"seanime/internal/manga"
	chapter_downloader "seanime/internal/manga/downloader"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleDownloadMangaChapters
//
//	@summary adds chapters to the download queue.
//	@route /api/v1/manga/download-chapters [POST]
//	@returns bool
func (h *Handler) HandleDownloadMangaChapters(c echo.Context) error {

	type body struct {
		MediaId    int      `json:"mediaId"`
		Provider   string   `json:"provider"`
		ChapterIds []string `json:"chapterIds"`
		StartNow   bool     `json:"startNow"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.InfoToast, "Adding chapters to download queue...")

	// Resolve the media title once for the ComicInfo.xml metadata.
	// Empty if the manga is not in the user's collection.
	mediaTitle := ""
	if mangaCollection, err := h.App.GetMangaCollection(false); err == nil {
		if listEntry, ok := mangaCollection.GetListEntryFromMangaId(b.MediaId); ok {
			if media := listEntry.GetMedia(); media != nil {
				mediaTitle = media.GetPreferredTitle()
			}
		}
	}

	// Queueing fetches the page list from the provider for every chapter, which is
	// slow and network-bound. Do it in the background so the request returns promptly,
	// and keep going if a single chapter fails instead of aborting the whole batch.
	go func() {
		chapterIds := b.ChapterIds
		var failed int
		for _, chapterId := range chapterIds {
			err := h.App.MangaDownloader.DownloadChapter(manga.DownloadChapterOptions{
				Provider:   b.Provider,
				MediaId:    b.MediaId,
				ChapterId:  chapterId,
				MediaTitle: mediaTitle,
				StartNow:   b.StartNow,
			})
			if err != nil {
				failed++
				h.App.Logger.Error().Err(err).Str("chapterId", chapterId).Msg("manga: Failed to queue chapter for download")
			}
			time.Sleep(400 * time.Millisecond) // Sleep to avoid rate limiting
		}

		switch {
		case failed == len(chapterIds):
			h.App.WSEventManager.SendEvent(events.ErrorToast, "Failed to add chapters to the download queue")
		case failed > 0:
			h.App.WSEventManager.SendEvent(events.WarningToast, fmt.Sprintf("Added chapters to the download queue (%d failed)", failed))
		default:
			h.App.WSEventManager.SendEvent(events.SuccessToast, "Added chapters to the download queue")
		}
	}()

	return h.RespondWithData(c, true)
}

// HandleGetMangaDownloadData
//
//	@summary returns the download data for a specific media.
//	@desc This is used to display information about the downloaded and queued chapters in the UI.
//	@desc If the 'cached' parameter is false, it will refresh the data by rescanning the download folder.
//	@route /api/v1/manga/download-data [POST]
//	@returns manga.MediaDownloadData
func (h *Handler) HandleGetMangaDownloadData(c echo.Context) error {

	type body struct {
		MediaId int  `json:"mediaId"`
		Cached  bool `json:"cached"` // If false, it will refresh the data
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	data, err := h.App.MangaDownloader.GetMediaDownloads(b.MediaId, b.Cached)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, data)
}

// HandleGetMangaDownloadQueue
//
//	@summary returns the items in the download queue.
//	@route /api/v1/manga/download-queue [GET]
//	@returns []models.ChapterDownloadQueueItem
func (h *Handler) HandleGetMangaDownloadQueue(c echo.Context) error {

	data, err := h.App.Database.GetChapterDownloadQueue()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, data)
}

// HandleStartMangaDownloadQueue
//
//	@summary starts the download queue if it's not already running.
//	@desc This will start the download queue if it's not already running.
//	@desc Returns 'true' whether the queue was started or not.
//	@route /api/v1/manga/download-queue/start [POST]
//	@returns bool
func (h *Handler) HandleStartMangaDownloadQueue(c echo.Context) error {

	h.App.MangaDownloader.RunChapterDownloadQueue()

	return h.RespondWithData(c, true)
}

// HandleStopMangaDownloadQueue
//
//	@summary stops the manga download queue.
//	@desc This will stop the manga download queue.
//	@desc Returns 'true' whether the queue was stopped or not.
//	@route /api/v1/manga/download-queue/stop [POST]
//	@returns bool
func (h *Handler) HandleStopMangaDownloadQueue(c echo.Context) error {

	h.App.MangaDownloader.StopChapterDownloadQueue()

	return h.RespondWithData(c, true)

}

// HandleClearAllChapterDownloadQueue
//
//	@summary clears all chapters from the download queue.
//	@desc This will clear all chapters from the download queue.
//	@desc Returns 'true' whether the queue was cleared or not.
//	@desc This will also send a websocket event telling the client to refetch the download queue.
//	@route /api/v1/manga/download-queue [DELETE]
//	@returns bool
func (h *Handler) HandleClearAllChapterDownloadQueue(c echo.Context) error {

	err := h.App.Database.ClearAllChapterDownloadQueueItems()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	return h.RespondWithData(c, true)
}

// HandleResetErroredChapterDownloadQueue
//
//	@summary resets the errored chapters in the download queue.
//	@desc This will reset the errored chapters in the download queue, so they can be re-downloaded.
//	@desc Returns 'true' whether the queue was reset or not.
//	@desc This will also send a websocket event telling the client to refetch the download queue.
//	@route /api/v1/manga/download-queue/reset-errored [POST]
//	@returns bool
func (h *Handler) HandleResetErroredChapterDownloadQueue(c echo.Context) error {

	err := h.App.Database.ResetErroredChapterDownloadQueueItems()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	h.App.WSEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	return h.RespondWithData(c, true)
}

// HandleDeleteMangaDownloadedChapters
//
//	@summary deletes downloaded chapters.
//	@desc This will delete downloaded chapters from the filesystem.
//	@desc Returns 'true' whether the chapters were deleted or not.
//	@desc The client should refetch the download data after this.
//	@route /api/v1/manga/download-chapter [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteMangaDownloadedChapters(c echo.Context) error {

	type body struct {
		DownloadIds []chapter_downloader.DownloadID `json:"downloadIds"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithError(c, err)
	}

	err := h.App.MangaDownloader.DeleteChapters(b.DownloadIds)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// resolveMangaTitleForFilename returns the media's preferred title for use in
// download filenames, or an empty string if the manga is not in the collection.
func (h *Handler) resolveMangaTitleForFilename(mediaId int) string {
	mangaCollection, err := h.App.GetMangaCollection(false)
	if err != nil {
		return ""
	}
	listEntry, ok := mangaCollection.GetListEntryFromMangaId(mediaId)
	if !ok {
		return ""
	}
	media := listEntry.GetMedia()
	if media == nil {
		return ""
	}
	return sanitizeArchiveFilename(media.GetPreferredTitle())
}

// sanitizeArchiveFilename strips characters that are invalid in filenames on
// common filesystems (and quotes, which would break the Content-Disposition header).
func sanitizeArchiveFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return ' '
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, name)
	return strings.Join(strings.Fields(name), " ")
}

// HandleDownloadMangaChapterArchive
//
//	@summary downloads a chapter's CBZ archive file.
//	@desc This serves the chapter's CBZ archive as a file attachment.
//	@desc Chapters still stored in the legacy loose-image layout are converted to CBZ on the fly.
//	@route /api/v1/manga/downloads/chapter-archive [GET]
//	@returns nil
func (h *Handler) HandleDownloadMangaChapterArchive(c echo.Context) error {

	provider := c.QueryParam("provider")
	chapterId := c.QueryParam("chapterId")
	mediaId, err := strconv.Atoi(c.QueryParam("mediaId"))
	if provider == "" || chapterId == "" || err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "provider, mediaId and chapterId are required")
	}

	downloadDir := h.App.Config.Manga.DownloadDir

	var id chapter_downloader.DownloadID
	relPath := ""
	for downloadId, p := range chapter_downloader.ScanDownloadDir(downloadDir) {
		if downloadId.Provider == provider && downloadId.MediaId == mediaId && downloadId.ChapterId == chapterId {
			id, relPath = downloadId, p
			break
		}
	}
	if relPath == "" {
		return echo.NewHTTPError(http.StatusNotFound, "chapter is not downloaded")
	}

	title := h.resolveMangaTitleForFilename(mediaId)
	if title == "" {
		title = chapter_downloader.FormatSeriesDirName(provider, mediaId)
	}
	filename := fmt.Sprintf("%s - Chapter %s.cbz", title, sanitizeArchiveFilename(id.ChapterNumber))

	if strings.HasSuffix(relPath, ".cbz") {
		return c.Attachment(filepath.Join(downloadDir, filepath.FromSlash(relPath)), filename)
	}

	// Legacy loose-image directory: wrap the pages into a CBZ on the fly
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.comicbook+zip")
	c.Response().WriteHeader(http.StatusOK)
	return chapter_downloader.WriteLegacyDirAsCBZ(c.Response(), filepath.Join(downloadDir, filepath.FromSlash(relPath)))
}

// HandleDownloadMangaMediaArchive
//
//	@summary downloads all downloaded chapters of a media as a zip of CBZ files.
//	@desc This streams a zip archive containing one CBZ file per downloaded chapter for the given provider and media.
//	@route /api/v1/manga/downloads/media-archive [GET]
//	@returns nil
func (h *Handler) HandleDownloadMangaMediaArchive(c echo.Context) error {

	provider := c.QueryParam("provider")
	mediaId, err := strconv.Atoi(c.QueryParam("mediaId"))
	if provider == "" || err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "provider and mediaId are required")
	}

	downloadDir := h.App.Config.Manga.DownloadDir

	// Verify there is something to archive before committing to a 200 response
	found := false
	for downloadId := range chapter_downloader.ScanDownloadDir(downloadDir) {
		if downloadId.Provider == provider && downloadId.MediaId == mediaId {
			found = true
			break
		}
	}
	if !found {
		return echo.NewHTTPError(http.StatusNotFound, "no downloaded chapters found")
	}

	title := h.resolveMangaTitleForFilename(mediaId)
	if title == "" {
		title = chapter_downloader.FormatSeriesDirName(provider, mediaId)
	}
	filename := fmt.Sprintf("%s (%s).zip", title, sanitizeArchiveFilename(provider))

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	if _, err := chapter_downloader.WriteMediaArchive(c.Response(), downloadDir, provider, mediaId); err != nil {
		// Headers are already sent; log and abort the stream
		h.App.Logger.Error().Err(err).Str("provider", provider).Int("mediaId", mediaId).Msg("manga: Failed to stream media archive")
		return err
	}
	return nil
}

// HandleGetMangaDownloadsList
//
//	@summary displays the list of downloaded manga.
//	@desc This analyzes the download folder and returns a well-formatted structure for displaying downloaded manga.
//	@desc It returns a list of manga.DownloadListItem where the media data might be nil if it's not in the AniList collection.
//	@route /api/v1/manga/downloads [GET]
//	@returns []manga.DownloadListItem
func (h *Handler) HandleGetMangaDownloadsList(c echo.Context) error {

	mangaCollection, err := h.App.GetMangaCollection(false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	res, err := h.App.MangaDownloader.NewDownloadList(&manga.NewDownloadListOptions{
		MangaCollection: mangaCollection,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, res)
}
