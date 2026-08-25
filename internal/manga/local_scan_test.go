package manga

import (
	"os"
	"path/filepath"
	manga_providers "seanime/internal/manga/providers"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanLocalMangaLibraryMapsConfidentMatches(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)
	writeLocalChapter(t, filepath.Join(root, "[Group] Berserk (2021)"), "Chapter 1.cbz", 2)
	writeLocalChapter(t, filepath.Join(root, "Something Nobody Reads"), "Chapter 1.cbz", 2)

	collection := newLocalMangaCollection(t, map[int]string{
		21:  "One Piece",
		30:  "Berserk",
		105: "Vinland Saga",
	})

	result, err := repository.ScanLocalMangaLibrary(collection, &LocalMangaScanOptions{SelectAsSource: true})
	require.NoError(t, err)
	require.Equal(t, 3, result.SeriesCount)

	matchedByDir := make(map[string]*LocalMangaScanMatch, len(result.Matched))
	for _, match := range result.Matched {
		matchedByDir[match.DirName] = match
	}
	require.Len(t, matchedByDir, 2)

	require.Equal(t, 21, matchedByDir["One Piece"].MediaId)
	// Bracketed metadata must not stop the folder from matching.
	require.Equal(t, 30, matchedByDir["[Group] Berserk (2021)"].MediaId)

	for _, match := range result.Matched {
		require.True(t, match.SelectedAsSource, "%s should now read from the local provider", match.DirName)

		mapping, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, match.MediaId)
		require.True(t, found)
		require.Equal(t, match.DirName, mapping.MangaID)
	}

	preferences, err := repository.GetMangaPreferences()
	require.NoError(t, err)
	require.Equal(t, manga_providers.LocalProvider, preferences.Entries[21].Provider)

	require.Len(t, result.Skipped, 1)
	require.Equal(t, "Something Nobody Reads", result.Skipped[0].DirName)
	require.Contains(t, []string{LocalMangaSkipNoMatch, LocalMangaSkipLowConfidence}, result.Skipped[0].Reason)
}

func TestScanLocalMangaLibrarySkipsMappedAndEmptySeries(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)
	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 999, "One Piece"))

	// A directory with nothing the chapter scanner recognises.
	require.NoError(t, writeEmptySeries(root, "Berserk"))

	collection := newLocalMangaCollection(t, map[int]string{21: "One Piece", 30: "Berserk"})

	result, err := repository.ScanLocalMangaLibrary(collection, nil)
	require.NoError(t, err)
	require.Empty(t, result.Matched)

	reasons := make(map[string]string, len(result.Skipped))
	for _, skipped := range result.Skipped {
		reasons[skipped.DirName] = skipped.Reason
	}
	require.Equal(t, LocalMangaSkipAlreadyMapped, reasons["One Piece"])
	require.Equal(t, LocalMangaSkipNoChapters, reasons["Berserk"])

	// The hand-made mapping must survive a scan that was not asked to re-map.
	mapping, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, 999)
	require.True(t, found)
	require.Equal(t, "One Piece", mapping.MangaID)
}

func TestScanLocalMangaLibraryRemapsWhenAsked(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)
	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 999, "One Piece"))

	collection := newLocalMangaCollection(t, map[int]string{21: "One Piece"})

	result, err := repository.ScanLocalMangaLibrary(collection, &LocalMangaScanOptions{Remap: true})
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	require.Equal(t, 21, result.Matched[0].MediaId)

	mapping, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, 21)
	require.True(t, found)
	require.Equal(t, "One Piece", mapping.MangaID)

	// The entry the folder used to belong to must let go of it, or both entries
	// would keep listing the same chapters.
	_, found = repository.db.GetMangaMapping(manga_providers.LocalProvider, 999)
	require.False(t, found)
}

func TestMapLocalMangaSeries(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)

	require.NoError(t, repository.MapLocalMangaSeries(21, "One Piece"))
	mapping, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, 21)
	require.True(t, found)
	require.Equal(t, "One Piece", mapping.MangaID)

	// Handing the folder to another entry takes it away from the first.
	require.NoError(t, repository.MapLocalMangaSeries(30, "One Piece"))
	_, found = repository.db.GetMangaMapping(manga_providers.LocalProvider, 21)
	require.False(t, found)

	require.ErrorIs(t, repository.MapLocalMangaSeries(21, "Nonexistent"), ErrLocalMangaSeriesNotFound)
	require.ErrorIs(t, repository.MapLocalMangaSeries(21, "../escape"), ErrInvalidLocalMangaName)
	require.Error(t, repository.MapLocalMangaSeries(0, "One Piece"))
}

func TestScanLocalMangaLibraryGivesEachEntryToOneSeries(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	// Two folders that both resolve to the same entry; only the better one may
	// claim it, or one would silently overwrite the other's mapping.
	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)
	writeLocalChapter(t, filepath.Join(root, "One Piece!"), "Chapter 1.cbz", 2)

	collection := newLocalMangaCollection(t, map[int]string{21: "One Piece"})

	result, err := repository.ScanLocalMangaLibrary(collection, nil)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	require.Equal(t, "One Piece", result.Matched[0].DirName)

	require.Len(t, result.Skipped, 1)
	require.Equal(t, "One Piece!", result.Skipped[0].DirName)
	require.Equal(t, LocalMangaSkipMediaTaken, result.Skipped[0].Reason)
	require.NotNil(t, result.Skipped[0].MediaId)
	require.Equal(t, 21, *result.Skipped[0].MediaId)
}

func TestScanLocalMangaLibraryReportsCandidatesForNearMisses(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "Vinland"), "Chapter 1.cbz", 2)

	collection := newLocalMangaCollection(t, map[int]string{105: "Vinland Saga"})

	result, err := repository.ScanLocalMangaLibrary(collection, nil)
	require.NoError(t, err)
	require.Empty(t, result.Matched)
	require.Len(t, result.Skipped, 1)

	skipped := result.Skipped[0]
	require.Equal(t, LocalMangaSkipLowConfidence, skipped.Reason)
	require.NotEmpty(t, skipped.Candidates)
	require.Equal(t, 105, skipped.Candidates[0].MediaId)
	require.Less(t, skipped.Candidates[0].Rating, localMangaMatchThreshold)
}

func TestScanLocalMangaLibraryKeepsExistingSource(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)

	// The entry already reads from an online source; the scan must map it without
	// switching the user's reading source over.
	provider := "comick"
	_, err := repository.PatchPreference(21, &MangaPreferencePatch{Provider: &provider}, false)
	require.NoError(t, err)

	result, err := repository.ScanLocalMangaLibrary(
		newLocalMangaCollection(t, map[int]string{21: "One Piece"}),
		&LocalMangaScanOptions{SelectAsSource: true},
	)
	require.NoError(t, err)
	require.Len(t, result.Matched, 1)
	require.False(t, result.Matched[0].SelectedAsSource)

	preferences, err := repository.GetMangaPreferences()
	require.NoError(t, err)
	require.Equal(t, "comick", preferences.Entries[21].Provider)
}

func TestScanLocalMangaLibraryRequiresACollection(t *testing.T) {
	repository, _ := newLocalMangaTestRepository(t)

	_, err := repository.ScanLocalMangaLibrary(nil, nil)
	require.Error(t, err)

	_, err = repository.ScanLocalMangaLibrary(newLocalMangaCollection(t, nil), nil)
	require.Error(t, err)
}

func writeEmptySeries(root string, name string) error {
	return os.MkdirAll(filepath.Join(root, name), 0o755)
}
