package scanner

import (
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatcherNestedSeriesFolders covers libraries that group several distinct entries under one
// franchise folder. The franchise folder's name is often an exact AniList title, which used to let
// it outscore the subfolder naming the entry a file actually belongs to.
func TestMatcherNestedSeriesFolders(t *testing.T) {
	const libraryDir = "E:/Anime"

	titled := func(id int, romaji, english string) *anime.NormalizedMedia {
		return &anime.NormalizedMedia{
			ID: id,
			Title: &anime.NormalizedMediaTitle{
				Romaji:        new(romaji),
				English:       new(english),
				UserPreferred: new(english),
			},
		}
	}

	type expectation struct {
		path    string
		mediaId int
	}

	tests := []struct {
		name  string
		media []*anime.NormalizedMedia
		files []expectation
	}{
		{
			// The franchise folder names a real entry, and the subfolders name entries whose titles
			// only partially match their own folder. The subfolder has to win anyway.
			name: "subfolders win over the franchise folder",
			media: []*anime.NormalizedMedia{
				titled(100, "Jinzou Bizarre Adventure", "Jinzou Bizarre Adventure"),
				titled(200, "Jinzou Bizarre Adventure: Stardust Crusaders", "Jinzou Bizarre Adventure: Stardust Crusaders"),
				titled(300, "Jinzou Bizarre Adventure: Diamond is Unbreakable", "Jinzou Bizarre Adventure: Diamond is Unbreakable"),
			},
			files: []expectation{
				{"E:/Anime/Jinzou Bizarre Adventure/Part 3 Stardust Crusaders/Stardust Crusaders - 01.mkv", 200},
				{"E:/Anime/Jinzou Bizarre Adventure/Part 4 Diamond is Unbreakable/Diamond is Unbreakable - 01.mkv", 300},
				// Nothing names this entry but the franchise folder itself, so the franchise folder
				// is still allowed to win it
				{"E:/Anime/Jinzou Bizarre Adventure/Part 1 Phantom Blood/Phantom Blood - 01.mkv", 100},
				// The closest *usable* folder title is the franchise folder, so it is not demoted
				{"E:/Anime/Jinzou Bizarre Adventure/Extras/NCED 01.mkv", 100},
			},
		},
		{
			// Here the subfolders abbreviate the parent's title rather than naming something else,
			// so the parent is what identifies the entry and must keep its full weight.
			name: "parent title is kept when the subfolder only abbreviates it",
			media: []*anime.NormalizedMedia{
				titled(600, "One Punch Man", "One Punch Man"),
				titled(700, "One Punch Man 2nd Season", "One Punch Man 2nd Season"),
			},
			files: []expectation{
				{"E:/Anime/One Punch Man Series/One Punch Man/One Punch Man - 01.mkv", 600},
				{"E:/Anime/One Punch Man Series/One Punch Man 2nd Season/One Punch Man - 13.mkv", 700},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lfs := make([]*anime.LocalFile, 0, len(tt.files))
			for _, f := range tt.files {
				lfs = append(lfs, anime.NewLocalFile(f.path, libraryDir))
			}

			mc := NewMediaContainer(&MediaContainerOptions{AllMedia: tt.media})

			matcher := &Matcher{
				LocalFiles:     lfs,
				MediaContainer: mc,
				Logger:         util.NewLogger(),
			}

			if !assert.NoError(t, matcher.MatchLocalFilesWithMedia()) {
				return
			}

			for i, f := range tt.files {
				assert.Equalf(t, f.mediaId, lfs[i].MediaId, "wrong match for %s", f.path)
			}
		})
	}
}
