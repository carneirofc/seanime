package handlers

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"seanime/internal/manga"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadLocalMangaUpload(t *testing.T) {
	t.Run("collects fields sent before the archive", func(t *testing.T) {
		body, contentType := buildUploadForm(t,
			field{name: "series", value: "One Piece"},
			field{name: "mediaId", value: "21"},
			field{name: "overwrite", value: "true"},
			field{name: "file", filename: "Chapter 1.cbz", value: "archive bytes"},
		)

		var captured *manga.LocalMangaUploadOptions
		var streamed string

		result, err := readLocalMangaUpload(newReader(t, body, contentType), func(opts *manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			data, readErr := io.ReadAll(opts.Source)
			require.NoError(t, readErr)
			streamed = string(data)
			captured = opts
			return &manga.LocalMangaUploadResult{Series: opts.Series}, nil
		})

		require.NoError(t, err)
		require.Equal(t, "One Piece", result.Series)
		require.Equal(t, "One Piece", captured.Series)
		require.Equal(t, "Chapter 1.cbz", captured.Filename)
		require.Equal(t, 21, captured.MediaId)
		require.True(t, captured.Overwrite)
		require.Equal(t, "archive bytes", streamed)
	})

	t.Run("falls back to the filename for the series", func(t *testing.T) {
		body, contentType := buildUploadForm(t,
			field{name: "file", filename: "[Group] Vinland.Saga.cbz", value: "archive"},
		)

		var captured *manga.LocalMangaUploadOptions
		_, err := readLocalMangaUpload(newReader(t, body, contentType), func(opts *manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			captured = opts
			return &manga.LocalMangaUploadResult{}, nil
		})

		require.NoError(t, err)
		require.Equal(t, "Vinland Saga", captured.Series)
	})

	t.Run("ignores an unparsable media id rather than failing", func(t *testing.T) {
		body, contentType := buildUploadForm(t,
			field{name: "mediaId", value: "not-a-number"},
			field{name: "overwrite", value: "no"},
			field{name: "unknown", value: "ignored"},
			field{name: "file", filename: "Chapter 1.cbz", value: "archive"},
		)

		var captured *manga.LocalMangaUploadOptions
		_, err := readLocalMangaUpload(newReader(t, body, contentType), func(opts *manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			captured = opts
			return &manga.LocalMangaUploadResult{}, nil
		})

		require.NoError(t, err)
		require.Zero(t, captured.MediaId)
		require.False(t, captured.Overwrite)
	})

	t.Run("rejects a form with no archive", func(t *testing.T) {
		body, contentType := buildUploadForm(t, field{name: "series", value: "One Piece"})

		_, err := readLocalMangaUpload(newReader(t, body, contentType), func(*manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			t.Fatal("store must not be called without an archive")
			return nil, nil
		})

		require.ErrorIs(t, err, errNoLocalMangaArchive)
	})

	t.Run("rejects a second archive", func(t *testing.T) {
		body, contentType := buildUploadForm(t,
			field{name: "file", filename: "Chapter 1.cbz", value: "first"},
			field{name: "file", filename: "Chapter 2.cbz", value: "second"},
		)

		calls := 0
		_, err := readLocalMangaUpload(newReader(t, body, contentType), func(*manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			calls++
			return &manga.LocalMangaUploadResult{}, nil
		})

		require.ErrorIs(t, err, errMultipleLocalMangaArchives)
		require.Equal(t, 1, calls, "the second archive must not be stored")
	})

	t.Run("surfaces the store error", func(t *testing.T) {
		body, contentType := buildUploadForm(t, field{name: "file", filename: "Chapter 1.cbz", value: "archive"})

		_, err := readLocalMangaUpload(newReader(t, body, contentType), func(*manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
			return nil, manga.ErrLocalMangaUploadNoPages
		})

		require.ErrorIs(t, err, manga.ErrLocalMangaUploadNoPages)
	})
}

type field struct {
	name     string
	filename string
	value    string
}

func buildUploadForm(t *testing.T, fields ...field) (*bytes.Buffer, string) {
	t.Helper()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	for _, f := range fields {
		var part io.Writer
		var err error
		if f.filename != "" {
			part, err = writer.CreateFormFile(f.name, f.filename)
		} else {
			part, err = writer.CreateFormField(f.name)
		}
		require.NoError(t, err)
		_, err = part.Write([]byte(f.value))
		require.NoError(t, err)
	}

	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func newReader(t *testing.T, body *bytes.Buffer, contentType string) *multipart.Reader {
	t.Helper()

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)

	return multipart.NewReader(body, params["boundary"])
}
