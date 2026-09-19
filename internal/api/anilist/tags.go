package anilist

import (
	"context"
	"seanime/internal/util/result"
	"slices"
	"time"

	"github.com/rs/zerolog"
)

// MediaTagMap maps a media id to the names of the AniList tags on it. Only names are
// stored: every consumer filters by tag name, so the rest of the tag object would be
// dead weight in the cache and on the wire.
type MediaTagMap map[int][]string

const (
	// MediaTagsPerPage is AniList's maximum page size. GetMediaTagsByID requests must be
	// chunked to it, and the query selects pageInfo.hasNextPage so a regression here is
	// detectable rather than silent truncation.
	MediaTagsPerPage = 50

	// maxIncrementalTagFetches caps how many GetMediaTagsByID requests one reconcile may
	// issue. Past this point the single whole-collection query is the cheaper way to
	// rebuild the map: it is one request regardless of collection size, where chunked
	// fetching costs len(missing)/MediaTagsPerPage. The cap also stops a cold map from
	// bursting a dozen requests into AniList's rate-limit budget at once.
	maxIncrementalTagFetches = 2

	// collectionTagCacheTTL bounds how long an in-memory tag map is reused before being
	// rebuilt. Tags do change on AniList occasionally — the community edits them — so the
	// map must not live forever, but a rebuild is nearly free: it goes through the cache
	// layer, which serves the collection-tags query from disk for a further 24h before
	// contacting AniList again.
	collectionTagCacheTTL = time.Hour
)

// collectionTagCache holds the per-account tag maps behind the handlers that serve them.
// It is keyed by media type and username so switching AniList accounts cannot serve the
// previous account's tags, and it is a result.Cache rather than a bare package variable
// because the collection-refresh callbacks run on their own goroutines.
var collectionTagCache = result.NewCache[string, MediaTagMap]()

// AnimeTagCacheKey and MangaTagCacheKey namespace the two maps a single account has.
func AnimeTagCacheKey(userName string) string { return "anime:" + userName }
func MangaTagCacheKey(userName string) string { return "manga:" + userName }

func GetCollectionTagCache(key string) (MediaTagMap, bool) {
	return collectionTagCache.Get(key)
}

func SetCollectionTagCache(key string, val MediaTagMap) {
	collectionTagCache.SetT(key, val, collectionTagCacheTTL)
}

func ClearCollectionTagCache(key string) {
	collectionTagCache.Delete(key)
}

func MediaTagMapFromAnimeCollectionTags(data *AnimeCollectionTags) MediaTagMap {
	ret := make(MediaTagMap)
	if data == nil || data.GetMediaListCollection() == nil {
		return ret
	}

	for _, list := range data.GetMediaListCollection().GetLists() {
		if list == nil {
			continue
		}
		for _, entry := range list.GetEntries() {
			if entry == nil || entry.GetMedia() == nil {
				continue
			}
			for _, tag := range entry.GetMedia().GetTags() {
				if tag == nil {
					continue
				}
				ret.add(entry.GetMedia().GetID(), tag.GetName())
			}
		}
	}

	return ret
}

func MediaTagMapFromMangaCollectionTags(data *MangaCollectionTags) MediaTagMap {
	ret := make(MediaTagMap)
	if data == nil || data.GetMediaListCollection() == nil {
		return ret
	}

	for _, list := range data.GetMediaListCollection().GetLists() {
		if list == nil {
			continue
		}
		for _, entry := range list.GetEntries() {
			if entry == nil || entry.GetMedia() == nil {
				continue
			}
			for _, tag := range entry.GetMedia().GetTags() {
				if tag == nil {
					continue
				}
				ret.add(entry.GetMedia().GetID(), tag.GetName())
			}
		}
	}

	return ret
}

func (m MediaTagMap) add(mediaID int, tagName string) {
	if tagName == "" {
		return
	}

	existing := m[mediaID]
	for _, current := range existing {
		if current == tagName {
			return
		}
	}

	m[mediaID] = append(existing, tagName)
}

// MissingIDs returns the ids present in the collection but absent from the map, sorted.
// A media with no tags at all is still recorded (as an empty slice) once fetched, so it
// is not reported as missing on every subsequent call.
func (m MediaTagMap) MissingIDs(collectionIDs []int) []int {
	missing := make([]int, 0)
	seen := make(map[int]struct{}, len(collectionIDs))

	for _, id := range collectionIDs {
		if id <= 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := m[id]; !ok {
			missing = append(missing, id)
		}
	}

	slices.Sort(missing)
	return missing
}

// ChunkMediaIDs splits ids into runs of at most MediaTagsPerPage, the largest page
// AniList will serve.
func ChunkMediaIDs(ids []int) [][]int {
	if len(ids) == 0 {
		return nil
	}

	chunks := make([][]int, 0, (len(ids)+MediaTagsPerPage-1)/MediaTagsPerPage)
	for start := 0; start < len(ids); start += MediaTagsPerPage {
		end := min(start+MediaTagsPerPage, len(ids))
		chunks = append(chunks, ids[start:end])
	}

	return chunks
}

// ReconcileMediaTagMap tops up m with the tags of any media in collectionIDs it does not
// already hold, and reports whether it changed.
//
// This is what makes a collection refresh cheap: a progress update or a score change
// leaves every id already in the map, so the common case issues no AniList request at
// all. Only a media that genuinely entered the collection costs anything, and then only
// one small query per MediaTagsPerPage ids.
//
// It reports needsFullRefetch when the delta is large enough that the single
// whole-collection tags query would be cheaper than chunked fetching; the caller should
// then rebuild the map from scratch rather than call this again.
func ReconcileMediaTagMap(
	ctx context.Context,
	client AnilistClient,
	logger *zerolog.Logger,
	m MediaTagMap,
	collectionIDs []int,
) (changed bool, needsFullRefetch bool, err error) {
	missing := m.MissingIDs(collectionIDs)
	if len(missing) == 0 {
		return false, false, nil
	}

	chunks := ChunkMediaIDs(missing)
	if len(chunks) > maxIncrementalTagFetches {
		return false, true, nil
	}

	perPage := MediaTagsPerPage
	page := 1

	for _, chunk := range chunks {
		res, fetchErr := client.GetMediaTagsByID(ctx, chunk, &page, &perPage)
		if fetchErr != nil {
			return changed, false, fetchErr
		}
		if res == nil || res.GetPage() == nil {
			continue
		}

		if pageInfo := res.GetPage().GetPageInfo(); pageInfo != nil && pageInfo.GetHasNextPage() != nil && *pageInfo.GetHasNextPage() {
			// Unreachable while chunks are capped at MediaTagsPerPage. If it ever fires,
			// AniList lowered its page cap and the map is now silently incomplete.
			logger.Warn().Int("chunk", len(chunk)).Msg("anilist: Media tags response was paginated, tag map may be incomplete")
		}

		for _, media := range res.GetPage().GetMedia() {
			if media == nil {
				continue
			}
			// Record the id even when it has no tags, so it stops counting as missing.
			if _, ok := m[media.GetID()]; !ok {
				m[media.GetID()] = []string{}
			}
			for _, tag := range media.GetTags() {
				if tag == nil {
					continue
				}
				m.add(media.GetID(), tag.GetName())
			}
			changed = true
		}
	}

	return changed, false, nil
}
