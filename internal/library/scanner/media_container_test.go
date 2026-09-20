package scanner

import (
	"seanime/internal/library/anime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewMediaContainerTitlelessMedia guards the container against media carrying no title at all.
// A custom source or the offline database can produce one, and the recovery above widens what
// reaches the container.
func TestNewMediaContainerTitlelessMedia(t *testing.T) {
	assert.NotPanics(t, func() {
		mc := NewMediaContainer(&MediaContainerOptions{
			AllMedia: []*anime.NormalizedMedia{
				{ID: 1},
				{ID: 2, Title: &anime.NormalizedMediaTitle{Romaji: new("Has A Title")}},
			},
		})
		assert.Len(t, mc.NormalizedMedia, 2)
	})
}
