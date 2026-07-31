package manga

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"os"
	"seanime/internal/api/anilist"
	"seanime/internal/extension"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"seanime/internal/hook"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"seanime/internal/util/comparison"
	"seanime/internal/util/limiter"
	"seanime/internal/util/result"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
)

type (
	// ChapterContainer is used to display the list of chapters from a provider in the client.
	// It is cached in a unique file cache bucket with a key of the format: {provider}${mediaId}
	ChapterContainer struct {
		MediaId  int                           `json:"mediaId"`
		Provider string                        `json:"provider"`
		Chapters []*hibikemanga.ChapterDetails `json:"chapters"`
	}
)

func getMangaChapterContainerCacheKey(provider string, mediaId int) string {
	return fmt.Sprintf("%s$%d", provider, mediaId)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type GetMangaChapterContainerOptions struct {
	Provider                    string
	MediaId                     int
	Titles                      []*string
	Year                        int
	IncludeProviderAvailability bool
	skipCache                   bool
	beforeProviderCall          func() error
	// allowAutoSearch lets the caller fall back to searching the provider when no manga mapping
	// exists for the entry. Callers driven by the UI leave this false so the user is asked to pick
	// a source explicitly (see ErrMangaMatchRequired); internal refresh flows opt in.
	allowAutoSearch bool
}

// GetMangaChapterContainer returns the ChapterContainer for a manga entry based on the provider.
// If it isn't cached, it will search for the manga, create a ChapterContainer and cache it.
func (r *Repository) GetMangaChapterContainer(opts *GetMangaChapterContainerOptions) (ret *ChapterContainer, err error) {
	defer util.HandlePanicInModuleWithError("manga/GetMangaChapterContainer", &err)

	provider := opts.Provider
	mediaId := opts.MediaId
	titles := opts.Titles

	providerExtension, ok := extension.GetExtension[extension.MangaProviderExtension](r.extensionBankRef.Get(), provider)
	if !ok {
		r.logger.Error().Str("provider", provider).Msg("manga: Provider not found")
		return nil, errors.New("manga: Provider not found")
	}

	// DEVNOTE: Local chapters can be cached
	localProvider, isLocalProvider := providerExtension.GetProvider().(*manga_providers.Local)

	// Set the source directory for local provider
	if isLocalProvider && r.settings.Manga.LocalSourceDirectory != "" {
		localProvider.SetSourceDirectory(r.settings.Manga.LocalSourceDirectory)
	}

	r.logger.Trace().
		Str("provider", provider).
		Int("mediaId", mediaId).
		Msgf("manga: Getting chapters")

	chapterContainerKey := getMangaChapterContainerCacheKey(provider, mediaId)

	// +---------------------+
	// |     Hook event      |
	// +---------------------+

	// Trigger hook event
	reqEvent := &MangaChapterContainerRequestedEvent{
		Provider: provider,
		MediaId:  mediaId,
		Titles:   titles,
		Year:     opts.Year,
		ChapterContainer: &ChapterContainer{
			MediaId:  mediaId,
			Provider: provider,
			Chapters: []*hibikemanga.ChapterDetails{},
		},
	}
	err = hook.GlobalHookManager.OnMangaChapterContainerRequested().Trigger(reqEvent)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
		return nil, fmt.Errorf("manga: Error in hook, %w", err)
	}

	// Default prevented, return the chapter container
	if reqEvent.DefaultPrevented {
		if reqEvent.ChapterContainer == nil {
			return nil, fmt.Errorf("manga: No chapter container returned by hook event")
		}
		return reqEvent.ChapterContainer, nil
	}

	// +---------------------+
	// |       Cache         |
	// +---------------------+

	var container *ChapterContainer
	containerBucket := r.getFcProviderBucket(provider, mediaId, bucketTypeChapter)

	// Check if the container is in the cache
	foundInCache := false
	if !opts.skipCache {
		foundInCache, _ = r.fileCacher.Get(containerBucket, chapterContainerKey, &container)
	}
	if foundInCache {
		r.logger.Info().Str("bucket", containerBucket.Name()).Msg("manga: Chapter Container Cache HIT")

		// Trigger hook event
		ev := &MangaChapterContainerEvent{
			ChapterContainer: container,
		}
		err = hook.GlobalHookManager.OnMangaChapterContainer().Trigger(ev)
		if err != nil {
			r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
		}
		container = ev.ChapterContainer

		if opts.IncludeProviderAvailability {
			r.attachAlternativeProviders(container, opts)
		}

		return container, nil
	}

	// Delete the map cache
	mangaLatestChapterNumberMap.Delete(ChapterCountMapCacheKey)

	var mangaId string

	// +---------------------+
	// |      Database       |
	// +---------------------+

	// Search for the mapping in the database
	mapping, found := r.db.GetMangaMapping(provider, mediaId)
	if found {
		r.logger.Debug().Str("mangaId", mapping.MangaID).Msg("manga: Using manual mapping")
		mangaId = mapping.MangaID
	}

	if mangaId == "" {
		if !opts.allowAutoSearch {
			r.logger.Debug().
				Str("provider", provider).
				Int("mediaId", mediaId).
				Msg("manga: Match selection required before loading chapters")
			return nil, ErrMangaMatchRequired
		}

		// +---------------------+
		// |       Search        |
		// +---------------------+

		r.logger.Trace().Msg("manga: Searching for manga")

		if titles == nil {
			return nil, ErrNoTitlesProvided
		}

		titles = lo.Filter(titles, func(title *string, _ int) bool {
			return util.IsMostlyLatinString(*title)
		})

		var searchRes []*hibikemanga.SearchResult

		var err error
		for _, title := range titles {
			var _searchRes []*hibikemanga.SearchResult

			if opts.beforeProviderCall != nil {
				if err := opts.beforeProviderCall(); err != nil {
					return nil, err
				}
			}
			_searchRes, err = providerExtension.GetProvider().Search(hibikemanga.SearchOptions{
				Query: *title,
				Year:  opts.Year,
			})
			if err == nil {

				HydrateSearchResultSearchRating(_searchRes, title)

				searchRes = append(searchRes, _searchRes...)
			} else {
				r.logger.Warn().Err(err).Msg("manga: Search failed")
			}
		}

		if len(searchRes) == 0 {
			r.logger.Error().Msg("manga: No search results found")
			if err != nil {
				return nil, fmt.Errorf("%w, %w", ErrNoResults, err)
			} else {
				return nil, ErrNoResults
			}
		}

		// Overwrite the provider just in case
		for _, res := range searchRes {
			res.Provider = provider
		}

		bestRes := GetBestSearchResult(searchRes)

		mangaId = bestRes.ID
	}

	// +---------------------+
	// |    Get chapters     |
	// +---------------------+

	if opts.beforeProviderCall != nil {
		if err := opts.beforeProviderCall(); err != nil {
			return nil, err
		}
	}
	chapterList, err := providerExtension.GetProvider().FindChapters(mangaId)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Failed to get chapters")
		// Always wrap the provider error: errors.Is(err, ErrNoChapters) still matches, and the
		// underlying cause is needed to tell "provider is down" apart from "no chapters exist".
		return nil, fmt.Errorf("%w: %w", ErrNoChapters, err)
	}

	hibikemanga.NormalizeChapterProviders(chapterList, provider)

	container = &ChapterContainer{
		MediaId:  mediaId,
		Provider: provider,
		Chapters: chapterList,
	}

	// Trigger hook event
	ev := &MangaChapterContainerEvent{
		ChapterContainer: container,
	}
	err = hook.GlobalHookManager.OnMangaChapterContainer().Trigger(ev)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
	}
	container = ev.ChapterContainer

	// Cache the container only if it has chapters
	if len(container.Chapters) > 0 {
		err = r.fileCacher.Set(containerBucket, chapterContainerKey, container)
		if err != nil {
			r.logger.Warn().Err(err).Msg("manga: Failed to populate cache")
		}
	}

	if opts.IncludeProviderAvailability {
		r.attachAlternativeProviders(container, opts)
	}

	r.logger.Info().Str("bucket", containerBucket.Name()).Msg("manga: Retrieved chapters")
	return container, nil
}

func (r *Repository) attachAlternativeProviders(container *ChapterContainer, opts *GetMangaChapterContainerOptions) {
	if container == nil || len(container.Chapters) == 0 {
		return
	}

	alternativeContainers := r.getAlternativeChapterContainers(opts)
	if len(alternativeContainers) == 0 {
		for _, chapter := range container.Chapters {
			if chapter != nil {
				chapter.AlternativeProviders = nil
			}
		}
		return
	}

	attachAlternativeProvidersToContainer(container, alternativeContainers)
}

func (r *Repository) getAlternativeChapterContainers(opts *GetMangaChapterContainerOptions) []*ChapterContainer {
	providerIDs := make([]string, 0)

	extension.RangeExtensions[extension.MangaProviderExtension](r.extensionBankRef.Get(), func(id string, _ extension.MangaProviderExtension) bool {
		if id == opts.Provider {
			return true
		}

		if _, found := r.db.GetMangaMapping(id, opts.MediaId); !found {
			return true
		}

		providerIDs = append(providerIDs, id)
		return true
	})

	if len(providerIDs) == 0 {
		return nil
	}

	resultsCh := make(chan *ChapterContainer, len(providerIDs))
	wg := sync.WaitGroup{}

	for _, providerID := range providerIDs {
		wg.Add(1)
		go func(providerID string) {
			defer wg.Done()

			container, err := r.GetMangaChapterContainer(&GetMangaChapterContainerOptions{
				Provider: providerID,
				MediaId:  opts.MediaId,
				Titles:   opts.Titles,
				Year:     opts.Year,
			})
			if err != nil {
				r.logger.Debug().
					Err(err).
					Str("provider", providerID).
					Int("mediaId", opts.MediaId).
					Msg("manga: Skipping alternative provider availability")
				return
			}

			if container == nil || len(container.Chapters) == 0 {
				return
			}

			resultsCh <- container
		}(providerID)
	}

	wg.Wait()
	close(resultsCh)

	ret := make([]*ChapterContainer, 0, len(providerIDs))
	for container := range resultsCh {
		ret = append(ret, container)
	}

	return ret
}

func attachAlternativeProvidersToContainer(primary *ChapterContainer, alternatives []*ChapterContainer) {
	if primary == nil {
		return
	}

	exactMatches := make(map[string][]hibikemanga.ChapterProviderOption)
	chapterMatches := make(map[string][]hibikemanga.ChapterProviderOption)

	for _, alternative := range alternatives {
		if alternative == nil {
			continue
		}

		for _, chapter := range alternative.Chapters {
			exactKey, chapterKey := getAlternativeProviderMatchKeys(chapter)
			if chapterKey == "" {
				continue
			}

			option := hibikemanga.ChapterProviderOption{
				Provider:  chapter.Provider,
				ChapterID: chapter.ID,
				Scanlator: chapter.Scanlator,
				Language:  chapter.Language,
			}

			if exactKey != "" {
				exactMatches[exactKey] = appendUniqueChapterProviderOption(exactMatches[exactKey], option)
			}
			chapterMatches[chapterKey] = appendUniqueChapterProviderOption(chapterMatches[chapterKey], option)
		}
	}

	for _, chapter := range primary.Chapters {
		if chapter == nil {
			continue
		}

		exactKey, chapterKey := getAlternativeProviderMatchKeys(chapter)

		var matches []hibikemanga.ChapterProviderOption
		if exactKey != "" {
			matches = append(matches, exactMatches[exactKey]...)
		}
		if len(matches) == 0 && chapterKey != "" {
			matches = append(matches, chapterMatches[chapterKey]...)
		}

		sortChapterProviderOptions(matches)
		chapter.AlternativeProviders = matches
	}
}

func getAlternativeProviderMatchKeys(chapter *hibikemanga.ChapterDetails) (exact string, chapterOnly string) {
	if chapter == nil {
		return "", ""
	}

	chapterNumber := strings.TrimSpace(chapter.Chapter)
	if chapterNumber == "" {
		return "", ""
	}

	normalizedChapter := strings.ToLower(manga_providers.GetNormalizedChapter(chapterNumber))
	if normalizedChapter == "" {
		return "", ""
	}

	language := strings.ToLower(strings.TrimSpace(chapter.Language))
	scanlator := strings.ToLower(strings.TrimSpace(chapter.Scanlator))

	return fmt.Sprintf("%s|%s|%s", normalizedChapter, language, scanlator), normalizedChapter
}

func appendUniqueChapterProviderOption(
	options []hibikemanga.ChapterProviderOption,
	option hibikemanga.ChapterProviderOption,
) []hibikemanga.ChapterProviderOption {
	for _, existing := range options {
		if existing.Provider == option.Provider && existing.ChapterID == option.ChapterID {
			return options
		}
	}

	return append(options, option)
}

func sortChapterProviderOptions(options []hibikemanga.ChapterProviderOption) {
	slices.SortFunc(options, func(a, b hibikemanga.ChapterProviderOption) int {
		if diff := cmp.Compare(a.Provider, b.Provider); diff != 0 {
			return diff
		}
		return cmp.Compare(a.ChapterID, b.ChapterID)
	})
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RefreshChapterContainers deletes all cached chapter containers and refetches them based on the selected provider map.
func (r *Repository) RefreshChapterContainers(mangaCollection *anilist.MangaCollection, selectedProviderMap map[int]string) (err error) {
	defer util.HandlePanicInModuleWithError("manga/RefreshChapterContainers", &err)

	// first, delete all chapter containers in the cache (RemoveAllBy also evicts
	// the matching in-memory stores)
	err = r.fileCacher.RemoveAllBy(func(filename string) bool {
		return strings.HasPrefix(filename, "manga_")
	})

	mangaLatestChapterNumberMap.Delete(ChapterCountMapCacheKey)

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	rateLimiters := make(map[string]*limiter.Limiter)
	// make a limiter for each provider
	for _, selectedProviderId := range selectedProviderMap {
		// 2 requests per second
		rateLimiters[selectedProviderId] = limiter.NewLimiter(time.Second, 2)
	}

	wg.Add(len(selectedProviderMap))
	for mediaId, selectedProviderId := range selectedProviderMap {
		go func() {
			defer wg.Done()
			// Get the selected provider
			provider, ok := r.extensionBankRef.Get().Get(selectedProviderId)
			if !ok {
				r.logger.Warn().Str("provider", selectedProviderId).Int("mediaId", mediaId).Msg("manga: Provider not found")
				return
			}

			// Get the manga from the collection
			mangaEntry, found := mangaCollection.GetListEntryFromMangaId(mediaId)
			if !found {
				return
			}

			// If the manga is not currently reading or repeating, continue
			if *mangaEntry.GetStatus() != anilist.MediaListStatusCurrent && *mangaEntry.GetStatus() != anilist.MediaListStatusRepeating {
				return
			}

			mu.Lock()
			rateLimiter, ok := rateLimiters[selectedProviderId]
			mu.Unlock()
			if ok {
				rateLimiter.Wait()
			}

			// Refetch the container
			_, err = r.GetMangaChapterContainer(&GetMangaChapterContainerOptions{
				Provider: provider.GetID(),
				MediaId:  mediaId,
				Titles:   mangaEntry.GetMedia().GetAllTitles(),
				Year:     mangaEntry.GetMedia().GetStartYearSafe(),
			})
			if err != nil {
				r.logger.Warn().Err(err).Str("provider", provider.GetID()).Int("mediaId", mediaId).Msg("manga: Failed to fetch chapter container")
				return
			}

			r.logger.Trace().Str("provider", provider.GetID()).Int("mediaId", mediaId).Msg("manga: Fetched chapter container")
		}()
	}
	wg.Wait()

	return nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

const ChapterCountMapCacheKey = 1

var mangaLatestChapterNumberMap = result.NewMap[int, map[int][]MangaLatestChapterNumberItem]()

type MangaLatestChapterNumberItem struct {
	Provider       string `json:"provider"`
	SourceProvider string `json:"sourceProvider"`
	Scanlator      string `json:"scanlator"`
	Language       string `json:"language"`
	Number         int    `json:"number"`
}

// GetMangaLatestChapterNumbersMap retrieves the latest chapter number for all manga entries.
// It scans the cache directory for chapter containers and counts the number of chapters fetched from the provider for each manga.
//
// Unlike [GetMangaLatestChapterNumberMap], it will segregate the chapter numbers by source provider, scanlator and language.
func (r *Repository) GetMangaLatestChapterNumbersMap() (ret map[int][]MangaLatestChapterNumberItem, err error) {
	defer util.HandlePanicInModuleThen("manga/GetMangaLatestChapterNumbersMap", func() {})
	ret = make(map[int][]MangaLatestChapterNumberItem)

	if m, ok := mangaLatestChapterNumberMap.Get(ChapterCountMapCacheKey); ok {
		ret = m
		return
	}

	// Go through all chapter container caches
	entries, err := os.ReadDir(r.cacheDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Get the provider and mediaId from the file cache name
		provider, mediaId, ok := parseChapterFileName(entry.Name())
		if !ok {
			continue
		}

		containerBucket := r.getFcProviderBucket(provider, mediaId, bucketTypeChapter)

		// Get the container from the file cache
		var container *ChapterContainer
		chapterContainerKey := getMangaChapterContainerCacheKey(provider, mediaId)
		if found, _ := r.fileCacher.Get(containerBucket, chapterContainerKey, &container); !found {
			continue
		}

		groupBySourceProvider := lo.GroupBy(container.Chapters, func(c *hibikemanga.ChapterDetails) string {
			return c.SourceProvider
		})

		for sourceProvider, sourceChapters := range groupBySourceProvider {
			groupByScanlator := lo.GroupBy(sourceChapters, func(c *hibikemanga.ChapterDetails) string {
				return c.Scanlator
			})

			for scanlator, chapters := range groupByScanlator {
				groupByLanguage := lo.GroupBy(chapters, func(c *hibikemanga.ChapterDetails) string {
					return c.Language
				})

				for language, chapters := range groupByLanguage {
					chapterCount := getLatestMangaChapterNumber(chapters)

					if _, ok := ret[mediaId]; !ok {
						ret[mediaId] = []MangaLatestChapterNumberItem{}
					}

					ret[mediaId] = append(ret[mediaId], MangaLatestChapterNumberItem{
						Provider:       provider,
						SourceProvider: sourceProvider,
						Scanlator:      scanlator,
						Language:       language,
						Number:         chapterCount,
					})
				}
			}
		}
	}

	// Trigger hook event
	ev := &MangaLatestChapterNumbersMapEvent{
		LatestChapterNumbersMap: ret,
	}
	err = hook.GlobalHookManager.OnMangaLatestChapterNumbersMap().Trigger(ev)
	if err != nil {
		r.logger.Error().Err(err).Msg("manga: Exception occurred while triggering hook event")
	}
	ret = ev.LatestChapterNumbersMap

	mangaLatestChapterNumberMap.Set(ChapterCountMapCacheKey, ret)
	return
}

func getLatestMangaChapterNumber(chapters []*hibikemanga.ChapterDetails) int {
	latest := 0.0
	for _, chapter := range chapters {
		if chapter == nil {
			continue
		}
		number, err := strconv.ParseFloat(chapter.Chapter, 64)
		if err == nil && number > latest {
			latest = number
		}
	}
	return int(math.Floor(latest))
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func parseChapterFileName(dirName string) (provider string, mId int, ok bool) {
	if !strings.HasPrefix(dirName, "manga_") {
		return "", 0, false
	}
	dirName = strings.TrimSuffix(dirName, ".cache")
	parts := strings.Split(dirName, "_")
	if len(parts) != 4 {
		return "", 0, false
	}

	provider = parts[1]
	mId, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, false
	}

	return provider, mId, true
}

// HydrateSearchResultSearchRating rates the search results based on the provided title
// It checks if all search results have a rating of 0 and if so, it calculates ratings
// using the Sorensen-Dice
func HydrateSearchResultSearchRating(_searchRes []*hibikemanga.SearchResult, title *string) {
	// Rate the search results if all ratings are 0
	if noRatings := lo.EveryBy(_searchRes, func(res *hibikemanga.SearchResult) bool {
		return res.SearchRating == 0
	}); noRatings {
		wg := sync.WaitGroup{}
		wg.Add(len(_searchRes))
		for _, res := range _searchRes {
			go func(res *hibikemanga.SearchResult) {
				defer wg.Done()

				compTitles := []*string{&res.Title}
				if res.Synonyms == nil || len(res.Synonyms) == 0 {
					return
				}
				for _, syn := range res.Synonyms {
					compTitles = append(compTitles, &syn)
				}

				compRes, ok := comparison.FindBestMatchWithSorensenDice(title, compTitles)
				if !ok {
					return
				}

				res.SearchRating = compRes.Rating
				return
			}(res)
		}
		wg.Wait()
	}
}
