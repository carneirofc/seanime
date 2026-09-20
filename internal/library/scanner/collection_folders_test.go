package scanner

import (
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCollectionFolders(t *testing.T) {
	const libraryDir = "E:/Anime"

	tests := []struct {
		name string
		// paths are the files being matched
		paths []string
		// extraPaths are files known to exist but not being matched (locked, ignored, shelved)
		extraPaths []string
		// collections must be flagged as grouping several distinct entries
		collections []string
		// notCollections must not be flagged
		notCollections []string
	}{
		{
			name: "franchise folder with distinctly named subfolders",
			paths: []string{
				"E:/Anime/Monogatari Series/Bakemonogatari/Bakemonogatari - 01.mkv",
				"E:/Anime/Monogatari Series/Nisemonogatari/Nisemonogatari - 01.mkv",
				"E:/Anime/Monogatari Series/Kizumonogatari/Kizumonogatari I - Tekketsu-hen.mkv",
			},
			collections:    []string{"E:/Anime/Monogatari Series"},
			notCollections: []string{"E:/Anime/Monogatari Series/Bakemonogatari"},
		},
		{
			name: "franchise folder whose subfolders are parts",
			paths: []string{
				"E:/Anime/JoJo's Bizarre Adventure/Part 1 Phantom Blood/Phantom Blood - 01.mkv",
				"E:/Anime/JoJo's Bizarre Adventure/Part 3 Stardust Crusaders/Stardust Crusaders - 01.mkv",
			},
			collections: []string{"E:/Anime/JoJo's Bizarre Adventure"},
		},
		{
			name: "series folder split into seasons is not a collection",
			paths: []string{
				"E:/Anime/Sousou no Frieren/Season 1/Frieren - 01.mkv",
				"E:/Anime/Sousou no Frieren/Specials/Frieren - NCED.mkv",
			},
			notCollections: []string{"E:/Anime/Sousou no Frieren"},
		},
		{
			name: "a single named subfolder plus extras is not a collection",
			paths: []string{
				"E:/Anime/One Punch Man Series (+OVA)/Extras/One Punch Man - S01OVA01.mkv",
				"E:/Anime/One Punch Man Series (+OVA)/One Punch Man/One Punch Man - 01.mkv",
			},
			notCollections: []string{"E:/Anime/One Punch Man Series (+OVA)"},
		},
		{
			name: "a release folder under a series folder is not a collection",
			paths: []string{
				"E:/Anime/Re Zero kara Hajimeru Isekai Seikatsu 3rd Season (Batch + OVAs)/ReZero S03 1080p Dual Audio WEBRip/S03E01.mkv",
			},
			notCollections: []string{
				"E:/Anime/Re Zero kara Hajimeru Isekai Seikatsu 3rd Season (Batch + OVAs)",
			},
		},
		{
			name: "files directly in a series folder",
			paths: []string{
				"E:/Anime/Sousou no Frieren/Frieren - 01.mkv",
				"E:/Anime/Sousou no Frieren/Frieren - 02.mkv",
			},
			notCollections: []string{"E:/Anime/Sousou no Frieren", "E:/Anime"},
		},
		{
			name: "a skipped sibling folder still reveals the collection",
			paths: []string{
				"E:/Anime/Slayers/Slayers Next/Slayers Next - 01.mkv",
			},
			extraPaths: []string{
				"E:/Anime/Slayers/Slayers Try/Slayers Try - 01.mkv",
			},
			collections: []string{"E:/Anime/Slayers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lfs := make([]*anime.LocalFile, 0, len(tt.paths))
			for _, p := range tt.paths {
				lfs = append(lfs, anime.NewLocalFile(p, libraryDir))
			}

			got := detectCollectionFolders(lfs, tt.extraPaths)

			for _, dir := range tt.collections {
				assert.Containsf(t, got, util.NormalizePath(dir), "%q should be a collection folder, got %v", dir, got)
			}
			for _, dir := range tt.notCollections {
				assert.NotContainsf(t, got, util.NormalizePath(dir), "%q should not be a collection folder", dir)
			}
		})
	}
}

func TestFolderPathAt(t *testing.T) {
	lf := anime.NewLocalFile("E:/Anime/Monogatari Series/Bakemonogatari/Bakemonogatari - 01.mkv", "E:/Anime")

	if !assert.Len(t, lf.ParsedFolderData, 2) {
		return
	}

	outer, ok := folderPathAt(lf, 0)
	if assert.True(t, ok) {
		assert.Equal(t, util.NormalizePath("E:/Anime/Monogatari Series"), outer)
	}

	inner, ok := folderPathAt(lf, 1)
	if assert.True(t, ok) {
		assert.Equal(t, util.NormalizePath("E:/Anime/Monogatari Series/Bakemonogatari"), inner)
	}

	_, ok = folderPathAt(lf, 2)
	assert.False(t, ok, "an out of range index should not resolve")
}

func TestTitlesAreRelated(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		// An abbreviation of the parent folder's title: the parent is what identifies the entry
		{"Re Zero kara Hajimeru Isekai Seikatsu", "ReZero", true},
		// The subfolder extends the parent's title
		{"One Punch Man Series", "One Punch Man", true},
		{"Monogatari Series", "Monogatari Series Second Season", true},
		// Genuinely different entries under one franchise folder
		{"JoJo's Bizarre Adventure", "Stardust Crusaders", false},
		{"Monogatari Series", "Bakemonogatari", false},
		{"Slayers", "Cowboy Bebop", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+" / "+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.expected, titlesAreRelated(NormalizeTitle(tt.a), NormalizeTitle(tt.b)))
		})
	}
}
