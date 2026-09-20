package anilist

import "testing"

func listGroupsPtr[T any](v T) *T { return &v }

func animeEntry(mediaID int, status MediaListStatus) *AnimeListEntry {
	return &AnimeListEntry{
		ID:     mediaID * 10,
		Media:  &BaseAnime{ID: mediaID},
		Status: listGroupsPtr(status),
	}
}

func animeStatusGroup(status MediaListStatus, entries ...*AnimeListEntry) *AnimeList {
	return &AnimeList{
		Status:       listGroupsPtr(status),
		Name:         listGroupsPtr(animeStatusListName(status)),
		IsCustomList: listGroupsPtr(false),
		Entries:      entries,
	}
}

func animeCustomGroup(name string, entries ...*AnimeListEntry) *AnimeList {
	return &AnimeList{
		Name:         listGroupsPtr(name),
		IsCustomList: listGroupsPtr(true),
		Entries:      entries,
	}
}

func mediaIDsIn(t *testing.T, lists []*AnimeList, status MediaListStatus) []int {
	t.Helper()
	for _, list := range lists {
		if list.Status != nil && *list.Status == status {
			ids := make([]int, 0, len(list.Entries))
			for _, e := range list.Entries {
				ids = append(ids, e.Media.ID)
			}
			return ids
		}
	}
	return nil
}

func TestFoldAnimeCustomLists(t *testing.T) {
	t.Run("folds a custom-list-only entry into its own status group", func(t *testing.T) {
		lists := []*AnimeList{
			animeStatusGroup(MediaListStatusCurrent, animeEntry(1, MediaListStatusCurrent)),
			animeCustomGroup("Hidden", animeEntry(2, MediaListStatusCompleted)),
			animeStatusGroup(MediaListStatusCompleted, animeEntry(3, MediaListStatusCompleted)),
		}

		got := FoldAnimeCustomLists(lists)

		if ids := mediaIDsIn(t, got, MediaListStatusCompleted); len(ids) != 2 || ids[0] != 3 || ids[1] != 2 {
			t.Fatalf("expected media 3 and 2 in the completed group, got %v", ids)
		}
		if ids := mediaIDsIn(t, got, MediaListStatusCurrent); len(ids) != 1 || ids[0] != 1 {
			t.Fatalf("expected only media 1 in the current group, got %v", ids)
		}
	})

	t.Run("does not duplicate an entry filed in both a status and a custom group", func(t *testing.T) {
		shared := animeEntry(1, MediaListStatusCurrent)
		lists := []*AnimeList{
			animeStatusGroup(MediaListStatusCurrent, shared),
			animeCustomGroup("Favourites", shared),
		}

		got := FoldAnimeCustomLists(lists)

		if ids := mediaIDsIn(t, got, MediaListStatusCurrent); len(ids) != 1 {
			t.Fatalf("expected media 1 exactly once, got %v", ids)
		}
	})

	t.Run("creates the status group when AniList returned none", func(t *testing.T) {
		lists := []*AnimeList{
			animeCustomGroup("Hidden", animeEntry(2, MediaListStatusDropped)),
		}

		got := FoldAnimeCustomLists(lists)

		if len(got) != 1 {
			t.Fatalf("expected one group, got %d", len(got))
		}
		if got[0].Status == nil || *got[0].Status != MediaListStatusDropped {
			t.Fatalf("expected a dropped group, got %+v", got[0].Status)
		}
		if got[0].IsCustomList == nil || *got[0].IsCustomList {
			t.Fatal("the created group must not be marked as a custom list")
		}
		if got[0].Name == nil || *got[0].Name != "Dropped" {
			t.Fatalf("expected the group to be named Dropped, got %v", got[0].Name)
		}
	})

	t.Run("drops entries with no status instead of panicking", func(t *testing.T) {
		lists := []*AnimeList{
			animeCustomGroup("Hidden", &AnimeListEntry{ID: 20, Media: &BaseAnime{ID: 2}}),
		}

		if got := FoldAnimeCustomLists(lists); len(got) != 0 {
			t.Fatalf("expected no group, got %d", len(got))
		}
	})

	t.Run("never returns a group without a status", func(t *testing.T) {
		lists := []*AnimeList{
			animeStatusGroup(MediaListStatusCurrent, animeEntry(1, MediaListStatusCurrent)),
			animeCustomGroup("Hidden", animeEntry(2, MediaListStatusCurrent)),
			nil,
		}

		for _, list := range FoldAnimeCustomLists(lists) {
			if list == nil || list.Status == nil {
				t.Fatal("a group with no status survived the fold")
			}
		}
	})

	t.Run("leaves the source groups untouched", func(t *testing.T) {
		source := animeStatusGroup(MediaListStatusCurrent, animeEntry(1, MediaListStatusCurrent))
		lists := []*AnimeList{
			source,
			animeCustomGroup("Hidden", animeEntry(2, MediaListStatusCurrent)),
		}

		FoldAnimeCustomLists(lists)

		if len(source.Entries) != 1 {
			t.Fatalf("the raw collection's group was mutated: %d entries", len(source.Entries))
		}
	})
}

func TestFoldMangaCustomLists(t *testing.T) {
	entry := &MangaListEntry{
		ID:     20,
		Media:  &BaseManga{ID: 2},
		Status: listGroupsPtr(MediaListStatusCurrent),
	}
	lists := []*MangaList{
		{Name: listGroupsPtr("Hidden"), IsCustomList: listGroupsPtr(true), Entries: []*MangaListEntry{entry}},
	}

	got := FoldMangaCustomLists(lists)

	if len(got) != 1 || got[0].Status == nil || *got[0].Status != MediaListStatusCurrent {
		t.Fatalf("expected the entry folded into a reading group, got %+v", got)
	}
	if got[0].Name == nil || *got[0].Name != "Reading" {
		t.Fatalf("expected the manga group to be named Reading, got %v", got[0].Name)
	}
}

func TestAnimeListEntryFromMediaList(t *testing.T) {
	src := &AnimeListEntriesNotIn_Page_MediaList{
		ID:                    99,
		Progress:              listGroupsPtr(5),
		Status:                listGroupsPtr(MediaListStatusCurrent),
		Private:               listGroupsPtr(true),
		HiddenFromStatusLists: listGroupsPtr(true),
		Media:                 &BaseAnime{ID: 7},
		StartedAt:             &AnimeListEntriesNotIn_Page_MediaList_StartedAt{Year: listGroupsPtr(2024), Month: listGroupsPtr(3), Day: listGroupsPtr(9)},
	}

	got := AnimeListEntryFromMediaList(src)

	if got.ID != 99 || got.Media.ID != 7 || got.GetProgressSafe() != 5 {
		t.Fatalf("scalars were not copied: %+v", got)
	}
	if got.HiddenFromStatusLists == nil || !*got.HiddenFromStatusLists {
		t.Fatal("hiddenFromStatusLists was not copied")
	}
	if got.StartedAt == nil || got.StartedAt.Year == nil || *got.StartedAt.Year != 2024 || *got.StartedAt.Day != 9 {
		t.Fatalf("startedAt was not rebuilt: %+v", got.StartedAt)
	}
	if got.CompletedAt != nil {
		t.Fatal("completedAt should stay nil when the source has none")
	}
	if AnimeListEntryFromMediaList(nil) != nil {
		t.Fatal("a nil source must convert to nil")
	}
}
