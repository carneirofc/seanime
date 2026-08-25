package manga_providers

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestFindChapterPagesReadsNestedArchive covers the archives users already have
// on disk: pages inside a folder, numbered without zero padding. Both details
// used to break reading — the page lookup is by filename, and a plain string
// sort puts page 10 before page 2.
func TestFindChapterPagesReadsNestedArchive(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "One Piece"), 0o755))

	writeTestArchive(t, filepath.Join(root, "One Piece", "Chapter 1.cbz"), map[string]string{
		"pages/1.png":   "one",
		"pages/2.png":   "two",
		"pages/10.png":  "ten",
		"ComicInfo.xml": "<ComicInfo/>",
	})

	logger := zerolog.Nop()
	provider := NewLocal(root, &logger).(*Local)

	pages, err := provider.FindChapterPages("One Piece/Chapter 1.cbz")
	require.NoError(t, err)
	require.Len(t, pages, 3)

	contents := make([]string, 0, len(pages))
	for index, page := range pages {
		require.Equal(t, index, page.Index)

		reader, err := provider.ReadPage(page.URL)
		require.NoError(t, err, "page %d should be readable", index)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		contents = append(contents, string(data))
	}

	require.Equal(t, []string{"one", "two", "ten"}, contents)
}

// TestFindChapterPagesDisambiguatesRepeatedNames covers archives whose folders
// each restart their page numbering: addressing pages by filename means the
// duplicates have to be given distinct names, or one page hides the other.
func TestFindChapterPagesDisambiguatesRepeatedNames(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "Berserk"), 0o755))

	writeTestArchive(t, filepath.Join(root, "Berserk", "Chapter 1.cbz"), map[string]string{
		"a/001.png": "first",
		"b/001.png": "second",
	})

	logger := zerolog.Nop()
	provider := NewLocal(root, &logger).(*Local)

	pages, err := provider.FindChapterPages("Berserk/Chapter 1.cbz")
	require.NoError(t, err)
	require.Len(t, pages, 2)

	seen := make(map[string]struct{}, len(pages))
	for _, page := range pages {
		reader, err := provider.ReadPage(page.URL)
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		seen[string(data)] = struct{}{}
	}

	require.Len(t, seen, 2, "both pages should be reachable, not just the last one written")
}

func writeTestArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := writer.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
}
