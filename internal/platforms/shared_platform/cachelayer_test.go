package shared_platform

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/events"
	"seanime/internal/util"
	"strconv"
	"testing"
	"time"

	"github.com/gqlgo/gqlgenc/clientv2"
	"github.com/stretchr/testify/require"
)

type cacheLayerTestClient struct {
	anilist.AnilistClient
	cacheDir            string
	animeCollection     *anilist.AnimeCollection
	mangaCollection     *anilist.MangaCollection
	updateEntryCalls    []cacheLayerUpdateEntryCall
	updateProgressCalls []cacheLayerUpdateProgressCall

	animeCollectionTagsCalls int
}

type cacheLayerUpdateEntryCall struct {
	MediaID     *int
	Status      *anilist.MediaListStatus
	ScoreRaw    *int
	Progress    *int
	StartedAt   *anilist.FuzzyDateInput
	CompletedAt *anilist.FuzzyDateInput
}

type cacheLayerUpdateProgressCall struct {
	MediaID  *int
	Progress *int
	Status   *anilist.MediaListStatus
}

func (c *cacheLayerTestClient) IsAuthenticated() bool {
	return true
}

func (c *cacheLayerTestClient) GetCacheDir() string {
	return c.cacheDir
}

func (c *cacheLayerTestClient) BaseAnimeByID(_ context.Context, id *int, _ ...clientv2.RequestInterceptor) (*anilist.BaseAnimeByID, error) {
	mediaID := 0
	if id != nil {
		mediaID = *id
	}
	return &anilist.BaseAnimeByID{Media: &anilist.BaseAnime{ID: mediaID}}, nil
}

func (c *cacheLayerTestClient) AnimeCollection(_ context.Context, _ *string, _ ...clientv2.RequestInterceptor) (*anilist.AnimeCollection, error) {
	return c.animeCollection, nil
}

func (c *cacheLayerTestClient) MangaCollection(_ context.Context, _ *string, _ ...clientv2.RequestInterceptor) (*anilist.MangaCollection, error) {
	return c.mangaCollection, nil
}

func (c *cacheLayerTestClient) UpdateMediaListEntry(_ context.Context, mediaID *int, status *anilist.MediaListStatus, scoreRaw *int, progress *int, startedAt *anilist.FuzzyDateInput, completedAt *anilist.FuzzyDateInput, _ *bool, _ *bool, _ ...clientv2.RequestInterceptor) (*anilist.UpdateMediaListEntry, error) {
	c.updateEntryCalls = append(c.updateEntryCalls, cacheLayerUpdateEntryCall{
		MediaID:     newCloned(mediaID),
		Status:      newCloned(status),
		ScoreRaw:    newCloned(scoreRaw),
		Progress:    newCloned(progress),
		StartedAt:   cloneFuzzyDateInput(startedAt),
		CompletedAt: cloneFuzzyDateInput(completedAt),
	})
	return &anilist.UpdateMediaListEntry{SaveMediaListEntry: &anilist.UpdateMediaListEntry_SaveMediaListEntry{ID: 999}}, nil
}

func (c *cacheLayerTestClient) UpdateMediaListEntryProgress(_ context.Context, mediaID *int, progress *int, status *anilist.MediaListStatus, _ ...clientv2.RequestInterceptor) (*anilist.UpdateMediaListEntryProgress, error) {
	c.updateProgressCalls = append(c.updateProgressCalls, cacheLayerUpdateProgressCall{
		MediaID:  newCloned(mediaID),
		Progress: newCloned(progress),
		Status:   newCloned(status),
	})
	return &anilist.UpdateMediaListEntryProgress{SaveMediaListEntry: &anilist.UpdateMediaListEntryProgress_SaveMediaListEntry{ID: 999}}, nil
}

// anilistErrorWithStatus builds the error shape parseResponse produces for a non-2xx AniList reply,
// so a test can describe what AniList actually answered rather than only what the message reads like.
func anilistErrorWithStatus(code int, message string) error {
	return &clientv2.ErrorResponse{
		NetworkError: &clientv2.HTTPError{
			Code:    code,
			Message: message,
		},
	}
}

// newLogoutTestCacheLayer returns a cache layer whose logout is observable.
// The logger is not optional: checkAndUpdateWorkingState logs the raw error before
// logging out, so a nil one panics and takes every other test in the package with it.
func newLogoutTestCacheLayer(t *testing.T) (*CacheLayer, chan struct{}) {
	t.Helper()

	previousEventManager := events.GlobalWSEventManager
	events.GlobalWSEventManager = &events.GlobalWSEventManagerWrapper{}
	t.Cleanup(func() {
		events.GlobalWSEventManager = previousEventManager
		clearFailureTracking()
		consecutiveAuthFailures.Store(0)
	})

	logoutCalled := make(chan struct{}, 1)
	return &CacheLayer{
		logger: util.NewLogger(),
		logoutFunc: func() {
			logoutCalled <- struct{}{}
		},
	}, logoutCalled
}

func TestCacheLayerLogsOutOnRepeatedInvalidToken(t *testing.T) {
	cacheLayer, logoutCalled := newLogoutTestCacheLayer(t)

	// Logging out blanks the stored account row, so one auth-shaped reply is not enough.
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(400, "graphql: Invalid token"))
	select {
	case <-logoutCalled:
		t.Fatal("a single invalid token error should not end the session")
	case <-time.After(50 * time.Millisecond):
	}

	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(400, "graphql: Invalid token"))
	select {
	case <-logoutCalled:
	case <-time.After(time.Second):
		t.Fatal("expected a second invalid token error to trigger logout")
	}
	require.Zero(t, getRecentFailureCount())
}

func TestCacheLayerLogsOutOnUnauthorized(t *testing.T) {
	cacheLayer, logoutCalled := newLogoutTestCacheLayer(t)

	// A 401 needs no message match at all, but still needs confirming.
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(401, "unauthorized"))
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(401, "unauthorized"))

	select {
	case <-logoutCalled:
	case <-time.After(time.Second):
		t.Fatal("expected repeated 401s to trigger logout")
	}
}

func TestCacheLayerDoesNotLogOutWithoutAnAniListStatus(t *testing.T) {
	cacheLayer, logoutCalled := newLogoutTestCacheLayer(t)

	// A transport failure carries no HTTP status. Quoting "invalid token" in its message must not
	// be enough to end the session — that substring match alone used to be the whole test.
	for range 5 {
		cacheLayer.checkAndUpdateWorkingState(errors.New("graphql: Invalid token"))
	}

	select {
	case <-logoutCalled:
		t.Fatal("a statusless error should never be treated as an auth failure")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCacheLayerAuthFailureCountResetsOnSuccess(t *testing.T) {
	cacheLayer, logoutCalled := newLogoutTestCacheLayer(t)

	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(400, "graphql: Invalid token"))
	cacheLayer.checkAndUpdateWorkingState(nil)
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(400, "graphql: Invalid token"))

	select {
	case <-logoutCalled:
		t.Fatal("a successful request between two auth errors should reset the count")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCacheLayerIgnoresStatusesInErrorText(t *testing.T) {
	// A media id or title in the message ("14042", "Room 404") matched the old substring test and
	// silently excused a real failure. Classification now follows the transported status.
	t.Cleanup(func() {
		clearFailureTracking()
		IsWorking.Store(true)
	})
	clearFailureTracking()

	cacheLayer := &CacheLayer{logger: util.NewLogger()}
	cacheLayer.checkAndUpdateWorkingState(errors.New("failed to fetch media 14042: connection reset"))
	require.Equal(t, 1, getRecentFailureCount())

	// A status AniList actually replied with is still excused.
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(404, "not found"))
	cacheLayer.checkAndUpdateWorkingState(anilistErrorWithStatus(429, "rate limited"))
	require.Equal(t, 1, getRecentFailureCount())
}

func TestCacheLayerQueuesProgressUpdateAndPatchesAnimeCache(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	// get the collection in cache
	_, err := cacheLayer.AnimeCollection(context.Background(), new("user"))
	require.NoError(t, err)

	IsWorking.Store(false)
	res, err := cacheLayer.UpdateMediaListEntryProgress(context.Background(), new(101), new(6), new(anilist.MediaListStatusCompleted))
	require.NoError(t, err)
	require.Equal(t, 321, res.GetSaveMediaListEntry().GetID())
	require.Empty(t, client.updateProgressCalls)

	// the cached entry should move lists immediately so refetches see the local change.
	cached := getCachedAnimeCollection(t, cacheLayer)
	entry, found := cached.GetListEntryFromAnimeId(101)
	require.True(t, found)
	require.Equal(t, 6, *entry.GetProgress())
	require.Equal(t, anilist.MediaListStatusCompleted, *entry.GetStatus())
	require.True(t, animeListContains(cached, anilist.MediaListStatusCompleted, 101))
	require.False(t, animeListContains(cached, anilist.MediaListStatusCurrent, 101))

	queued := getQueuedUpdate(t, cacheLayer, 101)
	require.Equal(t, 101, queued.MediaID)
	require.Equal(t, 6, *queued.Progress)
	require.Equal(t, anilist.MediaListStatusCompleted, *queued.Status)
	require.False(t, queued.FullUpdate)
}

func TestCacheLayerQueuesEntryUpdateAndSyncsWhenOnline(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		mangaCollection: newTestMangaCollection(202, 654, anilist.MediaListStatusCurrent, 4),
	}
	cacheLayer := newTestCacheLayer(t, client)

	// seed manga cache, then queue an edit while the api is marked down
	_, err := cacheLayer.MangaCollection(context.Background(), new("user"))
	require.NoError(t, err)

	IsWorking.Store(false)
	startedAt := &anilist.FuzzyDateInput{Year: new(2025), Month: new(1), Day: new(2)}
	completedAt := &anilist.FuzzyDateInput{Year: new(2025), Month: new(2), Day: new(3)}
	res, err := cacheLayer.UpdateMediaListEntry(context.Background(), new(202), new(anilist.MediaListStatusCompleted), new(85), new(12), startedAt, completedAt, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 654, res.GetSaveMediaListEntry().GetID())
	require.Empty(t, client.updateEntryCalls)

	cached := getCachedMangaCollection(t, cacheLayer)
	entry, found := cached.GetListEntryFromMangaId(202)
	require.True(t, found)
	require.Equal(t, 12, *entry.GetProgress())
	require.Equal(t, float64(85), *entry.GetScore())
	require.Equal(t, anilist.MediaListStatusCompleted, *entry.GetStatus())
	require.True(t, mangaListContains(cached, anilist.MediaListStatusCompleted, 202))
	require.False(t, mangaListContains(cached, anilist.MediaListStatusCurrent, 202))

	// when the api is healthy again, the queued full edit is flushed once and removed
	IsWorking.Store(true)
	cacheLayer.syncQueuedUpdates(context.Background())
	require.Len(t, client.updateEntryCalls, 1)
	require.Equal(t, 202, *client.updateEntryCalls[0].MediaID)
	require.Equal(t, anilist.MediaListStatusCompleted, *client.updateEntryCalls[0].Status)
	require.Equal(t, 85, *client.updateEntryCalls[0].ScoreRaw)
	require.Equal(t, 12, *client.updateEntryCalls[0].Progress)
	require.Equal(t, 2025, *client.updateEntryCalls[0].StartedAt.Year)
	require.Equal(t, 3, *client.updateEntryCalls[0].CompletedAt.Day)
	requireNoQueuedUpdate(t, cacheLayer, 202)
}

func TestCacheLayerLiveProgressUpdateClearsQueuedUpdate(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollection(context.Background(), new("user"))
	require.NoError(t, err)

	// first update is queued while the api is marked down
	IsWorking.Store(false)
	_, err = cacheLayer.UpdateMediaListEntryProgress(context.Background(), new(101), new(6), new(anilist.MediaListStatusCompleted))
	require.NoError(t, err)
	queued := getQueuedUpdate(t, cacheLayer, 101)
	require.Equal(t, 6, *queued.Progress)

	// a later online update should win and remove the stale queued state
	IsWorking.Store(true)
	_, err = cacheLayer.UpdateMediaListEntryProgress(context.Background(), new(101), new(7), new(anilist.MediaListStatusCurrent))
	require.NoError(t, err)
	requireNoQueuedUpdate(t, cacheLayer, 101)

	cacheLayer.syncQueuedUpdates(context.Background())
	require.Len(t, client.updateProgressCalls, 1)
	require.Equal(t, 7, *client.updateProgressCalls[0].Progress)
}

func TestCacheLayerLiveEntryUpdateClearsQueuedUpdate(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		mangaCollection: newTestMangaCollection(202, 654, anilist.MediaListStatusCurrent, 4),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.MangaCollection(context.Background(), new("user"))
	require.NoError(t, err)

	// queue an older edit while AniList is unavailable
	IsWorking.Store(false)
	_, err = cacheLayer.UpdateMediaListEntry(context.Background(), new(202), new(anilist.MediaListStatusCompleted), new(80), new(12), nil, nil, nil, nil)
	require.NoError(t, err)
	queued := getQueuedUpdate(t, cacheLayer, 202)
	require.Equal(t, 80, *queued.ScoreRaw)

	// the successful online edit replaces it and should prevent stale replay
	IsWorking.Store(true)
	_, err = cacheLayer.UpdateMediaListEntry(context.Background(), new(202), new(anilist.MediaListStatusCurrent), new(90), new(13), nil, nil, nil, nil)
	require.NoError(t, err)
	requireNoQueuedUpdate(t, cacheLayer, 202)

	cacheLayer.syncQueuedUpdates(context.Background())
	require.Len(t, client.updateEntryCalls, 1)
	require.Equal(t, 90, *client.updateEntryCalls[0].ScoreRaw)
	require.Equal(t, 13, *client.updateEntryCalls[0].Progress)
}

func newTestCacheLayer(t *testing.T, client *cacheLayerTestClient) *CacheLayer {
	t.Helper()
	ShouldCache.Store(true)
	IsWorking.Store(true)
	clearFailureTracking()

	clientRef := util.NewRef[anilist.AnilistClient](client)
	cacheLayer, ok := NewCacheLayer(clientRef).(*CacheLayer)
	require.True(t, ok)

	t.Cleanup(func() {
		ShouldCache.Store(true)
		IsWorking.Store(true)
		clearFailureTracking()
	})

	return cacheLayer
}

func newTestAnimeCollection(mediaID int, entryID int, status anilist.MediaListStatus, progress int) *anilist.AnimeCollection {
	return &anilist.AnimeCollection{
		MediaListCollection: &anilist.AnimeCollection_MediaListCollection{
			Lists: []*anilist.AnimeCollection_MediaListCollection_Lists{
				newTestAnimeList(status, newTestAnimeEntry(mediaID, entryID, status, progress)),
				newTestAnimeList(anilist.MediaListStatusCompleted),
			},
		},
	}
}

func newTestAnimeList(status anilist.MediaListStatus, entries ...*anilist.AnimeCollection_MediaListCollection_Lists_Entries) *anilist.AnimeCollection_MediaListCollection_Lists {
	return &anilist.AnimeCollection_MediaListCollection_Lists{
		Status:       new(status),
		Name:         new(string(status)),
		IsCustomList: new(false),
		Entries:      entries,
	}
}

func newTestAnimeEntry(mediaID int, entryID int, status anilist.MediaListStatus, progress int) *anilist.AnimeCollection_MediaListCollection_Lists_Entries {
	return &anilist.AnimeCollection_MediaListCollection_Lists_Entries{
		ID:       entryID,
		Media:    &anilist.BaseAnime{ID: mediaID, Episodes: new(12)},
		Status:   new(status),
		Progress: new(progress),
		Score:    new(0.0),
	}
}

func newTestMangaCollection(mediaID int, entryID int, status anilist.MediaListStatus, progress int) *anilist.MangaCollection {
	return &anilist.MangaCollection{
		MediaListCollection: &anilist.MangaCollection_MediaListCollection{
			Lists: []*anilist.MangaCollection_MediaListCollection_Lists{
				newTestMangaList(status, newTestMangaEntry(mediaID, entryID, status, progress)),
				newTestMangaList(anilist.MediaListStatusCompleted),
			},
		},
	}
}

func newTestMangaList(status anilist.MediaListStatus, entries ...*anilist.MangaCollection_MediaListCollection_Lists_Entries) *anilist.MangaCollection_MediaListCollection_Lists {
	return &anilist.MangaCollection_MediaListCollection_Lists{
		Status:       new(status),
		Name:         new(string(status)),
		IsCustomList: new(false),
		Entries:      entries,
	}
}

func newTestMangaEntry(mediaID int, entryID int, status anilist.MediaListStatus, progress int) *anilist.MangaCollection_MediaListCollection_Lists_Entries {
	return &anilist.MangaCollection_MediaListCollection_Lists_Entries{
		ID:       entryID,
		Media:    &anilist.BaseManga{ID: mediaID, Chapters: new(20)},
		Status:   new(status),
		Progress: new(progress),
		Score:    new(0.0),
	}
}

func getCachedAnimeCollection(t *testing.T, cacheLayer *CacheLayer) *anilist.AnimeCollection {
	t.Helper()
	var cached anilist.AnimeCollection
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[AnimeCollectionBucket], cacheLayer.collectionCacheKey("collection", nil), &cached)
	require.NoError(t, err)
	require.True(t, found)
	return &cached
}

func getCachedMangaCollection(t *testing.T, cacheLayer *CacheLayer) *anilist.MangaCollection {
	t.Helper()
	var cached anilist.MangaCollection
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[MangaCollectionBucket], cacheLayer.collectionCacheKey("collection", nil), &cached)
	require.NoError(t, err)
	require.True(t, found)
	return &cached
}

func getQueuedUpdate(t *testing.T, cacheLayer *CacheLayer, mediaID int) queuedMediaListUpdate {
	t.Helper()
	var queued queuedMediaListUpdate
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[PendingMediaListUpdatesBucket], strconv.Itoa(mediaID), &queued)
	require.NoError(t, err)
	require.True(t, found)
	return queued
}

func requireNoQueuedUpdate(t *testing.T, cacheLayer *CacheLayer, mediaID int) {
	t.Helper()
	var queued queuedMediaListUpdate
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[PendingMediaListUpdatesBucket], strconv.Itoa(mediaID), &queued)
	require.NoError(t, err)
	require.False(t, found)
}

func animeListContains(collection *anilist.AnimeCollection, status anilist.MediaListStatus, mediaID int) bool {
	for _, list := range collection.GetMediaListCollection().GetLists() {
		if list.GetStatus() == nil || *list.GetStatus() != status {
			continue
		}
		for _, entry := range list.GetEntries() {
			if entry.GetMedia().GetID() == mediaID {
				return true
			}
		}
	}
	return false
}

func mangaListContains(collection *anilist.MangaCollection, status anilist.MediaListStatus, mediaID int) bool {
	for _, list := range collection.GetMediaListCollection().GetLists() {
		if list.GetStatus() == nil || *list.GetStatus() != status {
			continue
		}
		for _, entry := range list.GetEntries() {
			if entry.GetMedia().GetID() == mediaID {
				return true
			}
		}
	}
	return false
}

func (c *cacheLayerTestClient) AnimeListEntriesNotIn(_ context.Context, _ *string, _ []*int, _ *int, _ *int, _ ...clientv2.RequestInterceptor) (*anilist.AnimeListEntriesNotIn, error) {
	return &anilist.AnimeListEntriesNotIn{Page: &anilist.AnimeListEntriesNotIn_Page{}}, nil
}

func (c *cacheLayerTestClient) MangaListEntriesNotIn(_ context.Context, _ *string, _ []*int, _ *int, _ *int, _ ...clientv2.RequestInterceptor) (*anilist.MangaListEntriesNotIn, error) {
	return &anilist.MangaListEntriesNotIn{Page: &anilist.MangaListEntriesNotIn_Page{}}, nil
}

// animeCollectionTagsCalls counts how often the tags query actually reached AniList.
func (c *cacheLayerTestClient) AnimeCollectionTags(_ context.Context, _ *string, _ ...clientv2.RequestInterceptor) (*anilist.AnimeCollectionTags, error) {
	c.animeCollectionTagsCalls++
	return &anilist.AnimeCollectionTags{
		MediaListCollection: &anilist.AnimeCollectionTags_MediaListCollection{
			Lists: []*anilist.AnimeCollectionTags_MediaListCollection_Lists{
				{
					Entries: []*anilist.AnimeCollectionTags_MediaListCollection_Lists_Entries{
						{
							Media: &anilist.AnimeCollectionTags_MediaListCollection_Lists_Entries_Media{
								ID:   101,
								Tags: []*anilist.AnimeCollectionTags_MediaListCollection_Lists_Entries_Media_Tags{{Name: "Action"}},
							},
						},
					},
				},
			},
		},
	}, nil
}

// TestCacheLayerAnimeCollectionTagsIsServedFromCache pins the cache-first behaviour:
// tags are immutable metadata, so a second read inside the TTL must not cost a request.
func TestCacheLayerAnimeCollectionTagsIsServedFromCache(t *testing.T) {
	client := &cacheLayerTestClient{cacheDir: t.TempDir()}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollectionTags(context.Background(), new("user"))
	require.NoError(t, err)
	require.Equal(t, 1, client.animeCollectionTagsCalls)

	_, err = cacheLayer.AnimeCollectionTags(context.Background(), new("user"))
	require.NoError(t, err)
	require.Equal(t, 1, client.animeCollectionTagsCalls, "a second read within the TTL must be served from the file cache")
}

// TestCacheLayerProgressUpdateKeepsCollectionTags pins the invalidation fix. A progress
// update cannot change any media's tags, and wiping the bucket here used to cost a
// whole-collection tags query on the very next request — a large part of the 429s.
func TestCacheLayerProgressUpdateKeepsCollectionTags(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollectionTags(context.Background(), new("user"))
	require.NoError(t, err)
	require.Equal(t, 1, client.animeCollectionTagsCalls)

	_, err = cacheLayer.UpdateMediaListEntryProgress(context.Background(), new(101), new(3), new(anilist.MediaListStatusCurrent))
	require.NoError(t, err)

	_, err = cacheLayer.AnimeCollectionTags(context.Background(), new("user"))
	require.NoError(t, err)
	require.Equal(t, 1, client.animeCollectionTagsCalls, "a progress update must not invalidate the collection tag cache")
}

// The real collection buckets must still be invalidated, or list and progress data
// would go stale after an edit.
func TestCacheLayerProgressUpdateStillInvalidatesCollection(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollection(context.Background(), new("user"))
	require.NoError(t, err)

	_, err = cacheLayer.UpdateMediaListEntryProgress(context.Background(), new(101), new(3), new(anilist.MediaListStatusCurrent))
	require.NoError(t, err)

	var cached anilist.AnimeCollection
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[AnimeCollectionBucket], cacheLayer.collectionCacheKey("collection", nil), &cached)
	require.NoError(t, err)
	require.False(t, found, "the anime collection cache must still be invalidated by a mutation")
}

func anilistBadRequestError() error {
	return &clientv2.ErrorResponse{
		NetworkError: &clientv2.HTTPError{
			Code:    400,
			Message: `Response body {"errors":[{"message":"validation","status":400}]}`,
		},
	}
}

func TestCacheLayerIgnoresValidationErrorsForApiHealth(t *testing.T) {
	// A 400 means Seanime sent a malformed request, not that AniList is down. Privatizing a batch of
	// entries used to produce one per entry, which tripped the failure threshold and dropped the whole
	// integration into cache-only mode.
	previousEventManager := events.GlobalWSEventManager
	events.GlobalWSEventManager = &events.GlobalWSEventManagerWrapper{}
	wasWorking := IsWorking.Load()
	t.Cleanup(func() {
		events.GlobalWSEventManager = previousEventManager
		IsWorking.Store(wasWorking)
		clearFailureTracking()
	})

	clearFailureTracking()
	IsWorking.Store(true)

	cacheLayer := &CacheLayer{logger: util.NewLogger()}
	for range failureThreshold + 1 {
		cacheLayer.checkAndUpdateWorkingState(anilistBadRequestError())
	}

	require.Zero(t, getRecentFailureCount())
	require.True(t, IsWorking.Load())
}

func TestShouldQueueMediaListUpdateSkipsValidationErrors(t *testing.T) {
	// Queueing a 400 would replay the same rejected mutation on every sync tick, forever.
	wasWorking := IsWorking.Load()
	t.Cleanup(func() { IsWorking.Store(wasWorking) })

	for _, working := range []bool{true, false} {
		IsWorking.Store(working)
		require.Falsef(t, shouldQueueMediaListUpdate(anilistBadRequestError()),
			"a 400 must not be queued (IsWorking=%v)", working)
	}

	// A genuine outage still queues, including while already in cache-only mode.
	IsWorking.Store(true)
	require.True(t, shouldQueueMediaListUpdate(errors.New("graphql: server error 503")))
}

func TestCollectionCacheIsKeyedPerAccount(t *testing.T) {
	// The file cache lives in one per-install directory. A key that ignored userName meant that
	// signing in as a second AniList account read — and then overwrote — the first account's
	// collection at the same key.
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollection(context.Background(), new("first"))
	require.NoError(t, err)

	client.animeCollection = newTestAnimeCollection(202, 654, anilist.MediaListStatusPlanning, 0)
	_, err = cacheLayer.AnimeCollection(context.Background(), new("second"))
	require.NoError(t, err)

	firstKey := cacheLayer.collectionCacheKey("collection", new("first"))
	secondKey := cacheLayer.collectionCacheKey("collection", new("second"))
	require.NotEqual(t, firstKey, secondKey)

	var firstCached anilist.AnimeCollection
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[AnimeCollectionBucket], firstKey, &firstCached)
	require.NoError(t, err)
	require.True(t, found)
	_, found = firstCached.GetListEntryFromAnimeId(101)
	require.True(t, found, "the first account's collection should survive the second account's fetch")
	_, found = firstCached.GetListEntryFromAnimeId(202)
	require.False(t, found)
}

func TestClearAccountCachesEmptiesCollectionBuckets(t *testing.T) {
	client := &cacheLayerTestClient{
		cacheDir:        t.TempDir(),
		animeCollection: newTestAnimeCollection(101, 321, anilist.MediaListStatusCurrent, 2),
	}
	cacheLayer := newTestCacheLayer(t, client)

	_, err := cacheLayer.AnimeCollection(context.Background(), new("first"))
	require.NoError(t, err)

	cacheKey := cacheLayer.collectionCacheKey("collection", new("first"))
	var cached anilist.AnimeCollection
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[AnimeCollectionBucket], cacheKey, &cached)
	require.NoError(t, err)
	require.True(t, found)

	cacheLayer.ClearAccountCaches()

	found, err = cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[AnimeCollectionBucket], cacheKey, &cached)
	require.NoError(t, err)
	require.False(t, found, "a previous account's collection should not outlive the logout")
}

func TestCustomQueryCacheKeyDependsOnToken(t *testing.T) {
	// The key used to be the raw request body, so a query issued under one account's credentials
	// could be served a response cached under another's.
	cacheLayer := newTestCacheLayer(t, &cacheLayerTestClient{cacheDir: t.TempDir()})
	body := []byte(`{"query":"{ Viewer { id } }"}`)

	first := cacheLayer.customQueryCacheKey(body, []string{"token-a"})
	second := cacheLayer.customQueryCacheKey(body, []string{"token-b"})
	require.NotEqual(t, first, second)
	require.Equal(t, first, cacheLayer.customQueryCacheKey(body, []string{"token-a"}))

	// The key becomes a filename, so it must never carry the token or the query itself.
	require.NotContains(t, first, "token-a")
	require.NotContains(t, first, "Viewer")
}

func TestExtractBaseAnimeFallsBackToTheRelationsCollection(t *testing.T) {
	// The relations bucket is written under its own key prefix. Reusing the plain collection key
	// meant this fallback could never hit, leaving it dead code on the offline path.
	client := &cacheLayerTestClient{cacheDir: t.TempDir()}
	cacheLayer := newTestCacheLayer(t, client)
	cacheLayer.rememberCollectionUser(new("first"))

	relCollection := &anilist.AnimeCollectionWithRelations{
		MediaListCollection: &anilist.AnimeCollectionWithRelations_MediaListCollection{
			Lists: []*anilist.AnimeCollectionWithRelations_MediaListCollection_Lists{
				{
					Status: new(anilist.MediaListStatusCurrent),
					Entries: []*anilist.AnimeCollectionWithRelations_MediaListCollection_Lists_Entries{
						{ID: 555, Media: &anilist.CompleteAnime{ID: 909}},
					},
				},
			},
		},
	}
	require.NoError(t, cacheLayer.fileCacher.SetPerm(
		cacheLayer.buckets[AnimeCollectionRelationsBucket],
		cacheLayer.collectionCacheKey("collection-relations", nil),
		relCollection,
	))

	res := cacheLayer.extractBaseAnimeFromCollection(909)
	require.NotNil(t, res)
	require.Equal(t, 909, res.GetMedia().GetID())
}

func TestQueuedUpdateIsDroppedAfterTooManyAttempts(t *testing.T) {
	// A queued update AniList will never accept used to retry every five minutes forever, taking
	// the sync lock on each attempt.
	previousEventManager := events.GlobalWSEventManager
	events.GlobalWSEventManager = &events.GlobalWSEventManagerWrapper{}
	t.Cleanup(func() { events.GlobalWSEventManager = previousEventManager })

	cacheLayer := newTestCacheLayer(t, &cacheLayerTestClient{cacheDir: t.TempDir()})
	syncErr := errors.New("AniList said no")

	queued, err := cacheLayer.saveQueuedMediaListUpdate(queuedMediaListUpdate{
		MediaID:  101,
		Progress: new(3),
	})
	require.NoError(t, err)

	for range queueMaxAttempts - 1 {
		cacheLayer.setQueuedUpdateSyncFailed(queued, syncErr)
		current := getQueuedUpdate(t, cacheLayer, 101)
		require.NotNil(t, current.NextAttemptAt)
		// setQueuedUpdateSyncFailed only acts on the entry it was handed, so carry the stored one
		// forward the way the sync loop does.
		queued = current
	}

	cacheLayer.setQueuedUpdateSyncFailed(queued, syncErr)

	var dropped queuedMediaListUpdate
	found, err := cacheLayer.fileCacher.GetPerm(cacheLayer.buckets[PendingMediaListUpdatesBucket], "101", &dropped)
	require.NoError(t, err)
	require.False(t, found, "the update should have been abandoned rather than retried forever")
}

func TestShouldQueueMediaListUpdateUsesTheTransportedStatus(t *testing.T) {
	// A media id in the message ("14042") used to read as a 404 and stop a real failure being
	// queued; a 404 AniList actually replied with still must not be queued.
	require.True(t, shouldQueueMediaListUpdate(errors.New("failed to update media 14042: connection reset")))
	require.False(t, shouldQueueMediaListUpdate(anilistErrorWithStatus(404, "not found")))
	require.False(t, shouldQueueMediaListUpdate(anilistErrorWithStatus(400, "bad request")))
	require.True(t, shouldQueueMediaListUpdate(anilistErrorWithStatus(429, "rate limited")))
	require.True(t, shouldQueueMediaListUpdate(anilistErrorWithStatus(503, "unavailable")))
}
