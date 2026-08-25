package db

import (
	"seanime/internal/database/models"
	"seanime/internal/util"
	"testing"

	"github.com/stretchr/testify/require"
)

func newMangaTestDatabase(t *testing.T) *Database {
	t.Helper()

	logger := util.NewLogger()
	database, err := NewDatabase(t.TempDir(), "manga_mapping_test", logger)
	require.NoError(t, err)

	// The mapping cache is package-level and shared between tests, so a stale
	// entry from another test would answer instead of the database.
	mangaMappingCache.Clear()

	return database
}

func TestInsertMangaMappingReplacesPrevious(t *testing.T) {
	database := newMangaTestDatabase(t)

	require.NoError(t, database.InsertMangaMapping("local-manga", 21, "One Piece"))
	require.NoError(t, database.InsertMangaMapping("local-manga", 21, "One Piece (Colored)"))

	// Reading past the cache is the point: the stale row only used to surface
	// once the in-memory entry was gone, i.e. after a restart.
	mangaMappingCache.Clear()

	mapping, found := database.GetMangaMapping("local-manga", 21)
	require.True(t, found)
	require.Equal(t, "One Piece (Colored)", mapping.MangaID)

	var rows []*models.MangaMapping
	require.NoError(t, database.gormdb.Where("provider = ? AND media_id = ?", "local-manga", 21).Find(&rows).Error)
	require.Len(t, rows, 1, "the superseded mapping must not be left behind")
}

func TestGetMangaMappingsByProvider(t *testing.T) {
	database := newMangaTestDatabase(t)

	require.NoError(t, database.InsertMangaMapping("local-manga", 21, "One Piece"))
	require.NoError(t, database.InsertMangaMapping("local-manga", 30, "Berserk"))
	require.NoError(t, database.InsertMangaMapping("comick", 21, "one-piece"))

	mappings, err := database.GetMangaMappingsByProvider("local-manga")
	require.NoError(t, err)
	require.Len(t, mappings, 2)

	byMediaId := make(map[int]string, len(mappings))
	for _, mapping := range mappings {
		byMediaId[mapping.MediaID] = mapping.MangaID
	}
	require.Equal(t, map[int]string{21: "One Piece", 30: "Berserk"}, byMediaId)

	empty, err := database.GetMangaMappingsByProvider("mangadex")
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestDeleteMangaMapping(t *testing.T) {
	database := newMangaTestDatabase(t)

	require.NoError(t, database.InsertMangaMapping("local-manga", 21, "One Piece"))
	require.NoError(t, database.DeleteMangaMapping("local-manga", 21))

	mangaMappingCache.Clear()

	_, found := database.GetMangaMapping("local-manga", 21)
	require.False(t, found)
}
