package manga

import (
	"os"
	"path/filepath"
	"seanime/internal/database/models"
	"seanime/internal/extension"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/testutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLocalMangaTitle(t *testing.T) {
	cases := map[string]string{
		"One Piece":                        "One Piece",
		"one_piece":                        "one piece",
		"[Group] Berserk (2021)":           "Berserk",
		"Vinland.Saga":                     "Vinland Saga",
		"One-Piece":                        "One Piece",
		"Berserk - Deluxe":                 "Berserk Deluxe",
		"Chainsaw Man [Digital] {v01-v11}": "Chainsaw Man",
		"Solo Leveling.cbz":                "Solo Leveling",
		"  spaced   out  ":                 "spaced out",
		"":                                 "",
	}

	for input, expected := range cases {
		require.Equal(t, expected, NormalizeLocalMangaTitle(input), "input %q", input)
	}
}

func TestSanitizeLocalMangaName(t *testing.T) {
	valid := []string{"One Piece", "Berserk - Deluxe", "第1巻", "a"}
	for _, name := range valid {
		got, err := SanitizeLocalMangaName(name)
		require.NoError(t, err, "name %q", name)
		require.Equal(t, name, got)
	}

	invalid := []string{
		"",
		"   ",
		"..",
		"../escape",
		"nested/name",
		`nested\name`,
		"colon:name",
		"wild*card",
		"pipe|name",
		"con",
		"LPT1.cbz",
		"bell\x07",
	}
	for _, name := range invalid {
		_, err := SanitizeLocalMangaName(name)
		require.ErrorIs(t, err, ErrInvalidLocalMangaName, "name %q should be rejected", name)
	}

	// Surrounding whitespace and dots are trimmed rather than rejected.
	got, err := SanitizeLocalMangaName("  One Piece.  ")
	require.NoError(t, err)
	require.Equal(t, "One Piece", got)
}

func TestResolveLocalMangaSeriesDirRejectsTraversal(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	dir, err := repository.resolveLocalMangaSeriesDir("One Piece")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "One Piece"), dir)

	for _, name := range []string{"../outside", "..", "/etc", `..\outside`} {
		_, err := repository.resolveLocalMangaSeriesDir(name)
		require.ErrorIs(t, err, ErrInvalidLocalMangaName, "name %q should be rejected", name)
	}
}

func TestLocalMangaDirectoryPrefersConfiguredSource(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	dir, configured := repository.LocalMangaDirectory()
	require.Equal(t, root, dir)
	require.True(t, configured)

	repository.SetSettings(&models.Settings{Manga: &models.MangaSettings{}})
	dir, configured = repository.LocalMangaDirectory()
	require.Equal(t, repository.localDir, dir)
	require.False(t, configured)
}

func TestGetLocalMangaLibraryReportsMappings(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 3)
	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 2.cbz", 3)
	writeLocalChapter(t, filepath.Join(root, "Berserk"), "Chapter 1.cbz", 2)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Empty"), 0o755))

	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 21, "One Piece"))

	library, err := repository.GetLocalMangaLibrary(newLocalMangaCollection(t, map[int]string{21: "One Piece"}))
	require.NoError(t, err)
	require.True(t, library.Configured)
	require.Len(t, library.Series, 3)

	byName := make(map[string]*LocalMangaSeries, len(library.Series))
	for _, series := range library.Series {
		byName[series.DirName] = series
	}

	require.Equal(t, 2, byName["One Piece"].ChapterCount)
	require.NotNil(t, byName["One Piece"].MediaId)
	require.Equal(t, 21, *byName["One Piece"].MediaId)
	require.Equal(t, "One Piece", byName["One Piece"].MediaTitle)
	require.Positive(t, byName["One Piece"].Size)

	require.Nil(t, byName["Berserk"].MediaId)
	require.Equal(t, 0, byName["Empty"].ChapterCount)
	require.Equal(t, 2, library.UnmappedCount)
}

func TestDeleteLocalMangaSeriesRemovesMapping(t *testing.T) {
	repository, root := newLocalMangaTestRepository(t)

	writeLocalChapter(t, filepath.Join(root, "One Piece"), "Chapter 1.cbz", 2)
	require.NoError(t, repository.db.InsertMangaMapping(manga_providers.LocalProvider, 21, "One Piece"))

	require.NoError(t, repository.DeleteLocalMangaSeries("One Piece"))

	require.NoDirExists(t, filepath.Join(root, "One Piece"))
	_, found := repository.db.GetMangaMapping(manga_providers.LocalProvider, 21)
	require.False(t, found)

	require.ErrorIs(t, repository.DeleteLocalMangaSeries("One Piece"), ErrLocalMangaSeriesNotFound)
	require.ErrorIs(t, repository.DeleteLocalMangaSeries("../outside"), ErrInvalidLocalMangaName)
}

// newLocalMangaTestRepository returns a repository whose local manga library is
// an empty directory, along with that directory.
func newLocalMangaTestRepository(t *testing.T) (*Repository, string) {
	t.Helper()

	env := testutil.NewTestEnv(t)
	repository := NewTestRepositoryWithEnv(env, env.NewDatabase("manga_local"))

	root := env.MustMkdirData("manga-local-source")
	repository.SetSettings(&models.Settings{Manga: &models.MangaSettings{LocalSourceDirectory: root}})

	logger := zerolog.Nop()
	repository.logger = &logger

	repository.extensionBankRef.Get().Set(
		manga_providers.LocalProvider,
		extension.NewMangaProviderExtension(&extension.Extension{
			ID:   manga_providers.LocalProvider,
			Name: "Local",
			Type: extension.TypeMangaProvider,
		}, manga_providers.NewLocal(root, repository.logger)),
	)

	return repository, root
}
