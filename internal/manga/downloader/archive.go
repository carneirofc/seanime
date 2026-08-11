package chapter_downloader

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteLegacyDirAsCBZ zips the image files of a legacy loose-image chapter
// directory into w as a CBZ archive (images in name order, Store method).
// Used to hand out chapters that could not be migrated to the CBZ layout.
func WriteLegacyDirAsCBZ(w io.Writer, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	imageNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isImageFilename(entry.Name()) {
			continue
		}
		imageNames = append(imageNames, entry.Name())
	}
	if len(imageNames) == 0 {
		return fmt.Errorf("no page images found in %s", dir)
	}
	sort.Strings(imageNames)

	zw := zip.NewWriter(w)
	for _, name := range imageNames {
		// Images are already compressed formats; Store avoids pointless deflate work.
		ew, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return err
		}
		src, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		_, cErr := io.Copy(ew, src)
		_ = src.Close()
		if cErr != nil {
			return cErr
		}
	}

	return zw.Close()
}

// WriteMediaArchive streams a zip archive containing every downloaded chapter
// of the given provider/media pair into w. Chapters stored as CBZ archives are
// copied verbatim under their on-disk filename; legacy loose-image directories
// are wrapped into CBZ entries on the fly.
// Returns the number of chapters written.
func WriteMediaArchive(w io.Writer, downloadDir string, provider string, mediaId int) (int, error) {
	type chapterEntry struct {
		entryName string
		relPath   string
	}

	chapters := make([]chapterEntry, 0)
	for id, relPath := range ScanDownloadDir(downloadDir) {
		if id.Provider != provider || id.MediaId != mediaId {
			continue
		}
		entryName := filepath.Base(relPath)
		if !strings.HasSuffix(relPath, ".cbz") {
			entryName = FormatChapterFileName(id.ChapterId, id.ChapterNumber)
		}
		chapters = append(chapters, chapterEntry{entryName: entryName, relPath: relPath})
	}
	if len(chapters) == 0 {
		return 0, fmt.Errorf("no downloaded chapters found for %s", FormatSeriesDirName(provider, mediaId))
	}
	sort.Slice(chapters, func(i, j int) bool { return chapters[i].entryName < chapters[j].entryName })

	zw := zip.NewWriter(w)
	for _, chapter := range chapters {
		fullPath := filepath.Join(downloadDir, filepath.FromSlash(chapter.relPath))

		// CBZ entries are zip archives themselves; Store avoids double compression.
		ew, err := zw.CreateHeader(&zip.FileHeader{Name: chapter.entryName, Method: zip.Store})
		if err != nil {
			return 0, err
		}

		if strings.HasSuffix(chapter.relPath, ".cbz") {
			src, err := os.Open(fullPath)
			if err != nil {
				return 0, err
			}
			_, cErr := io.Copy(ew, src)
			_ = src.Close()
			if cErr != nil {
				return 0, cErr
			}
			continue
		}

		if err := WriteLegacyDirAsCBZ(ew, fullPath); err != nil {
			return 0, err
		}
	}

	if err := zw.Close(); err != nil {
		return 0, err
	}
	return len(chapters), nil
}
