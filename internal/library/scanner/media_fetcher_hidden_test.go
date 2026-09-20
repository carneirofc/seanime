package scanner

import (
	"seanime/internal/api/anilist"
	"seanime/internal/customsource"
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCollectMissingCollectionMedia covers the entries AniList keeps out of a MediaListCollection
// response. The plain collection has them put back by ReconcileHiddenAnimeEntries; the collection
// with relations that the scanner queries does not, so they have to be picked up from the plain one
// or the matcher never sees them.
func TestCollectMissingCollectionMedia(t *testing.T) {
	customSourceId := customsource.GenerateMediaId(1, 7)

	entry := func(id int) *anilist.AnimeCollection_MediaListCollection_Lists_Entries {
		return &anilist.AnimeCollection_MediaListCollection_Lists_Entries{
			ID:    id,
			Media: &anilist.BaseAnime{ID: id},
		}
	}
	list := func(entries ...*anilist.AnimeCollection_MediaListCollection_Lists_Entries) *anilist.AnimeCollection_MediaListCollection_Lists {
		return &anilist.AnimeCollection_MediaListCollection_Lists{Entries: entries}
	}
	collection := func(lists ...*anilist.AnimeCollection_MediaListCollection_Lists) *anilist.AnimeCollection {
		return &anilist.AnimeCollection{
			MediaListCollection: &anilist.AnimeCollection_MediaListCollection{Lists: lists},
		}
	}

	tests := []struct {
		name string
		// collection is the plain (reconciled) collection
		collection *anilist.AnimeCollection
		// knownIds are the media the collection with relations already returned
		knownIds []int
		// expectedIds are the media ids expected back, in order
		expectedIds []int
		// expectedRecovered counts only the AniList entries, not the custom source ones
		expectedRecovered int
	}{
		{
			name:              "an entry hidden from the status lists is recovered",
			collection:        collection(list(entry(100), entry(200))),
			knownIds:          []int{100},
			expectedIds:       []int{200},
			expectedRecovered: 1,
		},
		{
			name:              "custom source entries come back but are not counted as recovered",
			collection:        collection(list(entry(100), entry(customSourceId))),
			knownIds:          []int{100},
			expectedIds:       []int{customSourceId},
			expectedRecovered: 0,
		},
		{
			name:              "nothing is returned when the collections agree",
			collection:        collection(list(entry(100), entry(200))),
			knownIds:          []int{100, 200},
			expectedIds:       []int{},
			expectedRecovered: 0,
		},
		{
			name:              "an entry listed twice is only taken once",
			collection:        collection(list(entry(200)), list(entry(200))),
			knownIds:          []int{},
			expectedIds:       []int{200},
			expectedRecovered: 1,
		},
		{
			name:              "nil lists and entries are skipped",
			collection:        collection(nil, list(nil, entry(200))),
			knownIds:          []int{},
			expectedIds:       []int{200},
			expectedRecovered: 1,
		},
		{
			name:              "no collection at all",
			collection:        nil,
			knownIds:          []int{100},
			expectedIds:       []int{},
			expectedRecovered: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			knownIds := make(map[int]struct{}, len(tt.knownIds))
			for _, id := range tt.knownIds {
				knownIds[id] = struct{}{}
			}

			extra, recovered := collectMissingCollectionMedia(tt.collection, knownIds)

			gotIds := make([]int, 0, len(extra))
			for _, m := range extra {
				gotIds = append(gotIds, m.ID)
			}

			assert.Equal(t, tt.expectedIds, gotIds)
			assert.Equal(t, tt.expectedRecovered, recovered, "recovered AniList entries")

			// Everything taken is recorded, so a later pass cannot take it again
			for _, id := range gotIds {
				assert.Containsf(t, knownIds, id, "media %d should have been marked as known", id)
			}
		})
	}
}

// TestCollectMissingCollectionMediaReachesTheMatcher checks the recovered media actually become
// matchable candidates, which is the point of recovering them.
func TestCollectMissingCollectionMediaReachesTheMatcher(t *testing.T) {
	hidden := &anilist.BaseAnime{
		ID: 200,
		Title: &anilist.BaseAnime_Title{
			Romaji:  new("Hidden Adult Show"),
			English: new("Hidden Adult Show"),
		},
		IsAdult: new(true),
	}

	collection := &anilist.AnimeCollection{
		MediaListCollection: &anilist.AnimeCollection_MediaListCollection{
			Lists: []*anilist.AnimeCollection_MediaListCollection_Lists{
				{
					Entries: []*anilist.AnimeCollection_MediaListCollection_Lists_Entries{
						{ID: 1, Media: hidden, Private: new(true), HiddenFromStatusLists: new(true)},
					},
				},
			},
		},
	}

	extra, recovered := collectMissingCollectionMedia(collection, map[int]struct{}{})
	if !assert.Equal(t, 1, recovered) {
		return
	}

	mc := NewMediaContainer(&MediaContainerOptions{
		AllMedia: NormalizedMediaFromAnilistComplete(extra),
	})

	lfs := []*anime.LocalFile{
		anime.NewLocalFile("E:/Anime/Hidden Adult Show/Hidden Adult Show - 01.mkv", "E:/Anime"),
	}

	matcher := &Matcher{
		LocalFiles:     lfs,
		MediaContainer: mc,
		Logger:         util.NewLogger(),
	}

	if assert.NoError(t, matcher.MatchLocalFilesWithMedia()) {
		assert.Equal(t, 200, lfs[0].MediaId, "the recovered entry should be matchable")
	}
}
