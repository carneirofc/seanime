package handlers

import (
	"context"
	"errors"
	"fmt"
	"seanime/internal/api/anilist"
	"seanime/internal/platforms/platform"
	"seanime/internal/platforms/shared_platform"
	"seanime/internal/util/limiter"
	"seanime/internal/util/result"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// HandleGetAnimeCollection
//
//	@summary returns the user's AniList anime collection.
//	@desc Calling GET will return the cached anime collection.
//	@desc The manga collection is also refreshed in the background, and upon completion, a WebSocket event is sent.
//	@desc Calling POST will refetch both the anime and manga collections.
//	@returns anilist.AnimeCollection
//	@route /api/v1/anilist/collection [GET,POST]
func (h *Handler) HandleGetAnimeCollection(c echo.Context) error {

	bypassCache := c.Request().Method == "POST"

	if !bypassCache {
		// Get the user's anilist collection
		animeCollection, err := h.App.GetAnimeCollection(false)
		if err != nil {
			return h.RespondWithError(c, err)
		}
		return h.RespondWithData(c, animeCollection)
	}

	animeCollection, err := h.App.RefreshAnimeCollection()
	if err != nil {
		return h.RespondWithError(c, err)
	}

	go func() {
		_, _ = h.App.RefreshMangaCollection()
	}()

	return h.RespondWithData(c, animeCollection)
}

// HandleGetRawAnimeCollection
//
//	@summary returns the user's AniList anime collection without filtering out custom lists.
//	@desc Calling GET will return the cached anime collection.
//	@returns anilist.AnimeCollection
//	@route /api/v1/anilist/collection/raw [GET,POST]
func (h *Handler) HandleGetRawAnimeCollection(c echo.Context) error {

	bypassCache := c.Request().Method == "POST"

	// Get the user's anilist collection
	animeCollection, err := h.App.GetRawAnimeCollection(bypassCache)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, animeCollection)
}

var tagsCache *anilist.MediaTagMap

// HandleGetRawAnimeCollectionTags
//
//	@summary returns the AniList tags for the user's raw anime collection.
//	@desc This runs a dedicated AniList tags query used by the lists page filters.
//	@returns anilist.MediaTagMap
//	@route /api/v1/anilist/collection/raw/tags [GET]
func (h *Handler) HandleGetRawAnimeCollectionTags(c echo.Context) error {
	h.App.OnRefreshAnilistCollectionFuncs.Set("HandleGetRawAnimeCollectionTags", func() {
		tagsCache = nil
	})

	if tagsCache != nil {
		return h.RespondWithData(c, *tagsCache)
	}

	userName := h.App.GetUsername()
	if userName == "" || h.App.GetUser().IsSimulated {
		return h.RespondWithData(c, anilist.MediaTagMap{})
	}

	ret, err := h.App.AnilistPlatformRef.Get().GetAnilistClient().AnimeCollectionTags(c.Request().Context(), &userName)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	tags := anilist.MediaTagMapFromAnimeCollectionTags(ret)
	tagsCache = &tags

	return h.RespondWithData(c, tags)
}

// HandleEditAnilistListEntry
//
//	@summary updates the user's list entry on Anilist.
//	@desc This is used to edit an entry on AniList.
//	@desc The "type" field is used to determine if the entry is an anime or manga and refreshes the collection accordingly.
//	@desc The client should refetch collection-dependent queries after this mutation.
//	@returns true
//	@route /api/v1/anilist/list-entry [POST]
func (h *Handler) HandleEditAnilistListEntry(c echo.Context) error {

	type body struct {
		MediaId               *int                     `json:"mediaId"`
		Status                *anilist.MediaListStatus `json:"status"`
		Score                 *int                     `json:"score"`
		Progress              *int                     `json:"progress"`
		StartDate             *anilist.FuzzyDateInput  `json:"startedAt"`
		EndDate               *anilist.FuzzyDateInput  `json:"completedAt"`
		Private               *bool                    `json:"private"`
		HiddenFromStatusLists *bool                    `json:"hiddenFromStatusLists"`
		Type                  string                   `json:"type"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	// Adult-privacy default (server backstop): when the media is being *added* (no existing entry)
	// and it is adult, default private + hidden from status lists to true unless the caller set them.
	private, hidden := h.resolveAdultPrivacyDefaults(c.Request().Context(), *p.MediaId, p.Type, p.Private, p.HiddenFromStatusLists)

	err := h.App.AnilistPlatformRef.Get().UpdateEntry(
		c.Request().Context(),
		platform.UpdateEntryParams{
			MediaID:               *p.MediaId,
			Status:                p.Status,
			ScoreRaw:              p.Score,
			Progress:              p.Progress,
			StartedAt:             p.StartDate,
			CompletedAt:           p.EndDate,
			Private:               private,
			HiddenFromStatusLists: hidden,
		},
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	switch p.Type {
	case "anime":
		_, _ = h.App.RefreshAnimeCollection()
	case "manga":
		_, _ = h.App.RefreshMangaCollection()
	default:
		_, _ = h.App.RefreshAnimeCollection()
		_, _ = h.App.RefreshMangaCollection()
	}

	return h.RespondWithData(c, true)
}

// resolveAdultPrivacyDefaults implements the adult-privacy default (server backstop).
// It returns the effective (private, hiddenFromStatusLists) to send to AniList.
//
// The default only applies when:
//   - the "make adult entries private" setting is enabled, and
//   - the media is adult (isAdult), and
//   - the entry is being added (no existing entry in the collection — the default is on-add only).
//
// A caller that explicitly provides a flag is never overridden.
func (h *Handler) resolveAdultPrivacyDefaults(ctx context.Context, mediaId int, mediaType string, private, hidden *bool) (*bool, *bool) {
	// Nothing to default if both are already set.
	if private != nil && hidden != nil {
		return private, hidden
	}

	if h.App.Settings == nil || h.App.Settings.GetAnilist() == nil || !h.App.Settings.GetAnilist().MakeAdultEntriesPrivate {
		return private, hidden
	}

	isAdult := false
	entryExists := false

	switch mediaType {
	case "manga":
		if collection, err := h.App.GetMangaCollection(false); err == nil && collection != nil {
			if _, found := collection.GetListEntryFromMangaId(mediaId); found {
				entryExists = true
			}
		}
		if !entryExists {
			if media, err := h.App.AnilistPlatformRef.Get().GetManga(ctx, mediaId); err == nil && media.GetIsAdult() != nil {
				isAdult = *media.GetIsAdult()
			}
		}
	default: // anime
		if collection, err := h.App.GetAnimeCollection(false); err == nil && collection != nil {
			if _, found := collection.GetListEntryFromAnimeId(mediaId); found {
				entryExists = true
			}
		}
		if !entryExists {
			if media, err := h.App.AnilistPlatformRef.Get().GetAnime(ctx, mediaId); err == nil && media.GetIsAdult() != nil {
				isAdult = *media.GetIsAdult()
			}
		}
	}

	// Default only on add of an adult entry.
	if entryExists || !isAdult {
		return private, hidden
	}

	if private == nil {
		private = new(true)
	}
	if hidden == nil {
		hidden = new(true)
	}
	return private, hidden
}

// HandlePrivatizeAdultEntries
//
//	@summary marks every adult (isAdult) list entry that is currently public as private + hidden from status lists.
//	@desc This is the bulk action behind the "adult titles are publicly visible" collection alert.
//	@desc The "type" field ("anime" | "manga" | "") restricts the action; empty means both.
//	@returns int - the number of entries updated
//	@route /api/v1/anilist/privatize-adult-entries [POST]
func (h *Handler) HandlePrivatizeAdultEntries(c echo.Context) error {
	type body struct {
		Type string `json:"type"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	ctx := c.Request().Context()
	platformRef := h.App.AnilistPlatformRef.Get()

	// Collect the media IDs of adult entries that are currently public (private != true).
	mediaIds := make([]int, 0)

	if p.Type == "" || p.Type == "anime" {
		if collection, err := h.App.GetAnimeCollection(false); err == nil && collection != nil && collection.MediaListCollection != nil {
			for _, list := range collection.MediaListCollection.Lists {
				if list == nil {
					continue
				}
				for _, entry := range list.GetEntries() {
					media := entry.GetMedia()
					if media == nil || media.GetIsAdult() == nil || !*media.GetIsAdult() {
						continue
					}
					if entry.GetPrivate() != nil && *entry.GetPrivate() {
						continue
					}
					mediaIds = append(mediaIds, media.GetID())
				}
			}
		}
	}

	if p.Type == "" || p.Type == "manga" {
		if collection, err := h.App.GetMangaCollection(false); err == nil && collection != nil && collection.MediaListCollection != nil {
			for _, list := range collection.MediaListCollection.Lists {
				if list == nil {
					continue
				}
				for _, entry := range list.Entries {
					media := entry.GetMedia()
					if media == nil || media.GetIsAdult() == nil || !*media.GetIsAdult() {
						continue
					}
					if entry.GetPrivate() != nil && *entry.GetPrivate() {
						continue
					}
					mediaIds = append(mediaIds, media.GetID())
				}
			}
		}
	}

	updated := 0
	rateLimiter := limiter.NewLimiter(1*time.Second, 1)
	for _, mediaId := range mediaIds {
		rateLimiter.Wait()
		err := platformRef.UpdateEntry(ctx, platform.UpdateEntryParams{
			MediaID:               mediaId,
			Private:               new(true),
			HiddenFromStatusLists: new(true),
		})
		if err != nil {
			h.App.Logger.Error().Err(err).Int("mediaId", mediaId).Msg("anilist: failed to privatize adult entry")
			continue
		}
		updated++
	}

	if p.Type == "" || p.Type == "anime" {
		_, _ = h.App.RefreshAnimeCollection()
	}
	if p.Type == "" || p.Type == "manga" {
		_, _ = h.App.RefreshMangaCollection()
	}

	return h.RespondWithData(c, updated)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

var (
	detailsCache = result.NewCache[int, *anilist.AnimeDetailsById_Media]()
)

// HandleGetAnilistAnimeDetails
//
//	@summary returns more details about an AniList anime entry.
//	@desc This fetches more fields omitted from the base queries.
//	@param id - int - true - "The AniList anime ID"
//	@returns anilist.AnimeDetailsById_Media
//	@route /api/v1/anilist/media-details/{id} [GET]
func (h *Handler) HandleGetAnilistAnimeDetails(c echo.Context) error {

	mId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if details, ok := detailsCache.Get(mId); ok {
		return h.RespondWithData(c, details)
	}
	details, err := h.App.AnilistPlatformRef.Get().GetAnimeDetails(c.Request().Context(), mId)
	if err != nil {
		return h.RespondWithError(c, err)
	}
	detailsCache.Set(mId, details)

	return h.RespondWithData(c, details)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

var studioDetailsMap = result.NewMap[int, *anilist.StudioDetails]()

// HandleGetAnilistStudioDetails
//
//	@summary returns details about a studio.
//	@desc This fetches media produced by the studio.
//	@param id - int - true - "The AniList studio ID"
//	@returns anilist.StudioDetails
//	@route /api/v1/anilist/studio-details/{id} [GET]
func (h *Handler) HandleGetAnilistStudioDetails(c echo.Context) error {

	mId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if details, ok := studioDetailsMap.Get(mId); ok {
		return h.RespondWithData(c, details)
	}
	details, err := h.App.AnilistPlatformRef.Get().GetStudioDetails(c.Request().Context(), mId)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	go func() {
		if details != nil {
			studioDetailsMap.Set(mId, details)
		}
	}()

	return h.RespondWithData(c, details)
}

//----------------------------------------------------------------------------------------------------------------------------------------------------

// HandleDeleteAnilistListEntry
//
//	@summary deletes an entry from the user's AniList list.
//	@desc This is used to delete an entry on AniList.
//	@desc The "type" field is used to determine if the entry is an anime or manga and refreshes the collection accordingly.
//	@desc The client should refetch collection-dependent queries after this mutation.
//	@route /api/v1/anilist/list-entry [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteAnilistListEntry(c echo.Context) error {

	type body struct {
		MediaId *int    `json:"mediaId"`
		Type    *string `json:"type"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Type == nil || p.MediaId == nil {
		return h.RespondWithError(c, errors.New("missing parameters"))
	}

	var listEntryID int

	switch *p.Type {
	case "anime":
		// Get the list entry ID
		animeCollection, err := h.App.GetAnimeCollection(false)
		if err != nil {
			return h.RespondWithError(c, err)
		}

		listEntry, found := animeCollection.GetListEntryFromAnimeId(*p.MediaId)
		if !found {
			return h.RespondWithError(c, errors.New("list entry not found"))
		}
		listEntryID = listEntry.ID
	case "manga":
		// Get the list entry ID
		mangaCollection, err := h.App.GetMangaCollection(false)
		if err != nil {
			return h.RespondWithError(c, err)
		}

		listEntry, found := mangaCollection.GetListEntryFromMangaId(*p.MediaId)
		if !found {
			return h.RespondWithError(c, errors.New("list entry not found"))
		}
		listEntryID = listEntry.ID
	}

	// Delete the list entry
	err := h.App.AnilistPlatformRef.Get().DeleteEntry(c.Request().Context(), *p.MediaId, listEntryID)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	switch *p.Type {
	case "anime":
		_, _ = h.App.RefreshAnimeCollection()
	case "manga":
		_, _ = h.App.RefreshMangaCollection()
	}

	return h.RespondWithData(c, true)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	anilistListAnimeCache       = result.NewCache[string, *anilist.ListAnime]()
	anilistListRecentAnimeCache = result.NewCache[string, *anilist.ListRecentAnime]() // holds 1 value
)

// HandleAnilistListAnime
//
//	@summary returns a list of anime based on the search parameters.
//	@desc This is used by the "Discover" and "Advanced Search".
//	@route /api/v1/anilist/list-anime [POST]
//	@returns anilist.ListAnime
func (h *Handler) HandleAnilistListAnime(c echo.Context) error {

	type body struct {
		Page                *int                   `json:"page,omitempty"`
		Search              *string                `json:"search,omitempty"`
		PerPage             *int                   `json:"perPage,omitempty"`
		Sort                []*anilist.MediaSort   `json:"sort,omitempty"`
		Status              []*anilist.MediaStatus `json:"status,omitempty"`
		Genres              []*string              `json:"genres,omitempty"`
		Tags                []*string              `json:"tags,omitempty"`
		AverageScoreGreater *int                   `json:"averageScore_greater,omitempty"`
		Season              *anilist.MediaSeason   `json:"season,omitempty"`
		SeasonYear          *int                   `json:"seasonYear,omitempty"`
		Format              *anilist.MediaFormat   `json:"format,omitempty"`
		IsAdult             *bool                  `json:"isAdult,omitempty"`
		CountryOfOrigin     *string                `json:"countryOfOrigin,omitempty"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Page == nil || p.PerPage == nil {
		*p.Page = 1
		*p.PerPage = 20
	}

	adultContentEnabled := h.App.Settings.GetAnilist().EnableAdultContent
	var isAdult *bool = nil
	if p.IsAdult != nil {
		v := *p.IsAdult && adultContentEnabled
		isAdult = &v
	}

	cacheKey := anilist.ListAnimeCacheKey(
		p.Page,
		p.Search,
		p.PerPage,
		p.Sort,
		p.Status,
		p.Genres,
		p.Tags,
		p.AverageScoreGreater,
		p.Season,
		p.SeasonYear,
		p.Format,
		isAdult,
		p.CountryOfOrigin,
	)

	cached, ok := anilistListAnimeCache.Get(cacheKey)
	if ok {
		return h.RespondWithData(c, cached)
	}

	ret, err := anilist.ListAnimeM(
		h.App.AnilistPlatformRef.Get().GetAnilistClient(),
		p.Page,
		p.Search,
		p.PerPage,
		p.Sort,
		p.Status,
		p.Genres,
		p.Tags,
		p.AverageScoreGreater,
		p.Season,
		p.SeasonYear,
		p.Format,
		isAdult,
		p.CountryOfOrigin,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	if ret != nil {
		anilistListAnimeCache.SetT(cacheKey, ret, time.Minute*10)
	}

	return h.RespondWithData(c, ret)
}

// HandleAnilistListRecentAiringAnime
//
//	@summary returns a list of recently aired anime.
//	@desc This is used by the "Schedule" page to display recently aired anime.
//	@route /api/v1/anilist/list-recent-anime [POST]
//	@returns anilist.ListRecentAnime
func (h *Handler) HandleAnilistListRecentAiringAnime(c echo.Context) error {

	type body struct {
		Page            *int                  `json:"page,omitempty"`
		Search          *string               `json:"search,omitempty"`
		PerPage         *int                  `json:"perPage,omitempty"`
		AiringAtGreater *int                  `json:"airingAt_greater,omitempty"`
		AiringAtLesser  *int                  `json:"airingAt_lesser,omitempty"`
		NotYetAired     *bool                 `json:"notYetAired,omitempty"`
		Sort            []*anilist.AiringSort `json:"sort,omitempty"`
	}

	p := new(body)
	if err := c.Bind(p); err != nil {
		return h.RespondWithError(c, err)
	}

	if p.Page == nil || p.PerPage == nil {
		*p.Page = 1
		*p.PerPage = 50
	}

	cacheKey := fmt.Sprintf("%v-%v-%v-%v-%v-%v-%v", p.Page, p.Search, p.PerPage, p.AiringAtGreater, p.AiringAtLesser, p.NotYetAired, p.Sort)

	cached, ok := anilistListRecentAnimeCache.Get(cacheKey)
	if ok {
		return h.RespondWithData(c, cached)
	}

	ret, err := anilist.ListRecentAiringAnimeM(
		h.App.AnilistPlatformRef.Get().GetAnilistClient(),
		p.Page,
		p.Search,
		p.PerPage,
		p.AiringAtGreater,
		p.AiringAtLesser,
		p.NotYetAired,
		p.Sort,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilistListRecentAnimeCache.SetT(cacheKey, ret, time.Hour*1)

	return h.RespondWithData(c, ret)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var anilistMissedSequelsCache = result.NewCache[int, []*anilist.BaseAnime]()

// HandleAnilistListMissedSequels
//
//	@summary returns a list of sequels not in the user's list.
//	@desc This is used by the "Discover" page to display sequels the user may have missed.
//	@route /api/v1/anilist/list-missed-sequels [GET]
//	@returns []anilist.BaseAnime
func (h *Handler) HandleAnilistListMissedSequels(c echo.Context) error {

	cached, ok := anilistMissedSequelsCache.Get(1)
	if ok {
		return h.RespondWithData(c, cached)
	}

	// Get complete anime collection
	animeCollection, err := h.App.AnilistPlatformRef.Get().GetAnimeCollectionWithRelations(c.Request().Context())
	if err != nil {
		return h.RespondWithError(c, err)
	}

	ret, err := anilist.ListMissedSequels(
		h.App.AnilistPlatformRef.Get().GetAnilistClient(),
		animeCollection,
		h.App.Logger,
		h.App.GetUserAnilistToken(),
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilistMissedSequelsCache.SetT(1, ret, time.Hour*4)

	return h.RespondWithData(c, ret)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var anilistStatsCache = result.NewCache[int, *anilist.Stats]()

// HandleGetAniListStats
//
//	@summary returns the anilist stats.
//	@desc This returns the AniList stats for the user.
//	@route /api/v1/anilist/stats [GET]
//	@returns anilist.Stats
func (h *Handler) HandleGetAniListStats(c echo.Context) error {
	cached, ok := anilistStatsCache.Get(0)
	if ok {
		return h.RespondWithData(c, cached)
	}

	stats, err := h.App.AnilistPlatformRef.Get().GetViewerStats(c.Request().Context())
	if err != nil {
		return h.RespondWithError(c, err)
	}

	ret, err := anilist.GetStats(
		c.Request().Context(),
		stats,
	)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilistStatsCache.SetT(0, ret, time.Hour*1)

	return h.RespondWithData(c, ret)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// HandleGetAnilistCacheLayerStatus
//
//	@summary returns the status of the AniList cache layer.
//	@desc This returns the status of the AniList cache layer.
//	@route /api/v1/anilist/cache-layer/status [GET]
//	@returns bool
func (h *Handler) HandleGetAnilistCacheLayerStatus(c echo.Context) error {
	return h.RespondWithData(c, shared_platform.IsWorking.Load())
}

// HandleToggleAnilistCacheLayerStatus
//
//	@summary toggles the status of the AniList cache layer.
//	@desc This toggles the status of the AniList cache layer.
//	@route /api/v1/anilist/cache-layer/status [POST]
//	@returns bool
func (h *Handler) HandleToggleAnilistCacheLayerStatus(c echo.Context) error {
	shared_platform.IsWorking.Store(!shared_platform.IsWorking.Load())
	return h.RespondWithData(c, shared_platform.IsWorking.Load())
}
