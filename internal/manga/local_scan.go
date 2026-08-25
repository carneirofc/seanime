package manga

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"seanime/internal/api/anilist"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"seanime/internal/util/comparison"
	"sort"
	"strings"

	"github.com/sourcegraph/conc/pool"
)

const (
	// localMangaMatchThreshold is the similarity a directory name must reach
	// before the scan maps it on its own.
	//
	// Mapping the wrong entry is worse than mapping none: the user reads the
	// wrong series and their AniList progress is updated against it. The bar is
	// therefore set where a title has to be recognisably the same, not merely the
	// closest of the available options.
	localMangaMatchThreshold = 0.8

	// localMangaSuggestionThreshold is the similarity below which a candidate is
	// not even worth showing the user as a manual option.
	localMangaSuggestionThreshold = 0.45

	// localMangaMaxSuggestions bounds the candidates reported per unmatched
	// series.
	localMangaMaxSuggestions = 5
)

// Reasons a series was not mapped, as reported to the client.
const (
	LocalMangaSkipAlreadyMapped = "already_mapped"
	LocalMangaSkipNoChapters    = "no_chapters"
	LocalMangaSkipNoMatch       = "no_match"
	LocalMangaSkipLowConfidence = "low_confidence"
	LocalMangaSkipMediaTaken    = "media_taken"
)

type (
	// LocalMangaScanOptions controls one run of the local library scan.
	LocalMangaScanOptions struct {
		// Remap re-evaluates series that already have a mapping. Off by default:
		// a mapping the user made by hand outranks anything the scan infers.
		Remap bool
		// SelectAsSource makes the local provider the reading source of a newly
		// mapped entry that has no source selected yet.
		SelectAsSource bool
	}

	// LocalMangaScanCandidate is an AniList entry a series could belong to.
	LocalMangaScanCandidate struct {
		MediaId int     `json:"mediaId"`
		Title   string  `json:"title"`
		Rating  float64 `json:"rating"`
	}

	// LocalMangaScanMatch is a series the scan mapped.
	LocalMangaScanMatch struct {
		DirName      string  `json:"dirName"`
		Title        string  `json:"title"`
		MediaId      int     `json:"mediaId"`
		MediaTitle   string  `json:"mediaTitle"`
		Rating       float64 `json:"rating"`
		ChapterCount int     `json:"chapterCount"`
		// SelectedAsSource reports that the entry now reads from the local
		// provider because it had no source of its own.
		SelectedAsSource bool `json:"selectedAsSource"`
	}

	// LocalMangaScanSkipped is a series the scan left alone, with enough context
	// for the user to finish the job by hand.
	LocalMangaScanSkipped struct {
		DirName    string                     `json:"dirName"`
		Title      string                     `json:"title"`
		Reason     string                     `json:"reason"`
		MediaId    *int                       `json:"mediaId"`
		Candidates []*LocalMangaScanCandidate `json:"candidates"`
	}

	// LocalMangaScanResult is the report of a completed scan.
	LocalMangaScanResult struct {
		SeriesCount int                      `json:"seriesCount"`
		Matched     []*LocalMangaScanMatch   `json:"matched"`
		Skipped     []*LocalMangaScanSkipped `json:"skipped"`
	}
)

// ScanLocalMangaLibrary walks the local manga library and maps each series
// directory to the AniList manga entry it belongs to.
//
// This is the manga counterpart of the anime library scan: the user drops files
// into a directory laid out one folder per series, and the scan works out which
// entry each folder is. Only entries in the user's AniList manga collection are
// considered — a mapping is a media ID, and an ID that is not in the collection
// has nothing to display it against.
//
// The scan is read-only towards the filesystem; the only thing it writes is
// mappings.
func (r *Repository) ScanLocalMangaLibrary(
	collection *anilist.MangaCollection,
	opts *LocalMangaScanOptions,
) (ret *LocalMangaScanResult, err error) {
	defer util.HandlePanicInModuleWithError("manga/ScanLocalMangaLibrary", &err)

	if opts == nil {
		opts = &LocalMangaScanOptions{}
	}
	if collection == nil || collection.MediaListCollection == nil {
		return nil, errors.New("manga: the AniList manga collection is unavailable")
	}

	provider, err := r.localMangaProvider()
	if err != nil {
		return nil, err
	}

	series, err := provider.ListSeries()
	if err != nil {
		return nil, err
	}

	index := newMangaTitleIndex(collection)
	if index.empty() {
		return nil, errors.New("manga: your AniList manga collection is empty, add the series before scanning")
	}

	existing := r.localMangaMappings()
	// Media already spoken for cannot be claimed again in this run: two folders
	// resolving to the same entry would leave whichever applied last silently
	// overwriting the other.
	claimed := make(map[int]string, len(existing))
	for dirName, mediaId := range existing {
		claimed[mediaId] = dirName
	}

	ret = &LocalMangaScanResult{
		SeriesCount: len(series),
		Matched:     make([]*LocalMangaScanMatch, 0),
		Skipped:     make([]*LocalMangaScanSkipped, 0),
	}

	// Every directory is compared against every title in the collection, so the
	// matching is done up front and in parallel; the decisions that follow depend
	// on each other and stay sequential.
	matchPool := pool.New().WithMaxGoroutines(runtime.NumCPU())
	toMatch := make([]*localMangaProposal, 0, len(series))
	proposals := make([]*localMangaProposal, 0, len(series))

	for _, info := range series {
		title := NormalizeLocalMangaTitle(info.DirName)

		if mediaId, mapped := existing[info.DirName]; mapped && !opts.Remap {
			ret.Skipped = append(ret.Skipped, &LocalMangaScanSkipped{
				DirName:    info.DirName,
				Title:      title,
				Reason:     LocalMangaSkipAlreadyMapped,
				MediaId:    &mediaId,
				Candidates: make([]*LocalMangaScanCandidate, 0),
			})
			continue
		}

		if info.ChapterCount == 0 {
			ret.Skipped = append(ret.Skipped, &LocalMangaScanSkipped{
				DirName:    info.DirName,
				Title:      title,
				Reason:     LocalMangaSkipNoChapters,
				Candidates: make([]*LocalMangaScanCandidate, 0),
			})
			continue
		}

		proposal := &localMangaProposal{dirName: info.DirName, title: title, chapterCount: info.ChapterCount}
		toMatch = append(toMatch, proposal)
		matchPool.Go(func() {
			proposal.candidates = index.bestCandidates(proposal.title, localMangaMaxSuggestions)
		})
	}

	matchPool.Wait()

	for _, proposal := range toMatch {
		candidates := proposal.candidates
		if len(candidates) == 0 || candidates[0].Rating < localMangaMatchThreshold {
			reason := LocalMangaSkipNoMatch
			if len(candidates) > 0 {
				reason = LocalMangaSkipLowConfidence
			}
			ret.Skipped = append(ret.Skipped, &LocalMangaScanSkipped{
				DirName:    proposal.dirName,
				Title:      proposal.title,
				Reason:     reason,
				Candidates: candidates,
			})
			continue
		}

		proposals = append(proposals, proposal)
	}

	// Strongest matches are applied first so that when two folders want the same
	// entry, the better one keeps it.
	sort.SliceStable(proposals, func(i, j int) bool {
		return proposals[i].candidates[0].Rating > proposals[j].candidates[0].Rating
	})

	for _, proposal := range proposals {
		best := proposal.candidates[0]

		if owner, taken := claimed[best.MediaId]; taken && owner != proposal.dirName {
			mediaId := best.MediaId
			ret.Skipped = append(ret.Skipped, &LocalMangaScanSkipped{
				DirName:    proposal.dirName,
				Title:      proposal.title,
				Reason:     LocalMangaSkipMediaTaken,
				MediaId:    &mediaId,
				Candidates: proposal.candidates,
			})
			continue
		}

		selectedAsSource, err := r.applyLocalMangaMapping(best.MediaId, proposal.dirName, opts.SelectAsSource)
		if err != nil {
			r.logger.Error().Err(err).
				Str("series", proposal.dirName).
				Int("mediaId", best.MediaId).
				Msg("manga: Failed to map local series")
			ret.Skipped = append(ret.Skipped, &LocalMangaScanSkipped{
				DirName:    proposal.dirName,
				Title:      proposal.title,
				Reason:     LocalMangaSkipNoMatch,
				Candidates: proposal.candidates,
			})
			continue
		}

		claimed[best.MediaId] = proposal.dirName
		ret.Matched = append(ret.Matched, &LocalMangaScanMatch{
			DirName:          proposal.dirName,
			Title:            proposal.title,
			MediaId:          best.MediaId,
			MediaTitle:       best.Title,
			Rating:           best.Rating,
			ChapterCount:     proposal.chapterCount,
			SelectedAsSource: selectedAsSource,
		})
	}

	sort.SliceStable(ret.Matched, func(i, j int) bool {
		return strings.ToLower(ret.Matched[i].DirName) < strings.ToLower(ret.Matched[j].DirName)
	})
	sort.SliceStable(ret.Skipped, func(i, j int) bool {
		return strings.ToLower(ret.Skipped[i].DirName) < strings.ToLower(ret.Skipped[j].DirName)
	})

	r.logger.Info().
		Int("series", ret.SeriesCount).
		Int("matched", len(ret.Matched)).
		Int("skipped", len(ret.Skipped)).
		Msg("manga: Scanned local manga library")

	return ret, nil
}

type localMangaProposal struct {
	dirName      string
	title        string
	chapterCount int
	candidates   []*LocalMangaScanCandidate
}

// MapLocalMangaSeries maps an AniList entry to a series directory of the local
// library, replacing whatever the entry mapped to before.
func (r *Repository) MapLocalMangaSeries(mediaId int, series string) (err error) {
	defer util.HandlePanicInModuleWithError("manga/MapLocalMangaSeries", &err)

	if mediaId <= 0 {
		return errors.New("invalid media id")
	}

	seriesDir, err := r.resolveLocalMangaSeriesDir(series)
	if err != nil {
		return err
	}

	// Mapping to a folder that is not there would look like it worked and then
	// show an empty chapter list, with nothing to say why.
	if info, statErr := os.Stat(seriesDir); statErr != nil || !info.IsDir() {
		return ErrLocalMangaSeriesNotFound
	}

	_, err = r.applyLocalMangaMapping(mediaId, filepath.Base(seriesDir), false)
	return err
}

// applyLocalMangaMapping records the mapping, drops the entry's cached chapter
// list and — when asked and when the entry has no source yet — makes the local
// provider the one it reads from.
//
// The preference is only ever filled in, never changed: an entry the user reads
// from an online source stays there even once a local copy exists.
func (r *Repository) applyLocalMangaMapping(mediaId int, series string, selectAsSource bool) (selected bool, err error) {
	r.releaseLocalMangaSeries(series, mediaId)

	if err := r.db.InsertMangaMapping(manga_providers.LocalProvider, mediaId, series); err != nil {
		return false, err
	}

	bucket := r.getFcProviderBucket(manga_providers.LocalProvider, mediaId, bucketTypeChapter)
	_ = r.fileCacher.Remove(bucket.Name())

	if !selectAsSource {
		return false, nil
	}

	preferences, err := r.GetMangaPreferences()
	if err != nil {
		r.logger.Warn().Err(err).Msg("manga: Failed to read preferences while mapping local series")
		return false, nil
	}
	if preferences != nil && preferences.Entries[mediaId].Provider != "" {
		return false, nil
	}

	provider := manga_providers.LocalProvider
	if _, err := r.PatchPreference(mediaId, &MangaPreferencePatch{Provider: &provider}, false); err != nil {
		r.logger.Warn().Err(err).Int("mediaId", mediaId).Msg("manga: Failed to select the local provider")
		return false, nil
	}

	return true, nil
}

// releaseLocalMangaSeries drops any other entry's mapping to a series folder.
//
// A folder holds one series, so it belongs to one entry. Without this, mapping a
// folder to a new entry would leave the previous one pointing at it too — the
// old entry would keep listing chapters that are no longer meant to be its own.
func (r *Repository) releaseLocalMangaSeries(series string, keepMediaId int) {
	mappings, err := r.db.GetMangaMappingsByProvider(manga_providers.LocalProvider)
	if err != nil {
		r.logger.Warn().Err(err).Msg("manga: Failed to read local manga mappings")
		return
	}

	for _, mapping := range mappings {
		if mapping == nil || mapping.MangaID != series || mapping.MediaID == keepMediaId {
			continue
		}
		if err := r.RemoveMapping(manga_providers.LocalProvider, mapping.MediaID); err != nil {
			r.logger.Warn().Err(err).
				Int("mediaId", mapping.MediaID).
				Str("series", series).
				Msg("manga: Failed to release local series from its previous entry")
		}
	}
}

// mangaTitleIndex holds every title of every entry in the collection, so a
// directory name can be compared against all of them at once.
type mangaTitleIndex struct {
	titles       []*string
	mediaIdByPtr map[*string]int
	titleByMedia map[int]string
}

func newMangaTitleIndex(collection *anilist.MangaCollection) *mangaTitleIndex {
	index := &mangaTitleIndex{
		titles:       make([]*string, 0),
		mediaIdByPtr: make(map[*string]int),
		titleByMedia: make(map[int]string),
	}

	seen := make(map[int]struct{})

	for _, list := range collection.MediaListCollection.Lists {
		for _, entry := range list.GetEntries() {
			if entry == nil || entry.GetMedia() == nil {
				continue
			}
			media := entry.GetMedia()
			if _, done := seen[media.ID]; done {
				continue
			}
			seen[media.ID] = struct{}{}
			index.titleByMedia[media.ID] = media.GetPreferredTitle()

			for _, title := range media.GetAllTitles() {
				if title == nil || strings.TrimSpace(*title) == "" {
					continue
				}
				normalized := NormalizeLocalMangaTitle(*title)
				if normalized == "" {
					continue
				}
				index.titles = append(index.titles, &normalized)
				index.mediaIdByPtr[&normalized] = media.ID
			}
		}
	}

	return index
}

func (index *mangaTitleIndex) empty() bool {
	return len(index.titles) == 0
}

// bestCandidates returns the closest entries to a directory title, best first,
// at most one row per entry.
func (index *mangaTitleIndex) bestCandidates(title string, limit int) []*LocalMangaScanCandidate {
	ret := make([]*LocalMangaScanCandidate, 0, limit)
	if strings.TrimSpace(title) == "" {
		return ret
	}

	bestByMedia := make(map[int]float64)
	for _, result := range comparison.CompareWithSorensenDice(&title, index.titles) {
		if result == nil || result.Value == nil {
			continue
		}
		mediaId, ok := index.mediaIdByPtr[result.Value]
		if !ok {
			continue
		}
		if rating, seen := bestByMedia[mediaId]; !seen || result.Rating > rating {
			bestByMedia[mediaId] = result.Rating
		}
	}

	for mediaId, rating := range bestByMedia {
		if rating < localMangaSuggestionThreshold {
			continue
		}
		ret = append(ret, &LocalMangaScanCandidate{
			MediaId: mediaId,
			Title:   index.titleByMedia[mediaId],
			Rating:  rating,
		})
	}

	sort.SliceStable(ret, func(i, j int) bool {
		if ret[i].Rating != ret[j].Rating {
			return ret[i].Rating > ret[j].Rating
		}
		return ret[i].MediaId < ret[j].MediaId
	})

	if len(ret) > limit {
		ret = ret[:limit]
	}

	return ret
}
