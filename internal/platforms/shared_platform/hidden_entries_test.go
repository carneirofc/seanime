package shared_platform

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/util"
	"testing"

	"github.com/gqlgo/gqlgenc/clientv2"
)

type hiddenEntriesTestClient struct {
	anilist.AnilistClient
	pages    [][]*anilist.AnimeListEntriesNotIn_Page_MediaList
	err      error
	calls    int
	excluded [][]*int
}

func (c *hiddenEntriesTestClient) AnimeListEntriesNotIn(_ context.Context, _ *string, excludedMediaIds []*int, page *int, _ *int, _ ...clientv2.RequestInterceptor) (*anilist.AnimeListEntriesNotIn, error) {
	c.calls++
	c.excluded = append(c.excluded, excludedMediaIds)
	if c.err != nil {
		return nil, c.err
	}

	idx := 0
	if page != nil {
		idx = *page - 1
	}
	var entries []*anilist.AnimeListEntriesNotIn_Page_MediaList
	if idx >= 0 && idx < len(c.pages) {
		entries = c.pages[idx]
	}
	hasNext := idx+1 < len(c.pages)

	return &anilist.AnimeListEntriesNotIn{
		Page: &anilist.AnimeListEntriesNotIn_Page{
			MediaList: entries,
			PageInfo:  &anilist.AnimeListEntriesNotIn_Page_PageInfo{HasNextPage: &hasNext},
		},
	}, nil
}

var hiddenEntriesUser = "tester"

func hiddenEntriesHelper() *PlatformHelper {
	return &PlatformHelper{logger: util.NewLogger()}
}

func hiddenEntriesCollection() *anilist.AnimeCollection {
	status := anilist.MediaListStatusCurrent
	name := "Watching"
	isCustom := false
	return &anilist.AnimeCollection{
		MediaListCollection: &anilist.AnimeCollection_MediaListCollection{
			Lists: []*anilist.AnimeCollection_MediaListCollection_Lists{
				{
					Status:       &status,
					Name:         &name,
					IsCustomList: &isCustom,
					Entries: []*anilist.AnimeListEntry{
						{ID: 10, Media: &anilist.BaseAnime{ID: 1}, Status: &status},
					},
				},
			},
		},
	}
}

func hiddenEntry(mediaID int, status anilist.MediaListStatus) *anilist.AnimeListEntriesNotIn_Page_MediaList {
	hidden := true
	return &anilist.AnimeListEntriesNotIn_Page_MediaList{
		ID:                    mediaID * 10,
		Media:                 &anilist.BaseAnime{ID: mediaID},
		Status:                &status,
		HiddenFromStatusLists: &hidden,
	}
}

func entryCount(collection *anilist.AnimeCollection) int {
	n := 0
	for _, list := range collection.MediaListCollection.Lists {
		n += len(list.Entries)
	}
	return n
}

func TestReconcileHiddenAnimeEntries(t *testing.T) {
	t.Run("stops after one request when nothing is missing", func(t *testing.T) {
		client := &hiddenEntriesTestClient{pages: nil}
		collection := hiddenEntriesCollection()

		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), collection, client, &hiddenEntriesUser); merged != 0 {
			t.Fatalf("expected nothing merged, got %d", merged)
		}
		if client.calls != 1 {
			t.Fatalf("expected exactly one request, got %d", client.calls)
		}
		if entryCount(collection) != 1 {
			t.Fatalf("the collection was modified: %d entries", entryCount(collection))
		}
	})

	t.Run("excludes the media already in the collection", func(t *testing.T) {
		client := &hiddenEntriesTestClient{}
		hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), hiddenEntriesCollection(), client, &hiddenEntriesUser)

		if len(client.excluded) != 1 || len(client.excluded[0]) != 1 || *client.excluded[0][0] != 1 {
			t.Fatalf("expected media 1 to be excluded, got %v", client.excluded)
		}
	})

	t.Run("merges recovered entries into their status group", func(t *testing.T) {
		client := &hiddenEntriesTestClient{pages: [][]*anilist.AnimeListEntriesNotIn_Page_MediaList{
			{hiddenEntry(2, anilist.MediaListStatusCurrent), hiddenEntry(3, anilist.MediaListStatusCompleted)},
		}}
		collection := hiddenEntriesCollection()

		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), collection, client, &hiddenEntriesUser); merged != 2 {
			t.Fatalf("expected 2 merged, got %d", merged)
		}
		if _, found := collection.GetListEntryFromAnimeId(2); !found {
			t.Fatal("media 2 did not reach the collection")
		}
		if _, found := collection.GetListEntryFromAnimeId(3); !found {
			t.Fatal("media 3 did not reach the collection")
		}
		if len(collection.MediaListCollection.Lists) != 2 {
			t.Fatalf("expected a completed group to be created, got %d groups", len(collection.MediaListCollection.Lists))
		}
	})

	t.Run("follows hasNextPage", func(t *testing.T) {
		client := &hiddenEntriesTestClient{pages: [][]*anilist.AnimeListEntriesNotIn_Page_MediaList{
			{hiddenEntry(2, anilist.MediaListStatusCurrent)},
			{hiddenEntry(3, anilist.MediaListStatusCurrent)},
		}}
		collection := hiddenEntriesCollection()

		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), collection, client, &hiddenEntriesUser); merged != 2 {
			t.Fatalf("expected 2 merged across pages, got %d", merged)
		}
		if client.calls != 2 {
			t.Fatalf("expected 2 requests, got %d", client.calls)
		}
	})

	t.Run("skips entries with no status", func(t *testing.T) {
		client := &hiddenEntriesTestClient{pages: [][]*anilist.AnimeListEntriesNotIn_Page_MediaList{
			{{ID: 20, Media: &anilist.BaseAnime{ID: 2}}},
		}}
		collection := hiddenEntriesCollection()

		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), collection, client, &hiddenEntriesUser); merged != 0 {
			t.Fatalf("expected nothing merged, got %d", merged)
		}
		if entryCount(collection) != 1 {
			t.Fatalf("a statusless entry reached the collection: %d entries", entryCount(collection))
		}
	})

	t.Run("swallows a client error and leaves the collection alone", func(t *testing.T) {
		client := &hiddenEntriesTestClient{err: errors.New("anilist is down")}
		collection := hiddenEntriesCollection()

		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), collection, client, &hiddenEntriesUser); merged != 0 {
			t.Fatalf("expected nothing merged, got %d", merged)
		}
		if entryCount(collection) != 1 {
			t.Fatalf("the collection was modified: %d entries", entryCount(collection))
		}
	})

	t.Run("is a no-op without a collection", func(t *testing.T) {
		client := &hiddenEntriesTestClient{}
		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), nil, client, &hiddenEntriesUser); merged != 0 {
			t.Fatalf("expected nothing merged, got %d", merged)
		}
		if client.calls != 0 {
			t.Fatalf("expected no request, got %d", client.calls)
		}
	})
}

func TestReconcileHiddenAnimeEntriesSkipsWithoutAUser(t *testing.T) {
	client := &hiddenEntriesTestClient{}
	empty := ""

	for _, userName := range []*string{nil, &empty} {
		if merged := hiddenEntriesHelper().ReconcileHiddenAnimeEntries(context.Background(), hiddenEntriesCollection(), client, userName); merged != 0 {
			t.Fatalf("expected nothing merged, got %d", merged)
		}
	}
	if client.calls != 0 {
		t.Fatalf("expected no request without a user, got %d", client.calls)
	}
}
