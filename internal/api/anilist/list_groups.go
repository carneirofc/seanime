package anilist

// AniList groups a media list by status, but an entry flagged hiddenFromStatusLists is
// filed under the user's custom lists instead of its status group — and under nothing at
// all when the user keeps no custom list. Seanime only ever consumes the status groups
// (see the nil-status guards in internal/library/anime/collection.go and
// internal/local/sync.go), so the helpers below put such entries back where the rest of
// the app can find them.

// animeStatusListName is the label AniList itself gives a status group. Only used when
// Seanime has to create a group AniList did not return.
func animeStatusListName(status MediaListStatus) string {
	switch status {
	case MediaListStatusCurrent:
		return "Watching"
	case MediaListStatusPlanning:
		return "Planning"
	case MediaListStatusCompleted:
		return "Completed"
	case MediaListStatusDropped:
		return "Dropped"
	case MediaListStatusPaused:
		return "Paused"
	case MediaListStatusRepeating:
		return "Rewatching"
	}
	return string(status)
}

func mangaStatusListName(status MediaListStatus) string {
	switch status {
	case MediaListStatusCurrent:
		return "Reading"
	case MediaListStatusRepeating:
		return "Rereading"
	}
	return animeStatusListName(status)
}

// AppendAnimeEntryToStatusList appends entry to the status group matching its own status,
// creating that group when the collection does not have one. Entries without a status are
// dropped: there is no group they could belong to. The (possibly extended) slice is
// returned, like append.
func AppendAnimeEntryToStatusList(lists []*AnimeList, entry *AnimeListEntry) []*AnimeList {
	if entry == nil || entry.Status == nil {
		return lists
	}

	for _, list := range lists {
		if list != nil && list.Status != nil && *list.Status == *entry.Status {
			list.Entries = append(list.Entries, entry)
			return lists
		}
	}

	status := *entry.Status
	name := animeStatusListName(status)
	isCustomList := false
	return append(lists, &AnimeList{
		Status:       &status,
		Name:         &name,
		IsCustomList: &isCustomList,
		Entries:      []*AnimeListEntry{entry},
	})
}

// AppendMangaEntryToStatusList is the manga counterpart of AppendAnimeEntryToStatusList.
func AppendMangaEntryToStatusList(lists []*MangaList, entry *MangaListEntry) []*MangaList {
	if entry == nil || entry.Status == nil {
		return lists
	}

	for _, list := range lists {
		if list != nil && list.Status != nil && *list.Status == *entry.Status {
			list.Entries = append(list.Entries, entry)
			return lists
		}
	}

	status := *entry.Status
	name := mangaStatusListName(status)
	isCustomList := false
	return append(lists, &MangaList{
		Status:       &status,
		Name:         &name,
		IsCustomList: &isCustomList,
		Entries:      []*MangaListEntry{entry},
	})
}

// FoldAnimeCustomLists moves entries that AniList returned only inside a custom list into
// the status group matching their own status, then returns the status groups alone. A media
// already present in a status group is left untouched, so an entry filed in both places is
// not duplicated.
func FoldAnimeCustomLists(lists []*AnimeList) []*AnimeList {
	statusLists := make([]*AnimeList, 0, len(lists))
	customLists := make([]*AnimeList, 0)
	seen := make(map[int]struct{})

	for _, list := range lists {
		if list == nil {
			continue
		}
		if list.Status == nil {
			customLists = append(customLists, list)
			continue
		}
		// Shallow-copy the group before folding anything into it. The caller hands the same
		// group pointers to both the filtered and the raw collection, and the raw one must
		// keep AniList's own shape — a custom-list entry appearing in its custom list only.
		copied := *list
		copied.Entries = append([]*AnimeListEntry(nil), list.Entries...)
		statusLists = append(statusLists, &copied)
		for _, entry := range list.Entries {
			if entry != nil && entry.Media != nil {
				seen[entry.Media.ID] = struct{}{}
			}
		}
	}

	for _, list := range customLists {
		for _, entry := range list.Entries {
			if entry == nil || entry.Media == nil {
				continue
			}
			if _, ok := seen[entry.Media.ID]; ok {
				continue
			}
			seen[entry.Media.ID] = struct{}{}
			statusLists = AppendAnimeEntryToStatusList(statusLists, entry)
		}
	}

	return statusLists
}

// FoldMangaCustomLists is the manga counterpart of FoldAnimeCustomLists.
func FoldMangaCustomLists(lists []*MangaList) []*MangaList {
	statusLists := make([]*MangaList, 0, len(lists))
	customLists := make([]*MangaList, 0)
	seen := make(map[int]struct{})

	for _, list := range lists {
		if list == nil {
			continue
		}
		if list.Status == nil {
			customLists = append(customLists, list)
			continue
		}
		// Shallow-copy the group before folding anything into it. The caller hands the same
		// group pointers to both the filtered and the raw collection, and the raw one must
		// keep AniList's own shape — a custom-list entry appearing in its custom list only.
		copied := *list
		copied.Entries = append([]*MangaListEntry(nil), list.Entries...)
		statusLists = append(statusLists, &copied)
		for _, entry := range list.Entries {
			if entry != nil && entry.Media != nil {
				seen[entry.Media.ID] = struct{}{}
			}
		}
	}

	for _, list := range customLists {
		for _, entry := range list.Entries {
			if entry == nil || entry.Media == nil {
				continue
			}
			if _, ok := seen[entry.Media.ID]; ok {
				continue
			}
			seen[entry.Media.ID] = struct{}{}
			statusLists = AppendMangaEntryToStatusList(statusLists, entry)
		}
	}

	return statusLists
}
