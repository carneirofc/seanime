package handlers

import (
	"seanime/internal/manga"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetBodyLimit(t *testing.T) {
	// The manga upload is the one route that has to accept a large body; the
	// default ceiling would reject any real chapter archive.
	uploadLimit := getBodyLimit("/api/v1/manga/local/upload")
	require.Greater(t, uploadLimit, manga.MaxLocalMangaUploadSize,
		"the route limit must leave room for the multipart envelope around a maximum-size archive")

	require.Equal(t, int64(2<<20), getBodyLimit("/api/v1/manga/local/scan"),
		"the sibling local-manga routes stay on the default limit")
	require.Equal(t, int64(2<<20), getBodyLimit("/api/v1/settings"))
	require.Equal(t, int64(100<<20), getBodyLimit("/api/v1/report/issue/decompress"))
}
