package chapter_downloader

import (
	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
	"seanime/internal/database/db"
	"seanime/internal/database/models"
	"seanime/internal/events"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"seanime/internal/util"
	"sync"
	"time"
)

const (
	QueueStatusNotStarted  QueueStatus = "not_started"
	QueueStatusDownloading QueueStatus = "downloading"
	QueueStatusErrored     QueueStatus = "errored"
)

// interChapterDelay throttles the rate at which chapters are dispatched to the
// downloader to avoid hammering providers (and getting rate-limited/banned).
const interChapterDelay = 2 * time.Second

type (
	// Queue is used to manage the download queue.
	// If feeds the downloader with the next item in the queue.
	Queue struct {
		logger         *zerolog.Logger
		mu             sync.Mutex
		db             *db.Database
		current        *QueueInfo
		runCh          chan *QueueInfo // Channel to tell downloader to run the next item
		active         bool
		wsEventManager events.WSEventManagerInterface
	}

	QueueStatus string

	// QueueInfo stores details about the download progress of a chapter.
	QueueInfo struct {
		DownloadID
		Pages          []*hibikemanga.ChapterPage
		DownloadedUrls []string
		Status         QueueStatus
	}
)

func NewQueue(db *db.Database, logger *zerolog.Logger, wsEventManager events.WSEventManagerInterface, runCh chan *QueueInfo) *Queue {
	return &Queue{
		logger:         logger,
		db:             db,
		runCh:          runCh,
		wsEventManager: wsEventManager,
	}
}

// Add adds a chapter to the download queue.
// It tells the queue to download the next item if possible.
func (q *Queue) Add(id DownloadID, pages []*hibikemanga.ChapterPage, runNext bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	marshalled, err := json.Marshal(pages)
	if err != nil {
		q.logger.Error().Err(err).Msgf("Failed to marshal pages for id %v", id)
		return err
	}

	err = q.db.InsertChapterDownloadQueueItem(&models.ChapterDownloadQueueItem{
		BaseModel:     models.BaseModel{},
		Provider:      id.Provider,
		MediaID:       id.MediaId,
		ChapterNumber: id.ChapterNumber,
		ChapterID:     id.ChapterId,
		PageData:      marshalled,
		Status:        string(QueueStatusNotStarted),
	})
	if err != nil {
		q.logger.Error().Err(err).Msgf("Failed to insert chapter download queue item for id %v", id)
		return err
	}

	q.logger.Info().Msgf("chapter downloader: Added chapter to download queue: %s", id.ChapterId)

	active := q.active

	q.wsEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	if runNext && active {
		// Tells queue to run next if possible. runNext acquires the lock itself,
		// so it must be invoked without holding q.mu.
		go q.runNext()
	}

	return nil
}

func (q *Queue) HasCompleted(queueInfo *QueueInfo) {
	q.mu.Lock()

	if queueInfo.Status == QueueStatusErrored {
		q.logger.Warn().Msgf("chapter downloader: Errored %s", queueInfo.DownloadID.ChapterId)
		// Update the status of the item in the database.
		_ = q.db.UpdateChapterDownloadQueueItemStatus(queueInfo.DownloadID.Provider, queueInfo.DownloadID.MediaId, queueInfo.DownloadID.ChapterId, string(QueueStatusErrored))
	} else {
		q.logger.Debug().Msgf("chapter downloader: Dequeueing %s", queueInfo.DownloadID.ChapterId)
		// Dequeue the item from the database.
		_, err := q.db.DequeueChapterDownloadQueueItem()
		if err != nil {
			q.logger.Error().Err(err).Msgf("Failed to dequeue chapter download queue item for id %v", queueInfo.DownloadID)
			q.mu.Unlock()
			return
		}
	}

	// Reset current item
	q.current = nil
	active := q.active
	q.mu.Unlock()

	q.wsEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)
	q.wsEventManager.SendEvent(events.RefreshedMangaDownloadData, nil)

	if active {
		// Tells queue to run next if possible (runNext locks internally).
		q.runNext()
	}
}

// Run activates the queue and invokes runNext
func (q *Queue) Run() {
	q.mu.Lock()
	if !q.active {
		q.logger.Debug().Msg("chapter downloader: Starting queue")
	}
	q.active = true
	q.mu.Unlock()

	// Tells queue to run next if possible (runNext locks internally).
	q.runNext()
}

// Stop deactivates the queue
func (q *Queue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active {
		q.logger.Debug().Msg("chapter downloader: Stopping queue")
	}

	q.active = false
}

// runNext runs the next item in the queue.
//   - Acquires the queue lock itself, so callers must NOT hold q.mu.
//   - Returns early if the queue is inactive or an item is already in progress.
//   - Otherwise it pulls the next item from the database, marks it current and
//     dispatches it to the downloader after a throttling delay.
func (q *Queue) runNext() {

	// Catch panic in runNext, so it doesn't bubble up and stop goroutines.
	defer util.HandlePanicInModuleThen("internal/manga/downloader/runNext", func() {
		q.logger.Error().Msg("chapter downloader: Panic in 'runNext'")
	})

	current, id, ok := q.prepareNext()
	if !ok {
		return
	}

	q.wsEventManager.SendEvent(events.ChapterDownloadQueueUpdated, nil)

	// Throttle and dispatch off the lock so the queue mutex is never held while
	// sleeping or blocking on the downloader channel.
	go func() {
		time.Sleep(interChapterDelay)
		q.logger.Info().Msgf("chapter downloader: Running next item in queue: %s", id.ChapterId)
		q.runCh <- current
	}()
}

// prepareNext pulls the next queued item under lock and marks it as current.
// It returns ok=false when the queue is inactive, already busy, empty, or the
// queued item's page data is invalid. The lock is always released (even on
// panic) via defer, so a recovered panic can never leave the queue deadlocked.
func (q *Queue) prepareNext() (current *QueueInfo, id DownloadID, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.logger.Debug().Msg("chapter downloader: Processing next item in queue")

	if !q.active {
		return nil, DownloadID{}, false
	}

	if q.current != nil {
		q.logger.Debug().Msg("chapter downloader: Current item is not nil")
		return nil, DownloadID{}, false
	}

	q.logger.Debug().Msg("chapter downloader: Checking next item in queue")

	// Get next item from the database.
	next, _ := q.db.GetNextChapterDownloadQueueItem()
	if next == nil {
		q.logger.Debug().Msg("chapter downloader: No next item in queue")
		return nil, DownloadID{}, false
	}

	id = DownloadID{
		Provider:      next.Provider,
		MediaId:       next.MediaID,
		ChapterId:     next.ChapterID,
		ChapterNumber: next.ChapterNumber,
	}

	q.logger.Debug().Msgf("chapter downloader: Preparing next item in queue: %s", id.ChapterId)

	current = &QueueInfo{
		DownloadID:     id,
		DownloadedUrls: make([]string, 0),
		Status:         QueueStatusDownloading,
	}

	// Unmarshal the page data.
	if err := json.Unmarshal(next.PageData, &current.Pages); err != nil {
		q.logger.Error().Err(err).Msgf("Failed to unmarshal pages for id %v", id)
		_ = q.db.UpdateChapterDownloadQueueItemStatus(id.Provider, id.MediaId, id.ChapterId, string(QueueStatusNotStarted))
		return nil, DownloadID{}, false
	}

	// Update status and mark as current while still holding the lock so no other
	// runNext call can pick up the same item.
	_ = q.db.UpdateChapterDownloadQueueItemStatus(id.Provider, id.MediaId, id.ChapterId, string(QueueStatusDownloading))
	q.current = current

	return current, id, true
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (q *Queue) GetCurrent() (qi *QueueInfo, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.current == nil {
		return nil, false
	}

	return q.current, true
}
