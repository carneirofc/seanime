package shared_platform

import (
	"context"
	"seanime/internal/api/anilist"
	"seanime/internal/customsource"
)

// AniList's MediaListCollection query does not return every list entry. An entry flagged
// hiddenFromStatusLists is filed under the user's custom lists instead of its status group,
// and an entry that is hidden while belonging to no custom list is not returned at all —
// so it is invisible to Seanime no matter how the response is walked.
//
// Page.mediaList has no such blind spot and accepts mediaId_not_in, so asking for "the
// entries whose media is not one of these" costs a single request when nothing is missing,
// and returns the stragglers complete with their media when something is.
//
// Note that pageInfo.total is unreliable under that filter (AniList reports a constant
// 5000), so the loop below is driven by the returned slice and hasNextPage alone.
const (
	hiddenEntriesPerPage  = 50
	hiddenEntriesMaxPages = 20
)

// ReconcileHiddenAnimeEntries merges the anime list entries that MediaListCollection left
// out into collection, and reports how many it added. It never fails the refresh: on error
// the caller keeps the collection it already has.
func (h *PlatformHelper) ReconcileHiddenAnimeEntries(ctx context.Context, collection *anilist.AnimeCollection, client anilist.AnilistClient, userName *string) int {
	if collection == nil || collection.MediaListCollection == nil || client == nil {
		return 0
	}
	// No user to ask about — the fixture client leaves the name unset.
	if userName == nil || *userName == "" {
		return 0
	}

	excluded := make([]*int, 0)
	for _, list := range collection.MediaListCollection.Lists {
		if list == nil {
			continue
		}
		for _, entry := range list.Entries {
			if entry == nil || entry.Media == nil || customsource.IsExtensionId(entry.Media.ID) {
				continue
			}
			id := entry.Media.ID
			excluded = append(excluded, &id)
		}
	}

	merged := 0
	perPage := hiddenEntriesPerPage

	for page := 1; page <= hiddenEntriesMaxPages; page++ {
		currentPage := page
		res, err := client.AnimeListEntriesNotIn(ctx, userName, excluded, &currentPage, &perPage)
		if err != nil {
			h.logger.Warn().Err(err).Msg("anilist: Failed to look up anime list entries missing from the collection")
			return merged
		}

		entries := res.GetPage().GetMediaList()
		if len(entries) == 0 {
			return merged
		}

		for _, src := range entries {
			entry := anilist.AnimeListEntryFromMediaList(src)
			if entry == nil || entry.Media == nil || entry.Status == nil {
				continue
			}
			collection.MediaListCollection.Lists = anilist.AppendAnimeEntryToStatusList(collection.MediaListCollection.Lists, entry)
			merged++
		}

		if hasNext := res.GetPage().GetPageInfo().GetHasNextPage(); hasNext == nil || !*hasNext {
			return merged
		}
	}

	h.logger.Warn().Int("pages", hiddenEntriesMaxPages).Msg("anilist: Stopped reconciling hidden anime entries at the page cap")
	return merged
}

// ReconcileHiddenMangaEntries is the manga counterpart of ReconcileHiddenAnimeEntries.
func (h *PlatformHelper) ReconcileHiddenMangaEntries(ctx context.Context, collection *anilist.MangaCollection, client anilist.AnilistClient, userName *string) int {
	if collection == nil || collection.MediaListCollection == nil || client == nil {
		return 0
	}
	// No user to ask about — the fixture client leaves the name unset.
	if userName == nil || *userName == "" {
		return 0
	}

	excluded := make([]*int, 0)
	for _, list := range collection.MediaListCollection.Lists {
		if list == nil {
			continue
		}
		for _, entry := range list.Entries {
			if entry == nil || entry.Media == nil || customsource.IsExtensionId(entry.Media.ID) {
				continue
			}
			id := entry.Media.ID
			excluded = append(excluded, &id)
		}
	}

	merged := 0
	perPage := hiddenEntriesPerPage

	for page := 1; page <= hiddenEntriesMaxPages; page++ {
		currentPage := page
		res, err := client.MangaListEntriesNotIn(ctx, userName, excluded, &currentPage, &perPage)
		if err != nil {
			h.logger.Warn().Err(err).Msg("anilist: Failed to look up manga list entries missing from the collection")
			return merged
		}

		entries := res.GetPage().GetMediaList()
		if len(entries) == 0 {
			return merged
		}

		for _, src := range entries {
			entry := anilist.MangaListEntryFromMediaList(src)
			if entry == nil || entry.Media == nil || entry.Status == nil {
				continue
			}
			collection.MediaListCollection.Lists = anilist.AppendMangaEntryToStatusList(collection.MediaListCollection.Lists, entry)
			merged++
		}

		if hasNext := res.GetPage().GetPageInfo().GetHasNextPage(); hasNext == nil || !*hasNext {
			return merged
		}
	}

	h.logger.Warn().Int("pages", hiddenEntriesMaxPages).Msg("anilist: Stopped reconciling hidden manga entries at the page cap")
	return merged
}
