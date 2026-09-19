package handlers

import (
	"context"
	"seanime/internal/api/anilist"

	"github.com/labstack/echo/v4"
)

// respondWithCollectionTags serves a MediaTagMap for the signed-in account, keeping the
// cached map across collection refreshes instead of discarding it.
//
// The map used to be rebuilt from a whole-collection AniList query every time the
// collection refreshed — which happens on a cron and after every list-entry edit — even
// though a progress or score change cannot alter any media's tags. Now the cached map is
// reconciled against the already-in-memory collection: if no media has been added since
// it was built, the request costs no AniList traffic at all, and if some have, only those
// ids are fetched.
//
// collectionIDs must read from the cached collection (bypassCache false) so reconciling
// never itself triggers a fetch.
func (h *Handler) respondWithCollectionTags(
	c echo.Context,
	cacheKey func(userName string) string,
	collectionIDs func() ([]int, error),
	fullFetch func(ctx context.Context, userName string) (anilist.MediaTagMap, error),
) error {
	userName := h.App.GetUsername()
	if userName == "" || h.App.GetUser().IsSimulated {
		return h.RespondWithData(c, anilist.MediaTagMap{})
	}

	ctx := c.Request().Context()
	key := cacheKey(userName)

	if cached, ok := anilist.GetCollectionTagCache(key); ok {
		ids, err := collectionIDs()
		if err != nil {
			// The collection is unavailable, so there is nothing to reconcile against.
			// The cached map is still the best answer we have.
			h.App.Logger.Warn().Err(err).Msg("anilist: Could not read collection to reconcile tags")
			return h.RespondWithData(c, cached)
		}

		changed, needsFullRefetch, err := anilist.ReconcileMediaTagMap(ctx, h.App.AnilistPlatformRef.Get().GetAnilistClient(), h.App.Logger, cached, ids)
		switch {
		case err != nil:
			// Serving a slightly incomplete map degrades a filter list; failing the
			// request breaks the page that renders it.
			h.App.Logger.Warn().Err(err).Msg("anilist: Could not fetch tags for newly added media")
			return h.RespondWithData(c, cached)
		case !needsFullRefetch:
			if changed {
				anilist.SetCollectionTagCache(key, cached)
			}
			return h.RespondWithData(c, cached)
		}
		// Too many media are missing for incremental fetching to be worthwhile — one
		// whole-collection query is cheaper. Fall through and rebuild.
	}

	tags, err := fullFetch(ctx, userName)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	anilist.SetCollectionTagCache(key, tags)

	return h.RespondWithData(c, tags)
}
