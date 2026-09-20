package scanner

import (
	"seanime/internal/library/anime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetFileSeasonAndPartUseClosestFolder covers nested folders that both carry a season or part
// marker. The folder closest to the file is the one that names the entry it belongs to.
func TestGetFileSeasonAndPartUseClosestFolder(t *testing.T) {
	const libraryDir = "E:/Anime"

	tests := []struct {
		path           string
		expectedSeason int
		expectedPart   int
	}{
		{
			path:           "E:/Anime/Franchise S2/Franchise S3/Franchise - 01.mkv",
			expectedSeason: 3,
			expectedPart:   -1,
		},
		{
			path:           "E:/Anime/Franchise Part 1/Franchise Part 2/Franchise - 01.mkv",
			expectedSeason: -1,
			expectedPart:   2,
		},
		// The marker only appears on the outer folder: unchanged
		{
			path:           "E:/Anime/Bungou Stray Dogs 5th Season/Bungou Stray Dogs/Bungou Stray Dogs - 61.mkv",
			expectedSeason: 5,
			expectedPart:   -1,
		},
		{
			path:           "E:/Anime/Attack on Titan Season 3/Part 2/Attack on Titan - 50.mkv",
			expectedSeason: 3,
			expectedPart:   2,
		},
		// The filename still takes precedence over every folder
		{
			path:           "E:/Anime/Franchise S2/Franchise.S03E01.mkv",
			expectedSeason: 3,
			expectedPart:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			lf := anime.NewLocalFile(tt.path, libraryDir)
			assert.Equal(t, tt.expectedSeason, getFileSeason(lf), "season")
			assert.Equal(t, tt.expectedPart, getFilePart(lf), "part")
		})
	}
}
