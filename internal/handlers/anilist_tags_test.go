package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"seanime/internal/api/anilist"
	"seanime/internal/core"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTagsContext(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/anilist/collection/raw/tags", nil)
	rec := httptest.NewRecorder()
	return echo.New().NewContext(req, rec), rec
}

// decodeTagMapResponse reads the MediaTagMap out of the SeaResponse envelope.
func decodeTagMapResponse(t *testing.T, rec *httptest.ResponseRecorder) anilist.MediaTagMap {
	t.Helper()
	var body struct {
		Data anilist.MediaTagMap `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

// TestRespondWithCollectionTagsServesAnEmptyMapWhenSignedOut pins the short-circuit: a
// simulated (offline) user has no AniList collection, so neither the collection read nor
// the tags query may run. The response is still a map rather than an error, because the
// filter UI renders it unconditionally.
func TestRespondWithCollectionTagsServesAnEmptyMapWhenSignedOut(t *testing.T) {
	h := &Handler{App: &core.App{}}
	c, rec := newTagsContext(t)

	collectionReads := 0
	fullFetches := 0

	err := h.respondWithCollectionTags(c,
		anilist.AnimeTagCacheKey,
		func() ([]int, error) {
			collectionReads++
			return nil, nil
		},
		func(_ context.Context, _ string) (anilist.MediaTagMap, error) {
			fullFetches++
			return anilist.MediaTagMap{1: {"Action"}}, nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, anilist.MediaTagMap{}, decodeTagMapResponse(t, rec))
	assert.Zero(t, collectionReads, "a signed-out request must not read the collection")
	assert.Zero(t, fullFetches, "a signed-out request must not query AniList")
}

// TestRespondWithCollectionTagsDoesNotCacheTheSignedOutMap guards against the empty map
// being written under a cache key: the key would be built from an empty username, and the
// next signed-in request would be served an empty tag map.
func TestRespondWithCollectionTagsDoesNotCacheTheSignedOutMap(t *testing.T) {
	h := &Handler{App: &core.App{}}
	c, _ := newTagsContext(t)

	require.NoError(t, h.respondWithCollectionTags(c,
		anilist.AnimeTagCacheKey,
		func() ([]int, error) { return nil, nil },
		func(_ context.Context, _ string) (anilist.MediaTagMap, error) { return anilist.MediaTagMap{}, nil },
	))

	_, ok := anilist.GetCollectionTagCache(anilist.AnimeTagCacheKey(""))
	assert.False(t, ok, "the signed-out response must not populate the tag cache")
}

func TestAnimeCollectionMediaIDs(t *testing.T) {
	t.Run("lists every entry, duplicates across custom lists included", func(t *testing.T) {
		// A media on both "Watching" and a custom list appears twice; MissingIDs is what
		// de-duplicates, so this must not drop the repeat and hide an ordering bug there.
		collection := &anilist.AnimeCollection{
			MediaListCollection: &anilist.AnimeCollection_MediaListCollection{
				Lists: []*anilist.AnimeCollection_MediaListCollection_Lists{
					{
						Entries: []*anilist.AnimeCollection_MediaListCollection_Lists_Entries{
							{Media: &anilist.BaseAnime{ID: 21}},
							{Media: &anilist.BaseAnime{ID: 101}},
						},
					},
					{
						Entries: []*anilist.AnimeCollection_MediaListCollection_Lists_Entries{
							{Media: &anilist.BaseAnime{ID: 21}},
						},
					},
				},
			},
		}

		assert.Equal(t, []int{21, 101, 21}, animeCollectionMediaIDs(collection))
	})

	t.Run("skips the nil nodes AniList leaves behind", func(t *testing.T) {
		collection := &anilist.AnimeCollection{
			MediaListCollection: &anilist.AnimeCollection_MediaListCollection{
				Lists: []*anilist.AnimeCollection_MediaListCollection_Lists{
					nil,
					{
						Entries: []*anilist.AnimeCollection_MediaListCollection_Lists_Entries{
							nil,
							{Media: nil},
							{Media: &anilist.BaseAnime{ID: 7}},
						},
					},
				},
			},
		}

		assert.Equal(t, []int{7}, animeCollectionMediaIDs(collection))
	})

	t.Run("returns nothing when the collection is unavailable", func(t *testing.T) {
		assert.Empty(t, animeCollectionMediaIDs(nil))
		assert.Empty(t, animeCollectionMediaIDs(&anilist.AnimeCollection{}))
	})
}

func TestMangaCollectionMediaIDs(t *testing.T) {
	t.Run("lists every entry, duplicates across custom lists included", func(t *testing.T) {
		collection := &anilist.MangaCollection{
			MediaListCollection: &anilist.MangaCollection_MediaListCollection{
				Lists: []*anilist.MangaCollection_MediaListCollection_Lists{
					{
						Entries: []*anilist.MangaCollection_MediaListCollection_Lists_Entries{
							{Media: &anilist.BaseManga{ID: 30002}},
							{Media: &anilist.BaseManga{ID: 53390}},
						},
					},
					{
						Entries: []*anilist.MangaCollection_MediaListCollection_Lists_Entries{
							{Media: &anilist.BaseManga{ID: 30002}},
						},
					},
				},
			},
		}

		assert.Equal(t, []int{30002, 53390, 30002}, mangaCollectionMediaIDs(collection))
	})

	t.Run("skips the nil nodes AniList leaves behind", func(t *testing.T) {
		collection := &anilist.MangaCollection{
			MediaListCollection: &anilist.MangaCollection_MediaListCollection{
				Lists: []*anilist.MangaCollection_MediaListCollection_Lists{
					nil,
					{
						Entries: []*anilist.MangaCollection_MediaListCollection_Lists_Entries{
							nil,
							{Media: nil},
							{Media: &anilist.BaseManga{ID: 9}},
						},
					},
				},
			},
		}

		assert.Equal(t, []int{9}, mangaCollectionMediaIDs(collection))
	})

	t.Run("returns nothing when the collection is unavailable", func(t *testing.T) {
		assert.Empty(t, mangaCollectionMediaIDs(nil))
		assert.Empty(t, mangaCollectionMediaIDs(&anilist.MangaCollection{}))
	})
}
