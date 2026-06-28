package manga

import (
	"cmp"
	"errors"
	"fmt"
	"seanime/internal/extension"
	hibikemanga "seanime/internal/extension/hibike/manga"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"slices"
	"strings"

	"github.com/samber/lo"
)

type GetMangaProviderMatchesOptions struct {
	Provider string
	Titles   []*string
	Year     int
}

// GetMangaProviderMatches searches the selected provider with the entry titles and returns
// a deduplicated, highest-rated list of possible matches for the client to confirm.
func (r *Repository) GetMangaProviderMatches(opts *GetMangaProviderMatchesOptions) (ret []*hibikemanga.SearchResult, err error) {
	defer util.HandlePanicInModuleWithError("manga/GetMangaProviderMatches", &err)

	providerExtension, ok := extension.GetExtension[extension.MangaProviderExtension](r.extensionBankRef.Get(), opts.Provider)
	if !ok {
		r.logger.Error().Str("provider", opts.Provider).Msg("manga: Provider not found")
		return nil, errors.New("manga: Provider not found")
	}

	localProvider, isLocalProvider := providerExtension.GetProvider().(*manga_providers.Local)
	if isLocalProvider && r.settings != nil && r.settings.Manga != nil && r.settings.Manga.LocalSourceDirectory != "" {
		localProvider.SetSourceDirectory(r.settings.Manga.LocalSourceDirectory)
	}

	titles := filterSearchTitles(opts.Titles)
	if len(titles) == 0 {
		return nil, ErrNoTitlesProvided
	}

	searchRes := make([]*hibikemanga.SearchResult, 0)
	var lastErr error

	for _, title := range titles {
		if title == nil {
			continue
		}

		_searchRes, searchErr := providerExtension.GetProvider().Search(hibikemanga.SearchOptions{
			Query: *title,
			Year:  opts.Year,
		})
		if searchErr != nil {
			lastErr = searchErr
			r.logger.Warn().Err(searchErr).Str("query", *title).Msg("manga: Search failed")
			continue
		}

		HydrateSearchResultSearchRating(_searchRes, title)
		searchRes = append(searchRes, _searchRes...)
	}

	if len(searchRes) == 0 {
		r.logger.Error().Str("provider", opts.Provider).Msg("manga: No search results found")
		if lastErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrNoResults, lastErr)
		}
		return nil, ErrNoResults
	}

	return dedupeAndSortProviderMatches(opts.Provider, searchRes), nil
}

func filterSearchTitles(titles []*string) []*string {
	filteredTitles := lo.Filter(titles, func(title *string, _ int) bool {
		return title != nil && strings.TrimSpace(*title) != "" && util.IsMostlyLatinString(*title)
	})

	if len(filteredTitles) > 0 {
		return filteredTitles
	}

	return lo.Filter(titles, func(title *string, _ int) bool {
		return title != nil && strings.TrimSpace(*title) != ""
	})
}

func dedupeAndSortProviderMatches(provider string, searchRes []*hibikemanga.SearchResult) []*hibikemanga.SearchResult {
	bestByID := make(map[string]*hibikemanga.SearchResult, len(searchRes))

	for _, res := range searchRes {
		if res == nil {
			continue
		}

		res.Provider = provider

		key := strings.TrimSpace(res.ID)
		if key == "" {
			key = strings.TrimSpace(res.Title)
		}
		if key == "" {
			continue
		}

		existing, found := bestByID[key]
		if !found || res.SearchRating > existing.SearchRating {
			bestByID[key] = res
			continue
		}

		if res.SearchRating == existing.SearchRating && existing.Image == "" && res.Image != "" {
			bestByID[key] = res
		}
	}

	ret := make([]*hibikemanga.SearchResult, 0, len(bestByID))
	for _, res := range bestByID {
		ret = append(ret, res)
	}

	slices.SortFunc(ret, func(a, b *hibikemanga.SearchResult) int {
		if diff := cmp.Compare(b.SearchRating, a.SearchRating); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)); diff != 0 {
			return diff
		}
		return cmp.Compare(a.ID, b.ID)
	})

	return ret
}

func GetBestSearchResult(searchRes []*hibikemanga.SearchResult) *hibikemanga.SearchResult {
	bestRes := searchRes[0]
	for _, res := range searchRes[1:] {
		if res.SearchRating > bestRes.SearchRating {
			bestRes = res
		}
	}
	return bestRes
}
