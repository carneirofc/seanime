package handlers

import (
	"fmt"
	"seanime/internal/events"
	"seanime/internal/manga"
	chapter_downloader "seanime/internal/manga/downloader"
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
