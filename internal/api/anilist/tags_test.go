package anilist

import (
	"context"
	"errors"
	"seanime/internal/util"
	"testing"

	"github.com/gqlgo/gqlgenc/clientv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaTagMapAddDeduplicatesAndSkipsEmptyNames(t *testing.T) {
	m := make(MediaTagMap)

	m.add(1, "Action")
	m.add(1, "Action")
	m.add(1, "")
	m.add(1, "Drama")
	m.add(2, "Comedy")

	assert.Equal(t, []string{"Action", "Drama"}, m[1])
	assert.Equal(t, []string{"Comedy"}, m[2])
}

func TestMediaTagMapMissingIDs(t *testing.T) {
	tests := []struct {
		name          string
		existing      MediaTagMap
		collectionIDs []int
		want          []int
	}{
		{
			name:          "everything already known",
			existing:      MediaTagMap{1: {"Action"}, 2: {"Drama"}},
			collectionIDs: []int{1, 2},
			want:          []int{},
		},
		{
			name:          "reports only the new ids, sorted",
			existing:      MediaTagMap{5: {"Action"}},
			collectionIDs: []int{9, 5, 3},
			want:          []int{3, 9},
		},
		{
			name:          "de-duplicates ids repeated across custom lists",
			existing:      MediaTagMap{},
			collectionIDs: []int{7, 7, 7},
			want:          []int{7},
		},
		{
			name: "a media known to have no tags is not missing",
			// This is the case that would otherwise be refetched forever: the media
			// exists, AniList returned no tags for it, and the empty slice records that.
			existing:      MediaTagMap{4: {}},
			collectionIDs: []int{4},
			want:          []int{},
		},
		{
			name:          "ignores non-positive ids",
			existing:      MediaTagMap{},
			collectionIDs: []int{0, -1, 8},
			want:          []int{8},
		},
		{
			name:          "empty collection",
			existing:      MediaTagMap{1: {"Action"}},
			collectionIDs: nil,
			want:          []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.existing.MissingIDs(tt.collectionIDs))
		})
	}
}

func TestChunkMediaIDsSplitsAtAniListPageCap(t *testing.T) {
	assert.Nil(t, ChunkMediaIDs(nil))
	assert.Nil(t, ChunkMediaIDs([]int{}))

	ids := make([]int, MediaTagsPerPage+1)
	for i := range ids {
		ids[i] = i + 1
	}

	chunks := ChunkMediaIDs(ids)
	require.Len(t, chunks, 2)
	assert.Len(t, chunks[0], MediaTagsPerPage)
	assert.Len(t, chunks[1], 1)

	exact := ChunkMediaIDs(ids[:MediaTagsPerPage])
	require.Len(t, exact, 1)
	assert.Len(t, exact[0], MediaTagsPerPage)
}

// tagFetchStub records the GetMediaTagsByID calls a reconcile makes and answers them
// from a canned id -> tag names table.
type tagFetchStub struct {
	AnilistClient
	calls   [][]int
	tags    map[int][]string
	err     error
	perPage []int
}

func (s *tagFetchStub) GetMediaTagsByID(_ context.Context, ids []int, _ *int, perPage *int, _ ...clientv2.RequestInterceptor) (*GetMediaTagsByID, error) {
	s.calls = append(s.calls, append([]int(nil), ids...))
	if perPage != nil {
		s.perPage = append(s.perPage, *perPage)
	}
	if s.err != nil {
		return nil, s.err
	}

	media := make([]*GetMediaTagsById_Page_Media, 0, len(ids))
	for _, id := range ids {
		names, ok := s.tags[id]
		if !ok {
			continue
		}
		tags := make([]*GetMediaTagsById_Page_Media_Tags, 0, len(names))
		for _, n := range names {
			tags = append(tags, &GetMediaTagsById_Page_Media_Tags{Name: n})
		}
		media = append(media, &GetMediaTagsById_Page_Media{ID: id, Tags: tags})
	}

	return &GetMediaTagsByID{Page: &GetMediaTagsById_Page{Media: media}}, nil
}

func TestReconcileMediaTagMapMakesNoRequestWhenNothingIsMissing(t *testing.T) {
	stub := &tagFetchStub{}
	m := MediaTagMap{1: {"Action"}, 2: {"Drama"}}

	changed, needsFull, err := ReconcileMediaTagMap(context.Background(), stub, util.NewLogger(), m, []int{1, 2})

	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, needsFull)
	// This is the whole point of the change: a progress update must cost no AniList traffic.
	assert.Empty(t, stub.calls, "reconcile must not contact AniList when every id is known")
}

func TestReconcileMediaTagMapFetchesOnlyTheNewIDs(t *testing.T) {
	stub := &tagFetchStub{tags: map[int][]string{7: {"Isekai", "Comedy"}}}
	m := MediaTagMap{1: {"Action"}}

	changed, needsFull, err := ReconcileMediaTagMap(context.Background(), stub, util.NewLogger(), m, []int{1, 7})

	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, needsFull)
	require.Len(t, stub.calls, 1)
	assert.Equal(t, []int{7}, stub.calls[0], "only the missing id should be requested")
	assert.Equal(t, []int{MediaTagsPerPage}, stub.perPage, "perPage must be explicit, never AniList's default")
	assert.Equal(t, []string{"Isekai", "Comedy"}, m[7])
	assert.Equal(t, []string{"Action"}, m[1])
}

func TestReconcileMediaTagMapRecordsMediaWithNoTags(t *testing.T) {
	// AniList returned the media but with no tags. It must still be recorded, or it
	// counts as missing on every subsequent reconcile and is refetched forever.
	stub := &tagFetchStub{tags: map[int][]string{3: {}}}
	m := MediaTagMap{}

	_, _, err := ReconcileMediaTagMap(context.Background(), stub, util.NewLogger(), m, []int{3})

	require.NoError(t, err)
	_, ok := m[3]
	assert.True(t, ok, "a media with no tags must still be recorded")
	assert.Empty(t, m.MissingIDs([]int{3}))
}

func TestReconcileMediaTagMapFallsBackToFullRefetchOnLargeDeltas(t *testing.T) {
	// Past the cap, chunked fetching costs more requests than the single
	// whole-collection query it is meant to avoid.
	ids := make([]int, MediaTagsPerPage*maxIncrementalTagFetches+1)
	for i := range ids {
		ids[i] = i + 1
	}

	stub := &tagFetchStub{}
	changed, needsFull, err := ReconcileMediaTagMap(context.Background(), stub, util.NewLogger(), MediaTagMap{}, ids)

	require.NoError(t, err)
	assert.False(t, changed)
	assert.True(t, needsFull)
	assert.Empty(t, stub.calls, "it must not start fetching before falling back")
}

func TestReconcileMediaTagMapPropagatesFetchErrors(t *testing.T) {
	stub := &tagFetchStub{err: errors.New("rate limited")}

	_, needsFull, err := ReconcileMediaTagMap(context.Background(), stub, util.NewLogger(), MediaTagMap{}, []int{1})

	require.Error(t, err)
	assert.False(t, needsFull)
}

func TestCollectionTagCacheIsKeyedPerAccountAndMediaType(t *testing.T) {
	// The keys must not collide: the same server serves one account at a time, but the
	// map must not survive an account switch, and anime and manga maps are distinct.
	assert.NotEqual(t, AnimeTagCacheKey("alice"), AnimeTagCacheKey("bob"))
	assert.NotEqual(t, AnimeTagCacheKey("alice"), MangaTagCacheKey("alice"))

	key := AnimeTagCacheKey("alice")
	t.Cleanup(func() { ClearCollectionTagCache(key) })

	_, ok := GetCollectionTagCache(key)
	require.False(t, ok)

	SetCollectionTagCache(key, MediaTagMap{1: {"Action"}})
	got, ok := GetCollectionTagCache(key)
	require.True(t, ok)
	assert.Equal(t, []string{"Action"}, got[1])

	_, ok = GetCollectionTagCache(AnimeTagCacheKey("bob"))
	assert.False(t, ok, "another account must not see this map")

	ClearCollectionTagCache(key)
	_, ok = GetCollectionTagCache(key)
	assert.False(t, ok)
}

func animeTagsCollection(lists ...*AnimeCollectionTags_MediaListCollection_Lists) *AnimeCollectionTags {
	return &AnimeCollectionTags{
		MediaListCollection: &AnimeCollectionTags_MediaListCollection{Lists: lists},
	}
}

func animeTagsList(entries ...*AnimeCollectionTags_MediaListCollection_Lists_Entries) *AnimeCollectionTags_MediaListCollection_Lists {
	return &AnimeCollectionTags_MediaListCollection_Lists{Entries: entries}
}

func animeTagsEntry(id int, tagNames ...string) *AnimeCollectionTags_MediaListCollection_Lists_Entries {
	tags := make([]*AnimeCollectionTags_MediaListCollection_Lists_Entries_Media_Tags, 0, len(tagNames))
	for _, name := range tagNames {
		tags = append(tags, &AnimeCollectionTags_MediaListCollection_Lists_Entries_Media_Tags{Name: name})
	}
	return &AnimeCollectionTags_MediaListCollection_Lists_Entries{
		Media: &AnimeCollectionTags_MediaListCollection_Lists_Entries_Media{ID: id, Tags: tags},
	}
}

func TestMediaTagMapFromAnimeCollectionTags(t *testing.T) {
	// A media on two custom lists arrives twice, so the builder must merge rather than
	// duplicate; nils appear whenever AniList omits an optional node.
	data := animeTagsCollection(
		animeTagsList(
			animeTagsEntry(1, "Action", "Isekai"),
			animeTagsEntry(2, "Drama"),
			nil,
			&AnimeCollectionTags_MediaListCollection_Lists_Entries{Media: nil},
		),
		nil,
		animeTagsList(
			animeTagsEntry(1, "Isekai", "Comedy"),
			animeTagsEntry(3),
		),
	)

	got := MediaTagMapFromAnimeCollectionTags(data)

	assert.Equal(t, []string{"Action", "Isekai", "Comedy"}, got[1])
	assert.Equal(t, []string{"Drama"}, got[2])
	// A media whose tag list is empty produces no key at all here; the reconcile path is
	// what records those, so they are not refetched forever.
	_, ok := got[3]
	assert.False(t, ok)
}

func TestMediaTagMapFromAnimeCollectionTagsHandlesMissingData(t *testing.T) {
	assert.Equal(t, MediaTagMap{}, MediaTagMapFromAnimeCollectionTags(nil))
	assert.Equal(t, MediaTagMap{}, MediaTagMapFromAnimeCollectionTags(&AnimeCollectionTags{}))
}

func TestMediaTagMapFromMangaCollectionTags(t *testing.T) {
	data := &MangaCollectionTags{
		MediaListCollection: &MangaCollectionTags_MediaListCollection{
			Lists: []*MangaCollectionTags_MediaListCollection_Lists{
				nil,
				{
					Entries: []*MangaCollectionTags_MediaListCollection_Lists_Entries{
						nil,
						{
							Media: &MangaCollectionTags_MediaListCollection_Lists_Entries_Media{
								ID: 42,
								Tags: []*MangaCollectionTags_MediaListCollection_Lists_Entries_Media_Tags{
									{Name: "Seinen"},
									nil,
									{Name: "Seinen"},
									{Name: ""},
									{Name: "Psychological"},
								},
							},
						},
					},
				},
			},
		},
	}

	got := MediaTagMapFromMangaCollectionTags(data)

	assert.Equal(t, []string{"Seinen", "Psychological"}, got[42])
	assert.Len(t, got, 1)
}

func TestMediaTagMapFromMangaCollectionTagsHandlesMissingData(t *testing.T) {
	assert.Equal(t, MediaTagMap{}, MediaTagMapFromMangaCollectionTags(nil))
	assert.Equal(t, MediaTagMap{}, MediaTagMapFromMangaCollectionTags(&MangaCollectionTags{}))
}
