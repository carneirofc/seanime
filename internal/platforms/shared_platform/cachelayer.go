package shared_platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"seanime/internal/api/anilist"
	"seanime/internal/events"
	"seanime/internal/util"
	"seanime/internal/util/filecache"
	"seanime/internal/util/result"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gqlgo/gqlgenc/clientv2"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

// devnote: I got lazy and used global variables

var ShouldCache = atomic.Bool{}
var IsWorking = atomic.Bool{}
var AnilistClient = atomic.Value{}

type failureRecord struct {
	timestamp time.Time
	err       error
}

var (
	failureTracking      = make([]failureRecord, 0)
	failureTrackingMutex sync.RWMutex
)

const (
	failureWindow     = 30 * time.Second // time window to consider failures
	failureThreshold  = 4                // number of failures needed to mark as down
	cleanupInterval   = 5 * time.Minute  // how often to clean up old failure records
	maxFailureRecords = 50               // maximum number of failure records to keep

	// authFailureLogoutThreshold is how many consecutive auth-shaped failures it takes to end the
	// session. Logging out blanks the stored account row, so it should not hinge on a single reply.
	authFailureLogoutThreshold = 2
)

// consecutiveAuthFailures counts auth-shaped failures since the last successful request.
var consecutiveAuthFailures atomic.Int64

func init() {
	ShouldCache.Store(true)
	IsWorking.Store(true)

	go func() {
		// Every 10 seconds, check if the AniList client is working
		for {
			time.Sleep(time.Second * 10)
			if !ShouldCache.Load() {
				IsWorking.Store(true)
				continue
			}
			if IsWorking.Load() {
				continue
			}
			if AnilistClient.Load() == nil {
				IsWorking.Store(true)
				continue
			}
			anilistClient, ok := AnilistClient.Load().(anilist.AnilistClient)
			if !ok {
				IsWorking.Store(true)
				continue
			}
			_, err := anilistClient.BaseAnimeByID(context.Background(), new(1))
			if err != nil {
				IsWorking.Store(false)
			} else {
				clearFailureTracking()
				events.GlobalWSEventManager.SendEvent(events.InfoToast, "The AniList API is back online")
				IsWorking.Store(true)
			}
		}
	}()

	// periodic cleanup of old failure records
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupOldFailures()
		}
	}()
}

type (
	// CacheLayer is a "network-first" wrapper around an AniList client that caches fetched data in cache files.
	// It detects when the API client is not working and falls back to the cached data instead.
	// When the API client not working, it will still send the requests in the background and transition back to working state when the API client is working again.
	// Entry/progress updates are queued when the API client is not working; other mutations return an error.
	// Caching strategy:
	// - All queries to a specific media that IS in the anime collection or manga collection will be always cached/updated without limit
	// - Media that are NOT in the anime or manga collection will be bounded to a maximum of 10 entries
	CacheLayer struct {
		anilistClientRef       *util.Ref[anilist.AnilistClient]
		fileCacher             *filecache.Cacher
		buckets                map[string]filecache.PermanentBucket
		logger                 *zerolog.Logger
		collectionMediaIDs     *result.Map[int, struct{}] // Track which media IDs are in collections
		collectionUserName     atomic.Value               // Last account a collection was fetched for (string)
		lastCollectionUpdate   atomic.Int64               // When collections were last fetched (unix nanos)
		collectionRefreshing   atomic.Bool                // Whether a collection-tracking refresh is in flight
		logoutFunc             func()                     // called when an invalid token is detected
		pendingUpdateSyncMutex sync.Mutex
		queueSyncInProgress    atomic.Bool // Whether a queued-update sync tick is running
	}
)

const (
	AnimeCollectionBucket          = "anime-collection"
	AnimeCollectionTagsBucket      = "anime-collection-tags"
	AnimeCollectionRelationsBucket = "anime-collection-relations"
	MangaCollectionBucket          = "manga-collection"
	MangaCollectionTagsBucket      = "manga-collection-tags"
	BaseAnimeBucket                = "base-anime"
	BaseAnimeMalBucket             = "base-anime-mal"
	CompleteAnimeBucket            = "complete-anime"
	AnimeDetailsBucket             = "anime-details"
	BaseMangaBucket                = "base-manga"
	MangaDetailsBucket             = "manga-details"
	ViewerBucket                   = "viewer"
	ViewerStatsBucket              = "viewer-stats"
	StudioDetailsBucket            = "studio-details"
	AnimeAiringScheduleBucket      = "anime-airing-schedule"
	AnimeAiringScheduleRawBucket   = "anime-airing-schedule-raw"
	ListAnimeBucket                = "list-anime"
	ListRecentAnimeBucket          = "list-recent-anime"
	SearchBaseMangaBucket          = "search-base-manga"
	ListMangaBucket                = "list-manga"
	SearchBaseAnimeByIdsBucket     = "search-base-anime-by-ids"
	CustomQueryBucket              = "custom-query"
	PendingMediaListUpdatesBucket  = "pending-media-list-updates"

	maxNonCollectionCacheEntries      = 10
	maxNonCollectionMediaCacheEntries = 50
	// Collection update interval (refresh collection tracking every 30 minutes)
	collectionUpdateInterval = 30 * time.Minute

	// defaultMediaReadCacheTTL bounds how long per-media reads (base anime/manga, details,
	// relations) are served straight from the file cache without contacting AniList.
	// Anime/manga metadata is effectively immutable, so a generous window avoids
	// hammering the API — and the 429s that come with it — during library scans,
	// which fan out a CompleteAnimeByID request for every node of every media tree.
	// Collections stay network-first, so list/progress data is always fresh — the one
	// exception being the collection *tag* maps, which are immutable metadata and ride
	// this same window rather than being refetched on every read.
	defaultMediaReadCacheTTL = 24 * time.Hour
)

// mediaReadCacheTTL holds the configured window in nanoseconds. It is the main lever against
// AniList's rate limit, so it is a setting rather than a constant; zero means the default.
var mediaReadCacheTTL atomic.Int64

// SetMediaReadCacheTTL applies the configured per-media cache window. A ttl of zero or less
// restores the default.
func SetMediaReadCacheTTL(ttl time.Duration) {
	if ttl <= 0 {
		mediaReadCacheTTL.Store(0)
		return
	}
	mediaReadCacheTTL.Store(int64(ttl))
}

// MediaReadCacheTTL returns the window currently in effect.
func MediaReadCacheTTL() time.Duration {
	if ttl := mediaReadCacheTTL.Load(); ttl > 0 {
		return time.Duration(ttl)
	}
	return defaultMediaReadCacheTTL
}

// addFailureRecord adds a new failure record to the tracking
func addFailureRecord(err error) {
	failureTrackingMutex.Lock()
	defer failureTrackingMutex.Unlock()

	now := time.Now()
	failureTracking = append(failureTracking, failureRecord{
		timestamp: now,
		err:       err,
	})

	// keep only the most recent records
	if len(failureTracking) > maxFailureRecords {
		failureTracking = failureTracking[len(failureTracking)-maxFailureRecords:]
	}
}

// getRecentFailureCount returns the number of failures within the failure window
func getRecentFailureCount() int {
	failureTrackingMutex.RLock()
	defer failureTrackingMutex.RUnlock()

	now := time.Now()
	cutoff := now.Add(-failureWindow)
	count := 0

	for _, record := range failureTracking {
		if record.timestamp.After(cutoff) {
			count++
		}
	}

	return count
}

// cleanupOldFailures removes failure records older than the failure window
func cleanupOldFailures() {
	failureTrackingMutex.Lock()
	defer failureTrackingMutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-failureWindow)
	validRecords := make([]failureRecord, 0, len(failureTracking))

	for _, record := range failureTracking {
		if record.timestamp.After(cutoff) {
			validRecords = append(validRecords, record)
		}
	}

	failureTracking = validRecords
}

// clearFailureTracking clears all failure records (called when API comes back online)
func clearFailureTracking() {
	failureTrackingMutex.Lock()
	defer failureTrackingMutex.Unlock()
	failureTracking = failureTracking[:0]
}

// NewCacheLayer returns a new instance of the global cache layer.
// An optional logoutFunc can be passed to perform server-side cleanup when an invalid token is detected.
func NewCacheLayer(anilistClientRef *util.Ref[anilist.AnilistClient], logoutFunc ...func()) anilist.AnilistClient {
	fileCacher, err := filecache.NewCacher(anilistClientRef.Get().GetCacheDir())
	if err != nil {
		return anilistClientRef.Get()
	}

	buckets := make(map[string]filecache.PermanentBucket)
	buckets[AnimeCollectionBucket] = filecache.NewPermanentBucket(AnimeCollectionBucket)
	buckets[AnimeCollectionTagsBucket] = filecache.NewPermanentBucket(AnimeCollectionTagsBucket)
	buckets[AnimeCollectionRelationsBucket] = filecache.NewPermanentBucket(AnimeCollectionRelationsBucket)
	buckets[MangaCollectionBucket] = filecache.NewPermanentBucket(MangaCollectionBucket)
	buckets[MangaCollectionTagsBucket] = filecache.NewPermanentBucket(MangaCollectionTagsBucket)
	buckets[BaseAnimeBucket] = filecache.NewPermanentBucket(BaseAnimeBucket)
	buckets[BaseAnimeMalBucket] = filecache.NewPermanentBucket(BaseAnimeMalBucket)
	buckets[CompleteAnimeBucket] = filecache.NewPermanentBucket(CompleteAnimeBucket)
	buckets[AnimeDetailsBucket] = filecache.NewPermanentBucket(AnimeDetailsBucket)
	buckets[BaseMangaBucket] = filecache.NewPermanentBucket(BaseMangaBucket)
	buckets[MangaDetailsBucket] = filecache.NewPermanentBucket(MangaDetailsBucket)
	buckets[ViewerBucket] = filecache.NewPermanentBucket(ViewerBucket)
	buckets[ViewerStatsBucket] = filecache.NewPermanentBucket(ViewerStatsBucket)
	buckets[StudioDetailsBucket] = filecache.NewPermanentBucket(StudioDetailsBucket)
	buckets[AnimeAiringScheduleBucket] = filecache.NewPermanentBucket(AnimeAiringScheduleBucket)
	buckets[AnimeAiringScheduleRawBucket] = filecache.NewPermanentBucket(AnimeAiringScheduleRawBucket)
	buckets[ListAnimeBucket] = filecache.NewPermanentBucket(ListAnimeBucket)
	buckets[ListRecentAnimeBucket] = filecache.NewPermanentBucket(ListRecentAnimeBucket)
	buckets[SearchBaseMangaBucket] = filecache.NewPermanentBucket(SearchBaseMangaBucket)
	buckets[ListMangaBucket] = filecache.NewPermanentBucket(ListMangaBucket)
	buckets[SearchBaseAnimeByIdsBucket] = filecache.NewPermanentBucket(SearchBaseAnimeByIdsBucket)
	buckets[CustomQueryBucket] = filecache.NewPermanentBucket(CustomQueryBucket)
	buckets[PendingMediaListUpdatesBucket] = filecache.NewPermanentBucket(PendingMediaListUpdatesBucket)

	logger := util.NewLogger()

	var logout func()
	if len(logoutFunc) > 0 {
		logout = logoutFunc[0]
	}

	cl := &CacheLayer{
		anilistClientRef:   anilistClientRef,
		fileCacher:         fileCacher,
		buckets:            buckets,
		logger:             logger,
		collectionMediaIDs: result.NewMap[int, struct{}](),
		logoutFunc:         logout,
	}

	AnilistClient.Store(anilistClientRef.Get())
	cl.startQueuedUpdateSync()

	return cl
}

var _ anilist.AnilistClient = (*CacheLayer)(nil)

func (c *CacheLayer) IsAuthenticated() bool {
	return c.anilistClientRef.Get().IsAuthenticated()
}

func (c *CacheLayer) GetCacheDir() string {
	return c.anilistClientRef.Get().GetCacheDir()
}

func (c *CacheLayer) CustomQuery(body []byte, logger *zerolog.Logger, token ...string) (interface{}, error) {
	cacheKey := c.customQueryCacheKey(body, token)
	bucket := c.buckets[CustomQueryBucket]

	// Try network first if API is working
	if IsWorking.Load() {
		res, err := c.anilistClientRef.Get().CustomQuery(body, logger, token...)
		c.checkAndUpdateWorkingState(err)

		if err == nil {
			go func() {
				if !ShouldCache.Load() {
					return
				}
				allData, err := filecache.GetAll[interface{}](c.fileCacher, filecache.NewBucket(bucket.Name(), 0))
				if err == nil && len(allData) >= maxNonCollectionCacheEntries {
					_ = c.fileCacher.DeletePermOldest(bucket)
				}

				if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
					c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache custom query result")
				}
			}()
			return res, nil
		}
	} else {
		// If API is not working, try it in the background to check if it's back
		go func() {
			res, err := c.anilistClientRef.Get().CustomQuery(body, logger, token...)
			c.checkAndUpdateWorkingState(err)
			if err == nil {
				// Cache the result for future use with bounded size
				allData, err := filecache.GetAll[interface{}](c.fileCacher, filecache.NewBucket(bucket.Name(), 0))
				if err == nil && len(allData) >= maxNonCollectionCacheEntries {
					_ = c.fileCacher.DeletePermOldest(bucket)
				}

				if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
					c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache background custom query result")
				}
			}
		}()
	}

	// Fall back to cache
	var cached interface{}
	found, err := c.fileCacher.GetPerm(bucket, cacheKey, &cached)
	if err != nil {
		return nil, fmt.Errorf("cache lookup failed: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("no cached data available")
	}

	c.logger.Debug().Str("bucket", CustomQueryBucket).Str("key", cacheKey).Msg("anilist cache: Serving custom query from cache")
	return cached, nil
}

// checkAndUpdateWorkingState checks if the API client is working and updates the state
func (c *CacheLayer) checkAndUpdateWorkingState(err error) {
	if err != nil {
		// Skip context.Canceled errors, not indicative of API issues
		if errors.Is(err, context.Canceled) {
			return
		}

		// The auth check comes first: AniList answers a dead token with a 400, which the skip
		// below would otherwise swallow before the session ever gets ended.
		if isAnilistAuthError(err) {
			// One auth-shaped error is not enough to end a session. AniList returns 400 with an
			// "Invalid token" body for a genuinely dead token, which is indistinguishable from a
			// transient server-side rejection, so a logout waits for a second consecutive one.
			// Any successful request in between resets the count.
			failures := consecutiveAuthFailures.Add(1)
			if failures < authFailureLogoutThreshold {
				c.logger.Warn().Err(err).Int64("consecutive", failures).
					Msg("anilist cache: AniList reported an auth error, waiting for confirmation before logging out")
				return
			}

			c.logger.Warn().Err(err).Msg("anilist cache: AniList reported an auth error, treating token as invalid and logging out")
			events.GlobalWSEventManager.SendEvent(events.ServerLoggedOutAnilist, "Your AniList session has expired. Please log in again.")
			if c.logoutFunc != nil {
				go c.logoutFunc()
			}
			return
		}

		// A reply AniList actually sent says nothing about whether the API is reachable:
		// 404 is a missing media, 429 is the rate limiter doing its job, and 400 is a malformed
		// request — counting the latter would let one client-side bug repeated across a batch of
		// entries trip the failure threshold and drop the whole integration into cache-only mode.
		// These are matched on the transported status rather than on the error text, because a
		// media id or title in the message ("14042", "Room 404") matched the old substring test.
		if anilistStatusIn(err, http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadRequest) {
			return
		}

		// Add failure to tracking
		addFailureRecord(err)

		// Only mark as down if we have enough recent failures and are currently marked as working
		if IsWorking.Load() {
			recentFailures := getRecentFailureCount()
			if recentFailures >= failureThreshold {
				c.logger.Warn().
					Err(err).
					Int("recent_failures", recentFailures).
					Dur("within_window", failureWindow).
					Msg("anilist cache: Multiple API failures detected, switching to cache-only mode.")
				events.GlobalWSEventManager.SendEvent(events.WarningToast,
					fmt.Sprintf("The AniList API is experiencing issues (%d failures in %v), switching to cache-only mode.",
						recentFailures, failureWindow))
				IsWorking.Store(false)
			} else {
				c.logger.Debug().
					Err(err).
					Int("recent_failures", recentFailures).
					Int("threshold", failureThreshold).
					Msg("anilist cache: API failure recorded, monitoring for more failures")
			}
		}
	} else {
		consecutiveAuthFailures.Store(0)

		// clear failure tracking and mark as working if not already
		if !IsWorking.Load() {
			c.logger.Info().Msg("anilist cache: API client is working again, switching back to network-first mode.")
			events.GlobalWSEventManager.SendEvent(events.InfoToast, "The AniList API is back online")
			IsWorking.Store(true)
			clearFailureTracking()
			return
		}

		// Only records that have aged out of the window are dropped. Clearing the whole window on
		// every success meant that under partial degradation — the case the breaker exists for —
		// any interleaved success reset the count and the threshold was never reached.
		cleanupOldFailures()
	}
}

// anilistHTTPStatus reports the HTTP status AniList replied with, when the error carries one.
// parseResponse wraps every non-2xx reply in a *clientv2.ErrorResponse, so this is exact where
// matching on the error text is not.
func anilistHTTPStatus(err error) (int, bool) {
	var resp *clientv2.ErrorResponse
	if errors.As(err, &resp) && resp.NetworkError != nil {
		return resp.NetworkError.Code, true
	}
	return 0, false
}

// anilistStatusIn reports whether err carries an HTTP status AniList actually replied with and
// that status is one of the given codes. It is exact where matching on the error text is not.
func anilistStatusIn(err error, codes ...int) bool {
	code, ok := anilistHTTPStatus(err)
	if !ok {
		return false
	}
	return slices.Contains(codes, code)
}

// isAnilistAuthError reports whether AniList rejected the request because the token is no longer
// good. The message match alone is not enough — a transient error whose body happens to quote
// "invalid token" would end the session — so it must be paired with a status AniList itself sent:
// 401, or the 400 AniList returns for a dead token. An error carrying no HTTP status is a transport
// failure and never an auth failure.
func isAnilistAuthError(err error) bool {
	if err == nil {
		return false
	}

	code, ok := anilistHTTPStatus(err)
	if !ok {
		return false
	}
	if code == http.StatusUnauthorized {
		return true
	}
	if code != http.StatusBadRequest && code != http.StatusForbidden {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "invalid token") || strings.Contains(errStr, "user not found")
}

// generateCacheKey generates a cache key from the given parameters
func (c *CacheLayer) generateCacheKey(params ...interface{}) string {
	var keyParts []string
	for _, param := range params {
		if param == nil {
			keyParts = append(keyParts, "nil")
			continue
		}
		switch v := param.(type) {
		case *int:
			if v != nil {
				keyParts = append(keyParts, strconv.Itoa(*v))
			} else {
				keyParts = append(keyParts, "nil")
			}
		case *string:
			if v != nil {
				keyParts = append(keyParts, *v)
			} else {
				keyParts = append(keyParts, "nil")
			}
		case *bool:
			if v != nil {
				keyParts = append(keyParts, strconv.FormatBool(*v))
			} else {
				keyParts = append(keyParts, "nil")
			}
		case []*int:
			tmp := make([]int, 0, len(v))
			for _, id := range v {
				if id != nil {
					tmp = append(tmp, *id)
				}
			}
			slices.Sort(tmp)
			for _, id := range tmp {
				keyParts = append(keyParts, strconv.Itoa(id))
			}
		case []*string:
			tmp := make([]string, 0, len(v))
			for _, s := range v {
				if s != nil {
					tmp = append(tmp, *s)
				}
			}
			slices.Sort(tmp)
			keyParts = append(keyParts, tmp...)
		default:
			keyParts = append(keyParts, fmt.Sprintf("%v", param))
		}
	}
	return lo.Reduce(keyParts, func(acc, item string, _ int) string {
		if acc == "" {
			return item
		}
		return acc + "-" + item
	}, "")
}

// setLastCollectionUpdate / lastCollectionUpdateTime guard the collection-tracking timestamp.
// It is read on the request path and written from the refresh goroutine, which was a plain data
// race on a time.Time field before.
func (c *CacheLayer) setLastCollectionUpdate(at time.Time) {
	if at.IsZero() {
		c.lastCollectionUpdate.Store(0)
		return
	}
	c.lastCollectionUpdate.Store(at.UnixNano())
}

func (c *CacheLayer) lastCollectionUpdateTime() time.Time {
	nanos := c.lastCollectionUpdate.Load()
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// rememberCollectionUser records which account the collections belong to, so the offline
// extract-from-collection fallbacks can rebuild the same cache key from a media id alone.
func (c *CacheLayer) rememberCollectionUser(userName *string) {
	if userName != nil && *userName != "" {
		c.collectionUserName.Store(*userName)
	}
}

// collectionCacheKey builds the key for a whole-collection bucket. The account has to be part of
// it: the file cache lives in one per-install directory, so a key that ignored userName let a
// second AniList account read and overwrite the first account's collection at the same key.
// A nil userName falls back to the last account a collection was fetched for.
func (c *CacheLayer) collectionCacheKey(prefix string, userName *string) string {
	if userName != nil && *userName != "" {
		return c.generateCacheKey(prefix, userName)
	}
	if remembered, ok := c.collectionUserName.Load().(string); ok && remembered != "" {
		return c.generateCacheKey(prefix, &remembered)
	}
	return c.generateCacheKey(prefix, nil)
}

// customQueryCacheKey keys a custom query on the token it is sent with as well as its body.
// The token is hashed, never stored: the key becomes a filename. Without the token in the key a
// query issued under one account's credentials could be served a response cached under another's.
func (c *CacheLayer) customQueryCacheKey(body []byte, token []string) string {
	hash := sha256.New()
	if len(token) > 0 && token[0] != "" {
		hash.Write([]byte(token[0]))
	} else if remembered, ok := c.collectionUserName.Load().(string); ok {
		hash.Write([]byte(remembered))
	}
	hash.Write([]byte{0})
	hash.Write(body)
	return hex.EncodeToString(hash.Sum(nil))
}

// ClearAccountCaches empties every bucket holding account-scoped data. It is called on login and
// logout: the cache keys are per-account, so a switch is already correct without this, but a
// previous account's whole collection would otherwise sit on disk forever.
func (c *CacheLayer) ClearAccountCaches() {
	c.collectionUserName.Store("")

	for _, bucketName := range []string{
		AnimeCollectionBucket,
		AnimeCollectionTagsBucket,
		AnimeCollectionRelationsBucket,
		MangaCollectionBucket,
		MangaCollectionTagsBucket,
		ViewerBucket,
		ViewerStatsBucket,
		CustomQueryBucket,
	} {
		if err := c.fileCacher.EmptyPerm(c.buckets[bucketName]); err != nil {
			c.logger.Warn().Err(err).Str("bucket", bucketName).Msg("anilist cache: Failed to clear account cache")
		}
	}

	c.collectionMediaIDs.Clear()
	c.setLastCollectionUpdate(time.Time{})
}

// isInCollection checks if a media ID is in the user's collection
func (c *CacheLayer) isInCollection(mediaID int) bool {
	// Update collection tracking if needed
	c.updateCollectionTracking()
	_, ok := c.collectionMediaIDs.Get(mediaID)
	return ok
}

// updateCollectionTracking updates the collection media IDs tracking.
//
// The timestamp is stamped before the fetch starts, not after it finishes, and a refresh is
// single-flighted. Stamping afterwards meant every caller arriving during the fetch also passed
// the interval check and launched its own full anime + manga collection request — a stampede that
// landed precisely during a library scan, which calls isInCollection once per media.
func (c *CacheLayer) updateCollectionTracking() {
	if time.Since(c.lastCollectionUpdateTime()) < collectionUpdateInterval {
		return
	}

	if !c.collectionRefreshing.CompareAndSwap(false, true) {
		return
	}
	c.setLastCollectionUpdate(time.Now())

	go func() {
		defer c.collectionRefreshing.Store(false)

		// Try to fetch anime collection
		if animeCollection, err := c.anilistClientRef.Get().AnimeCollection(context.Background(), nil); err == nil && animeCollection != nil {
			for _, list := range animeCollection.MediaListCollection.Lists {
				if list != nil {
					for _, entry := range list.Entries {
						if entry != nil && entry.Media != nil {
							c.collectionMediaIDs.Set(entry.Media.ID, struct{}{})
						}
					}
				}
			}
		}

		// Try to fetch manga collection
		if mangaCollection, err := c.anilistClientRef.Get().MangaCollection(context.Background(), nil); err == nil && mangaCollection != nil {
			for _, list := range mangaCollection.MediaListCollection.Lists {
				if list != nil {
					for _, entry := range list.Entries {
						if entry != nil && entry.Media != nil {
							c.collectionMediaIDs.Set(entry.Media.ID, struct{}{})
						}
					}
				}
			}
		}
	}()
}

// cacheFirstGet serves a recent cached value without contacting AniList when one
// exists (fresh within mediaReadCacheTTL), otherwise it falls back to the
// network-first path. This is used for immutable per-media reads so repeated
// access (most importantly library scans fanning out media tree requests) does
// not generate redundant AniList traffic and trip rate limiting (429).
func cacheFirstGet[T any](c *CacheLayer, bucketName string, cacheKey string, networkFn func() (*T, error)) (*T, error) {
	if ShouldCache.Load() {
		var cached T
		found, err := c.fileCacher.GetPermFresh(c.buckets[bucketName], cacheKey, &cached, MediaReadCacheTTL())
		if err == nil && found {
			c.logger.Trace().Str("bucket", bucketName).Str("key", cacheKey).Msg("anilist cache: Serving fresh entry from cache (cache-first)")
			return &cached, nil
		}
	}
	return networkFirstGet(c, bucketName, cacheKey, networkFn)
}

// networkFirstGet performs a network-first get operation with caching
func networkFirstGet[T any](c *CacheLayer, bucketName string, cacheKey string, networkFn func() (*T, error)) (*T, error) {
	if !ShouldCache.Load() {
		return networkFn()
	}

	bucket := c.buckets[bucketName]

	// Try network first if API is working
	if IsWorking.Load() {
		res, err := networkFn()
		c.checkAndUpdateWorkingState(err)

		if err == nil && res != nil {
			// Cache the successful result
			if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache result")
			}
			return res, nil
		}
	} else {
		// If API is not working, try it in the background to check if it's back
		go func() {
			res, err := networkFn()
			c.checkAndUpdateWorkingState(err)
			if err == nil && res != nil {
				// Cache the result for future use
				if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
					c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache background result")
				}
			}
		}()
	}

	// Fall back to cache
	var cached T
	found, err := c.fileCacher.GetPerm(bucket, cacheKey, &cached)
	if err != nil {
		return nil, fmt.Errorf("cache lookup failed: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("no cached data available")
	}

	c.logger.Debug().Str("bucket", bucketName).Str("key", cacheKey).Msg("anilist cache: Serving from cache")
	return &cached, nil
}

// boundedCacheSet caches data with a limit on non-collection entries
func (c *CacheLayer) boundedCacheSet(bucketName string, cacheKey string, data interface{}, mediaID int) error {
	if !ShouldCache.Load() {
		return nil
	}

	bucket := c.buckets[bucketName]

	// Always cache collection media
	if c.isInCollection(mediaID) {
		return c.fileCacher.SetPerm(bucket, cacheKey, data)
	}

	// For non-collection media, enforce the limit. Overwriting a key the bucket already holds
	// does not grow it, so the count only needs checking when the key is new.
	var existing interface{}
	if found, err := c.fileCacher.GetPerm(bucket, cacheKey, &existing); err == nil && found {
		return c.fileCacher.SetPerm(bucket, cacheKey, data)
	}

	count, err := c.fileCacher.CountPerm(bucket)
	if err != nil {
		return err
	}

	// Evict by age rather than by whichever key a map range happened to yield first — Go
	// randomizes that order, so the "FIFO" this replaces dropped an arbitrary entry.
	if count >= maxNonCollectionMediaCacheEntries {
		if err := c.fileCacher.DeletePermOldest(bucket); err != nil {
			c.logger.Debug().Err(err).Str("bucket", bucketName).Msg("anilist cache: Failed to evict the oldest bounded cache entry")
		}
	}

	return c.fileCacher.SetPerm(bucket, cacheKey, data)
}

// updateCollectionTrackingFromAnimeCollection updates collection tracking from anime collection
func (c *CacheLayer) updateCollectionTrackingFromAnimeCollection(collection *anilist.AnimeCollection) {
	if !ShouldCache.Load() {
		return
	}

	if !ShouldCache.Load() || collection == nil || collection.MediaListCollection == nil {
		return
	}

	for _, list := range collection.MediaListCollection.Lists {
		if list != nil {
			for _, entry := range list.Entries {
				if entry != nil && entry.Media != nil {
					c.collectionMediaIDs.Set(entry.Media.ID, struct{}{})
				}
			}
		}
	}
	c.setLastCollectionUpdate(time.Now())
}

func (c *CacheLayer) updateCollectionTrackingFromAnimeCollectionWithRelations(collection *anilist.AnimeCollectionWithRelations) {
	if !ShouldCache.Load() {
		return
	}

	if !ShouldCache.Load() || collection == nil || collection.MediaListCollection == nil {
		return
	}

	for _, list := range collection.MediaListCollection.Lists {
		if list != nil {
			for _, entry := range list.Entries {
				if entry != nil && entry.Media != nil {
					c.collectionMediaIDs.Set(entry.Media.ID, struct{}{})
				}
			}
		}
	}
	c.setLastCollectionUpdate(time.Now())
}

func (c *CacheLayer) updateCollectionTrackingFromMangaCollection(collection *anilist.MangaCollection) {
	if !ShouldCache.Load() {
		return
	}

	if !ShouldCache.Load() || collection == nil || collection.MediaListCollection == nil {
		return
	}

	for _, list := range collection.MediaListCollection.Lists {
		if list != nil {
			for _, entry := range list.Entries {
				if entry != nil && entry.Media != nil {
					c.collectionMediaIDs.Set(entry.Media.ID, struct{}{})
				}
			}
		}
	}
	c.setLastCollectionUpdate(time.Now())
}

// invalidateMediaCaches invalidates caches for a specific media ID
func (c *CacheLayer) invalidateMediaCaches(mediaID int) {
	if !ShouldCache.Load() {
		return
	}

	mediaIDStr := strconv.Itoa(mediaID)

	// Delete from all media-specific buckets
	buckets := []string{
		BaseAnimeBucket,
		CompleteAnimeBucket,
		AnimeDetailsBucket,
		BaseMangaBucket,
		MangaDetailsBucket,
	}

	for _, bucketName := range buckets {
		bucket := c.buckets[bucketName]
		if err := c.fileCacher.DeletePerm(bucket, mediaIDStr); err != nil {
			c.logger.Debug().Err(err).Str("bucket", bucketName).Int("mediaID", mediaID).Msg("anilist cache: Failed to invalidate cache entry")
		}
	}
}

// invalidateCollectionCaches invalidates all collection caches and custom queries
func (c *CacheLayer) invalidateCollectionCaches() {
	if !ShouldCache.Load() {
		return
	}

	// The tag buckets are deliberately absent. invalidateCollectionCaches runs after
	// every list-entry mutation, and a progress, score or repeat change cannot alter
	// any media's tag set — only collection *membership* changes do. Wiping them here
	// meant an episode-progress tick cost a second whole-collection AniList query on
	// the next request, which is a large part of why this client saw 429s. Newly
	// added media are picked up incrementally instead, by the reconcile in
	// anilist.ReconcileMediaTagMap. A deleted entry leaves a stale key in the map,
	// which is harmless: the map is only ever indexed by ids taken from the
	// collection being filtered, so an orphan key is never read.
	collectionBuckets := []string{
		AnimeCollectionBucket,
		AnimeCollectionRelationsBucket,
		MangaCollectionBucket,
		CustomQueryBucket,
	}

	for _, bucketName := range collectionBuckets {
		bucket := c.buckets[bucketName]
		if err := c.fileCacher.EmptyPerm(bucket); err != nil {
			c.logger.Warn().Err(err).Str("bucket", bucketName).Msg("anilist cache: Failed to invalidate collection cache")
		}
	}

	// Reset collection tracking
	c.collectionMediaIDs.Clear()
	c.setLastCollectionUpdate(time.Time{})
}

// extractBaseAnimeFromCollection attempts to extract BaseAnime data from cached anime collection
func (c *CacheLayer) extractBaseAnimeFromCollection(mediaID int) *anilist.BaseAnimeByID {
	// Try anime collection
	bucket := c.buckets[AnimeCollectionBucket]
	cacheKey := c.collectionCacheKey("collection", nil)
	var animeCollection anilist.AnimeCollection
	found, err := c.fileCacher.GetPerm(bucket, cacheKey, &animeCollection)
	if err == nil && found && animeCollection.MediaListCollection != nil {
		for _, list := range animeCollection.MediaListCollection.Lists {
			if list != nil {
				for _, entry := range list.Entries {
					if entry != nil && entry.Media != nil && entry.Media.ID == mediaID {
						return &anilist.BaseAnimeByID{
							Media: entry.Media,
						}
					}
				}
			}
		}
	}

	// Try anime collection with relations. Its bucket is written under its own prefix, so reusing
	// the key built above meant this lookup could never hit and the fallback was dead code.
	relBucket := c.buckets[AnimeCollectionRelationsBucket]
	relCacheKey := c.collectionCacheKey("collection-relations", nil)
	var animeCollectionRel anilist.AnimeCollectionWithRelations
	found, err = c.fileCacher.GetPerm(relBucket, relCacheKey, &animeCollectionRel)
	if err == nil && found && animeCollectionRel.MediaListCollection != nil {
		for _, list := range animeCollectionRel.MediaListCollection.Lists {
			if list != nil {
				for _, entry := range list.Entries {
					if entry != nil && entry.Media != nil && entry.Media.ID == mediaID {
						return &anilist.BaseAnimeByID{
							Media: entry.Media.ToBaseAnime(),
						}
					}
				}
			}
		}
	}

	return nil
}

// extractBaseMangaFromCollection attempts to extract BaseManga data from cached manga collection
func (c *CacheLayer) extractBaseMangaFromCollection(mediaID int) *anilist.BaseMangaByID {
	if !ShouldCache.Load() {
		return nil
	}

	bucket := c.buckets[MangaCollectionBucket]
	cacheKey := c.collectionCacheKey("collection", nil)
	var mangaCollection anilist.MangaCollection
	found, err := c.fileCacher.GetPerm(bucket, cacheKey, &mangaCollection)
	if err == nil && found && mangaCollection.MediaListCollection != nil {
		for _, list := range mangaCollection.MediaListCollection.Lists {
			if list != nil {
				for _, entry := range list.Entries {
					if entry != nil && entry.Media != nil && entry.Media.ID == mediaID {
						return &anilist.BaseMangaByID{
							Media: entry.Media,
						}
					}
				}
			}
		}
	}

	return nil
}

// networkFirstGetWithBoundedCache performs a network-first get operation with bounded caching for list/search results
func networkFirstGetWithBoundedCache[T any](c *CacheLayer, bucketName string, cacheKey string, networkFn func() (*T, error)) (*T, error) {
	bucket := c.buckets[bucketName]

	// Try network first if API is working
	if IsWorking.Load() {
		res, err := networkFn()
		c.checkAndUpdateWorkingState(err)

		if err == nil && res != nil {
			// Cache the successful result with bounded size
			go func() {
				// For list/search results, always apply bounded caching
				allData, err := filecache.GetAll[interface{}](c.fileCacher, filecache.NewBucket(bucket.Name(), 0))
				if err == nil && len(allData) >= maxNonCollectionCacheEntries {
					_ = c.fileCacher.DeletePermOldest(bucket)
				}

				if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
					c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache bounded result")
				}
			}()
			return res, nil
		}
	} else {
		// If API is not working, try it in the background to check if it's back
		go func() {
			res, err := networkFn()
			c.checkAndUpdateWorkingState(err)
			if err == nil && res != nil {
				// Cache the result for future use with bounded size
				allData, err := filecache.GetAll[interface{}](c.fileCacher, filecache.NewBucket(bucket.Name(), 0))
				if err == nil && len(allData) >= maxNonCollectionCacheEntries {
					_ = c.fileCacher.DeletePermOldest(bucket)
				}

				if err := c.fileCacher.SetPerm(bucket, cacheKey, res); err != nil {
					c.logger.Warn().Err(err).Msg("anilist cache: Failed to cache background bounded result")
				}
			}
		}()
	}

	// Fall back to cache
	var cached T
	found, err := c.fileCacher.GetPerm(bucket, cacheKey, &cached)
	if err != nil {
		return nil, fmt.Errorf("cache lookup failed: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("no cached data available")
	}

	c.logger.Debug().Str("bucket", bucketName).Str("key", cacheKey).Msg("anilist cache: Serving bounded result from cache")
	return &cached, nil
}

func (c *CacheLayer) AnimeCollection(ctx context.Context, userName *string, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeCollection, error) {
	c.rememberCollectionUser(userName)
	cacheKey := c.collectionCacheKey("collection", userName)
	res, err := networkFirstGet(c, AnimeCollectionBucket, cacheKey, func() (*anilist.AnimeCollection, error) {
		return c.anilistClientRef.Get().AnimeCollection(ctx, userName, interceptors...)
	})

	if err == nil && res != nil && c.applyQueuedUpdatesToAnimeCollection(res) {
		if err := c.fileCacher.SetPerm(c.buckets[AnimeCollectionBucket], cacheKey, res); err != nil {
			c.logger.Warn().Err(err).Msg("anilist cache: Failed to apply queued updates to anime collection cache")
		}
	}

	// Update collection tracking with the fetched data
	if err == nil && res != nil {
		go c.updateCollectionTrackingFromAnimeCollection(res)
	}

	return res, err
}

func (c *CacheLayer) AnimeCollectionTags(ctx context.Context, userName *string, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeCollectionTags, error) {
	cacheKey := c.collectionCacheKey("collection-tags", userName)
	return cacheFirstGet(c, AnimeCollectionTagsBucket, cacheKey, func() (*anilist.AnimeCollectionTags, error) {
		return c.anilistClientRef.Get().AnimeCollectionTags(ctx, userName, interceptors...)
	})
}

// GetMediaTagsByID is deliberately not cached. Its caller only ever asks for media ids
// missing from the tag map it is filling, so a cached response would essentially never
// be reused, while every distinct id set would occupy a permanent bucket entry forever.
// Availability is handled a level up: when this fails, the handler serves the tag map it
// already holds rather than failing the request.
func (c *CacheLayer) GetMediaTagsByID(ctx context.Context, ids []int, page *int, perPage *int, interceptors ...clientv2.RequestInterceptor) (*anilist.GetMediaTagsByID, error) {
	return c.anilistClientRef.Get().GetMediaTagsByID(ctx, ids, page, perPage, interceptors...)
}

// AnimeListEntriesNotIn and MangaListEntriesNotIn are deliberately not cached, for the same
// reason as GetMediaTagsByID: the exclusion set is derived from the collection that was just
// fetched, so it differs on nearly every call and a cached response would never be reused.
// A failure here is not fatal — the caller keeps the collection it already has.
func (c *CacheLayer) AnimeListEntriesNotIn(ctx context.Context, userName *string, excludedMediaIds []*int, page *int, perPage *int, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeListEntriesNotIn, error) {
	return c.anilistClientRef.Get().AnimeListEntriesNotIn(ctx, userName, excludedMediaIds, page, perPage, interceptors...)
}

func (c *CacheLayer) MangaListEntriesNotIn(ctx context.Context, userName *string, excludedMediaIds []*int, page *int, perPage *int, interceptors ...clientv2.RequestInterceptor) (*anilist.MangaListEntriesNotIn, error) {
	return c.anilistClientRef.Get().MangaListEntriesNotIn(ctx, userName, excludedMediaIds, page, perPage, interceptors...)
}

func (c *CacheLayer) AnimeCollectionWithRelations(ctx context.Context, userName *string, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeCollectionWithRelations, error) {
	c.rememberCollectionUser(userName)
	cacheKey := c.collectionCacheKey("collection-relations", userName)
	res, err := networkFirstGet(c, AnimeCollectionRelationsBucket, cacheKey, func() (*anilist.AnimeCollectionWithRelations, error) {
		return c.anilistClientRef.Get().AnimeCollectionWithRelations(ctx, userName, interceptors...)
	})

	// Update collection tracking with the fetched data
	if err == nil && res != nil {
		go c.updateCollectionTrackingFromAnimeCollectionWithRelations(res)
	}

	return res, err
}

func (c *CacheLayer) BaseAnimeByMalID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.BaseAnimeByMalID, error) {
	if id == nil {
		return c.anilistClientRef.Get().BaseAnimeByMalID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey("mal", id)
	return networkFirstGet(c, BaseAnimeMalBucket, cacheKey, func() (*anilist.BaseAnimeByMalID, error) {
		return c.anilistClientRef.Get().BaseAnimeByMalID(ctx, id, interceptors...)
	})
}

func (c *CacheLayer) BaseAnimeByID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.BaseAnimeByID, error) {
	if id == nil {
		return c.anilistClientRef.Get().BaseAnimeByID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	res, err := cacheFirstGet(c, BaseAnimeBucket, cacheKey, func() (*anilist.BaseAnimeByID, error) {
		return c.anilistClientRef.Get().BaseAnimeByID(ctx, id, interceptors...)
	})

	// If network and direct cache failed, try to extract from collection cache
	if err != nil {
		if collectionResult := c.extractBaseAnimeFromCollection(*id); collectionResult != nil {
			c.logger.Debug().Int("mediaID", *id).Msg("anilist cache: Extracted BaseAnime from collection cache")
			return collectionResult, nil
		}
	}

	// If successful, update bounded cache for non-collection media
	if err == nil && res != nil {
		go func() {
			if err := c.boundedCacheSet(BaseAnimeBucket, cacheKey, res, *id); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: Failed to update bounded cache")
			}
		}()
	}

	return res, err
}

func (c *CacheLayer) SearchBaseAnimeByIds(ctx context.Context, ids []*int, page *int, perPage *int, status []*anilist.MediaStatus, inCollection *bool, sort []*anilist.MediaSort, season *anilist.MediaSeason, year *int, genre *string, format *anilist.MediaFormat, interceptors ...clientv2.RequestInterceptor) (*anilist.SearchBaseAnimeByIds, error) {
	cacheKey := c.generateCacheKey(ids, page, perPage, status, inCollection, sort, season, year, genre, format)
	return networkFirstGetWithBoundedCache(c, SearchBaseAnimeByIdsBucket, cacheKey, func() (*anilist.SearchBaseAnimeByIds, error) {
		return c.anilistClientRef.Get().SearchBaseAnimeByIds(ctx, ids, page, perPage, status, inCollection, sort, season, year, genre, format, interceptors...)
	})
}

func (c *CacheLayer) CompleteAnimeByID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.CompleteAnimeByID, error) {
	if id == nil {
		return c.anilistClientRef.Get().CompleteAnimeByID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	res, err := cacheFirstGet(c, CompleteAnimeBucket, cacheKey, func() (*anilist.CompleteAnimeByID, error) {
		return c.anilistClientRef.Get().CompleteAnimeByID(ctx, id, interceptors...)
	})

	// If successful, update bounded cache for non-collection media
	if err == nil && res != nil {
		go func() {
			if err := c.boundedCacheSet(CompleteAnimeBucket, cacheKey, res, *id); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: failed to update bounded cache")
			}
		}()
	}

	return res, err
}

func (c *CacheLayer) AnimeDetailsByID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeDetailsByID, error) {
	if id == nil {
		return c.anilistClientRef.Get().AnimeDetailsByID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	res, err := cacheFirstGet(c, AnimeDetailsBucket, cacheKey, func() (*anilist.AnimeDetailsByID, error) {
		return c.anilistClientRef.Get().AnimeDetailsByID(ctx, id, interceptors...)
	})

	// If successful, update bounded cache for non-collection media
	if err == nil && res != nil {
		go func() {
			if err := c.boundedCacheSet(AnimeDetailsBucket, cacheKey, res, *id); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: failed to update bounded cache")
			}
		}()
	}

	return res, err
}

func (c *CacheLayer) ListAnime(ctx context.Context, page *int, search *string, perPage *int, sort []*anilist.MediaSort, status []*anilist.MediaStatus, genres []*string, tags []*string, averageScoreGreater *int, season *anilist.MediaSeason, seasonYear *int, format *anilist.MediaFormat, isAdult *bool, interceptors ...clientv2.RequestInterceptor) (*anilist.ListAnime, error) {
	cacheKey := c.generateCacheKey(page, search, perPage, sort, status, genres, averageScoreGreater, season, seasonYear, format, isAdult)
	return networkFirstGetWithBoundedCache(c, ListAnimeBucket, cacheKey, func() (*anilist.ListAnime, error) {
		return c.anilistClientRef.Get().ListAnime(ctx, page, search, perPage, sort, status, genres, tags, averageScoreGreater, season, seasonYear, format, isAdult, interceptors...)
	})
}

func (c *CacheLayer) ListRecentAnime(ctx context.Context, page *int, perPage *int, airingAtGreater *int, airingAtLesser *int, notYetAired *bool, interceptors ...clientv2.RequestInterceptor) (*anilist.ListRecentAnime, error) {
	// devnote: don't include airingAt params since they're unique for each requests, just return from the other params
	cacheKey := c.generateCacheKey(page, perPage, notYetAired)
	return networkFirstGetWithBoundedCache(c, ListRecentAnimeBucket, cacheKey, func() (*anilist.ListRecentAnime, error) {
		return c.anilistClientRef.Get().ListRecentAnime(ctx, page, perPage, airingAtGreater, airingAtLesser, notYetAired, interceptors...)
	})
}

func (c *CacheLayer) UpdateMediaListEntry(ctx context.Context, mediaID *int, status *anilist.MediaListStatus, scoreRaw *int, progress *int, startedAt *anilist.FuzzyDateInput, completedAt *anilist.FuzzyDateInput, private *bool, hiddenFromStatusLists *bool, interceptors ...clientv2.RequestInterceptor) (*anilist.UpdateMediaListEntry, error) {
	// Mutations require the API to be working
	if !IsWorking.Load() {
		entryID, err := c.queueMediaListEntryUpdate(mediaID, status, scoreRaw, progress, startedAt, completedAt, private, hiddenFromStatusLists)
		if err != nil {
			return nil, err
		}
		return &anilist.UpdateMediaListEntry{SaveMediaListEntry: &anilist.UpdateMediaListEntry_SaveMediaListEntry{ID: entryID}}, nil
	}

	res, err := c.sendMediaListEntryUpdate(ctx, mediaID, status, scoreRaw, progress, startedAt, completedAt, private, hiddenFromStatusLists, interceptors...)
	c.checkAndUpdateWorkingState(err)
	if err != nil && shouldQueueMediaListUpdate(err) {
		entryID, queueErr := c.queueMediaListEntryUpdate(mediaID, status, scoreRaw, progress, startedAt, completedAt, private, hiddenFromStatusLists)
		if queueErr != nil {
			return nil, queueErr
		}
		return &anilist.UpdateMediaListEntry{SaveMediaListEntry: &anilist.UpdateMediaListEntry_SaveMediaListEntry{ID: entryID}}, nil
	}

	// Invalidate relevant caches on successful mutation
	if err == nil && mediaID != nil {
		c.invalidateMediaCaches(*mediaID)
		c.invalidateCollectionCaches()
	}

	return res, err
}

func (c *CacheLayer) UpdateMediaListEntryProgress(ctx context.Context, mediaID *int, progress *int, status *anilist.MediaListStatus, interceptors ...clientv2.RequestInterceptor) (*anilist.UpdateMediaListEntryProgress, error) {
	// Mutations require the API to be working
	if !IsWorking.Load() {
		entryID, err := c.queueMediaListEntryProgressUpdate(mediaID, progress, status)
		if err != nil {
			return nil, err
		}
		return &anilist.UpdateMediaListEntryProgress{SaveMediaListEntry: &anilist.UpdateMediaListEntryProgress_SaveMediaListEntry{ID: entryID}}, nil
	}

	res, err := c.sendMediaListEntryProgressUpdate(ctx, mediaID, progress, status, interceptors...)
	c.checkAndUpdateWorkingState(err)
	if err != nil && shouldQueueMediaListUpdate(err) {
		entryID, queueErr := c.queueMediaListEntryProgressUpdate(mediaID, progress, status)
		if queueErr != nil {
			return nil, queueErr
		}
		return &anilist.UpdateMediaListEntryProgress{SaveMediaListEntry: &anilist.UpdateMediaListEntryProgress_SaveMediaListEntry{ID: entryID}}, nil
	}

	// Invalidate relevant caches on successful mutation
	if err == nil && mediaID != nil {
		c.invalidateMediaCaches(*mediaID)
		c.invalidateCollectionCaches()
	}

	return res, err
}

func (c *CacheLayer) UpdateMediaListEntryRepeat(ctx context.Context, mediaID *int, repeat *int, interceptors ...clientv2.RequestInterceptor) (*anilist.UpdateMediaListEntryRepeat, error) {
	// Mutations require the API to be working
	if !IsWorking.Load() {
		return nil, fmt.Errorf("anilist cache: API client is not working, mutation operations are not available")
	}

	res, err := c.anilistClientRef.Get().UpdateMediaListEntryRepeat(ctx, mediaID, repeat, interceptors...)
	c.checkAndUpdateWorkingState(err)

	// Invalidate relevant caches on successful mutation
	if err == nil && mediaID != nil {
		c.invalidateMediaCaches(*mediaID)
		c.invalidateCollectionCaches()
	}

	return res, err
}

func (c *CacheLayer) DeleteEntry(ctx context.Context, mediaListEntryID *int, interceptors ...clientv2.RequestInterceptor) (*anilist.DeleteEntry, error) {
	// Mutations require the API to be working
	if !IsWorking.Load() {
		return nil, fmt.Errorf("anilist cache: API client is not working, mutation operations are not available")
	}

	res, err := c.anilistClientRef.Get().DeleteEntry(ctx, mediaListEntryID, interceptors...)
	c.checkAndUpdateWorkingState(err)

	// Invalidate collection caches on successful deletion
	if err == nil {
		c.invalidateCollectionCaches()
	}

	return res, err
}

func (c *CacheLayer) MangaCollection(ctx context.Context, userName *string, interceptors ...clientv2.RequestInterceptor) (*anilist.MangaCollection, error) {
	c.rememberCollectionUser(userName)
	cacheKey := c.collectionCacheKey("collection", userName)
	res, err := networkFirstGet(c, MangaCollectionBucket, cacheKey, func() (*anilist.MangaCollection, error) {
		return c.anilistClientRef.Get().MangaCollection(ctx, userName, interceptors...)
	})

	if err == nil && res != nil && c.applyQueuedUpdatesToMangaCollection(res) {
		if err := c.fileCacher.SetPerm(c.buckets[MangaCollectionBucket], cacheKey, res); err != nil {
			c.logger.Warn().Err(err).Msg("anilist cache: Failed to apply queued updates to manga collection cache")
		}
	}

	// Update collection tracking with the fetched data
	if err == nil && res != nil {
		go c.updateCollectionTrackingFromMangaCollection(res)
	}

	return res, err
}

func (c *CacheLayer) MangaCollectionTags(ctx context.Context, userName *string, interceptors ...clientv2.RequestInterceptor) (*anilist.MangaCollectionTags, error) {
	cacheKey := c.collectionCacheKey("collection-tags", userName)
	return cacheFirstGet(c, MangaCollectionTagsBucket, cacheKey, func() (*anilist.MangaCollectionTags, error) {
		return c.anilistClientRef.Get().MangaCollectionTags(ctx, userName, interceptors...)
	})
}

func (c *CacheLayer) SearchBaseManga(ctx context.Context, page *int, perPage *int, sort []*anilist.MediaSort, search *string, status []*anilist.MediaStatus, interceptors ...clientv2.RequestInterceptor) (*anilist.SearchBaseManga, error) {
	cacheKey := c.generateCacheKey(page, perPage, sort, search, status)
	return networkFirstGetWithBoundedCache(c, SearchBaseMangaBucket, cacheKey, func() (*anilist.SearchBaseManga, error) {
		return c.anilistClientRef.Get().SearchBaseManga(ctx, page, perPage, sort, search, status, interceptors...)
	})
}

func (c *CacheLayer) BaseMangaByID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.BaseMangaByID, error) {
	if id == nil {
		return c.anilistClientRef.Get().BaseMangaByID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	res, err := cacheFirstGet(c, BaseMangaBucket, cacheKey, func() (*anilist.BaseMangaByID, error) {
		return c.anilistClientRef.Get().BaseMangaByID(ctx, id, interceptors...)
	})

	// If network and direct cache failed, try to extract from collection cache
	if err != nil {
		if collectionResult := c.extractBaseMangaFromCollection(*id); collectionResult != nil {
			c.logger.Debug().Int("mediaID", *id).Msg("anilist cache: Extracted BaseManga from collection cache")
			return collectionResult, nil
		}
	}

	// If successful, update bounded cache for non-collection media
	if err == nil && res != nil {
		go func() {
			if err := c.boundedCacheSet(BaseMangaBucket, cacheKey, res, *id); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: Failed to update bounded cache")
			}
		}()
	}

	return res, err
}

func (c *CacheLayer) MangaDetailsByID(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.MangaDetailsByID, error) {
	if id == nil {
		return c.anilistClientRef.Get().MangaDetailsByID(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	res, err := cacheFirstGet(c, MangaDetailsBucket, cacheKey, func() (*anilist.MangaDetailsByID, error) {
		return c.anilistClientRef.Get().MangaDetailsByID(ctx, id, interceptors...)
	})

	// If successful, update bounded cache for non-collection media
	if err == nil && res != nil {
		go func() {
			if err := c.boundedCacheSet(MangaDetailsBucket, cacheKey, res, *id); err != nil {
				c.logger.Warn().Err(err).Msg("anilist cache: failed to update bounded cache")
			}
		}()
	}

	return res, err
}

func (c *CacheLayer) ListManga(ctx context.Context, page *int, search *string, perPage *int, sort []*anilist.MediaSort, status []*anilist.MediaStatus, genres []*string, tags []*string, averageScoreGreater *int, startDateGreater *string, startDateLesser *string, format *anilist.MediaFormat, countryOfOrigin *string, isAdult *bool, interceptors ...clientv2.RequestInterceptor) (*anilist.ListManga, error) {
	cacheKey := c.generateCacheKey(page, search, perPage, sort, status, genres, tags, averageScoreGreater, startDateGreater, startDateLesser, format, countryOfOrigin, isAdult)
	return networkFirstGetWithBoundedCache(c, ListMangaBucket, cacheKey, func() (*anilist.ListManga, error) {
		return c.anilistClientRef.Get().ListManga(ctx, page, search, perPage, sort, status, genres, tags, averageScoreGreater, startDateGreater, startDateLesser, format, countryOfOrigin, isAdult, interceptors...)
	})
}

func (c *CacheLayer) ViewerStats(ctx context.Context, interceptors ...clientv2.RequestInterceptor) (*anilist.ViewerStats, error) {
	cacheKey := "stats"
	return networkFirstGet(c, ViewerStatsBucket, cacheKey, func() (*anilist.ViewerStats, error) {
		return c.anilistClientRef.Get().ViewerStats(ctx, interceptors...)
	})
}

func (c *CacheLayer) StudioDetails(ctx context.Context, id *int, interceptors ...clientv2.RequestInterceptor) (*anilist.StudioDetails, error) {
	if id == nil {
		return c.anilistClientRef.Get().StudioDetails(ctx, id, interceptors...)
	}

	cacheKey := c.generateCacheKey(id)
	return networkFirstGet(c, StudioDetailsBucket, cacheKey, func() (*anilist.StudioDetails, error) {
		return c.anilistClientRef.Get().StudioDetails(ctx, id, interceptors...)
	})
}

func (c *CacheLayer) GetViewer(ctx context.Context, interceptors ...clientv2.RequestInterceptor) (*anilist.GetViewer, error) {
	cacheKey := "viewer"
	return networkFirstGet(c, ViewerBucket, cacheKey, func() (*anilist.GetViewer, error) {
		return c.anilistClientRef.Get().GetViewer(ctx, interceptors...)
	})
}

func (c *CacheLayer) AnimeAiringSchedule(ctx context.Context, ids []*int, season *anilist.MediaSeason, seasonYear *int, previousSeason *anilist.MediaSeason, previousSeasonYear *int, nextSeason *anilist.MediaSeason, nextSeasonYear *int, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeAiringSchedule, error) {
	cacheKey := c.generateCacheKey(ids, season, seasonYear, previousSeason, previousSeasonYear, nextSeason, nextSeasonYear)
	return networkFirstGet(c, AnimeAiringScheduleBucket, cacheKey, func() (*anilist.AnimeAiringSchedule, error) {
		return c.anilistClientRef.Get().AnimeAiringSchedule(ctx, ids, season, seasonYear, previousSeason, previousSeasonYear, nextSeason, nextSeasonYear, interceptors...)
	})
}

func (c *CacheLayer) AnimeAiringScheduleRaw(ctx context.Context, ids []*int, interceptors ...clientv2.RequestInterceptor) (*anilist.AnimeAiringScheduleRaw, error) {
	cacheKey := c.generateCacheKey(ids)
	return networkFirstGet(c, AnimeAiringScheduleRawBucket, cacheKey, func() (*anilist.AnimeAiringScheduleRaw, error) {
		return c.anilistClientRef.Get().AnimeAiringScheduleRaw(ctx, ids, interceptors...)
	})
}
