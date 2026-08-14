package chapter_downloader

import (
	"archive/zip"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// stagingDirPrefix marks in-progress chapter downloads. The dot prefix keeps
// staging directories invisible to ScanDownloadDir and the migration routine.
const stagingDirPrefix = ".downloading_"

func FormatSeriesDirName(provider string, mediaId int) string {
	return fmt.Sprintf("%s_%d", provider, mediaId)
}

// ParseSeriesDirName parses a series directory name, e.g. "comick_1234".
// Callers must try ParseChapterDirName first: a legacy chapter directory name
// ("comick_1234_chapterid_13") would otherwise also satisfy this parser.
func ParseSeriesDirName(name string) (provider string, mediaId int, ok bool) {
	idx := strings.LastIndex(name, "_")
	if idx <= 0 || idx == len(name)-1 {
		return "", 0, false
	}
	mediaId, err := strconv.Atoi(name[idx+1:])
	if err != nil {
		return "", 0, false
	}
	return name[:idx], mediaId, true
}

// FormatChapterFileName formats a chapter CBZ filename, e.g. "13.5_chapter$UNDERSCORE$id.cbz".
// The chapter number comes first so files sort naturally in external readers.
func FormatChapterFileName(chapterId string, chapterNumber string) string {
	return fmt.Sprintf("%s_%s.cbz", chapterNumber, EscapeChapterID(chapterId))
}

// ParseChapterFileName parses a chapter CBZ filename back into its components.
func ParseChapterFileName(name string) (chapterId string, chapterNumber string, ok bool) {
	if !strings.HasSuffix(name, ".cbz") {
		return "", "", false
	}
	name = strings.TrimSuffix(name, ".cbz")
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return UnescapeChapterID(parts[1]), parts[0], true
}

func (cd *Downloader) getChapterCBZPath(id DownloadID) string {
	return filepath.Join(cd.downloadDir, FormatSeriesDirName(id.Provider, id.MediaId), FormatChapterFileName(id.ChapterId, id.ChapterNumber))
}

func (cd *Downloader) getChapterStagingDir(id DownloadID) string {
	return filepath.Join(cd.downloadDir, stagingDirPrefix+FormatChapterDirName(id.Provider, id.MediaId, id.ChapterId, id.ChapterNumber))
}

func isImageFilename(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tiff", ".avif":
		return true
	}
	return false
}

// writeCBZ writes the chapter pages found in srcDir into a CBZ archive at destPath.
// Pages are stored in ascending registry index order under 3-digit zero-padded
// names (001.png, 002.png, ...) followed by ComicInfo.xml.
// The write is atomic: content goes to destPath+".tmp" which is renamed into
// place only after a successful sync, so readers never observe a partial archive.
func writeCBZ(destPath string, srcDir string, registry Registry, info *ComicInfo) (err error) {
	tmpPath := destPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	zw := zip.NewWriter(f)

	indexes := make([]int, 0, len(registry))
	for idx := range registry {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)

	for position, idx := range indexes {
		pageInfo := registry[idx]
		entryName := fmt.Sprintf("%03d%s", position+1, strings.ToLower(filepath.Ext(pageInfo.Filename)))

		// Images are already compressed formats; Store avoids pointless deflate work.
		w, wErr := zw.CreateHeader(&zip.FileHeader{
			Name:   entryName,
			Method: zip.Store,
		})
		if wErr != nil {
			err = wErr
			return err
		}

		src, sErr := os.Open(filepath.Join(srcDir, pageInfo.Filename))
		if sErr != nil {
			err = sErr
			return err
		}
		_, cErr := io.Copy(w, src)
		_ = src.Close()
		if cErr != nil {
			err = cErr
			return err
		}
	}

	if info != nil {
		data, mErr := info.marshal()
		if mErr != nil {
			err = mErr
			return err
		}
		w, wErr := zw.Create(comicInfoFilename)
		if wErr != nil {
			err = wErr
			return err
		}
		if _, wErr = w.Write(data); wErr != nil {
			err = wErr
			return err
		}
	}

	if err = zw.Close(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, destPath)
}

// CBZEntry describes one page image inside a CBZ archive.
type CBZEntry struct {
	// Name of the zip entry.
	Name string
	// Width and Height of the image. Sourced from ComicInfo.xml when present,
	// otherwise decoded from the image header.
	Width  int
	Height int
}

// ReadCBZ lists the page images of a CBZ archive in reading order along with
// the parsed ComicInfo document (nil if the archive has none, e.g. a
// hand-made CBZ dropped into the download directory).
// Image bytes are not buffered; dimensions fall back to decoding only the
// image header when ComicInfo.xml is absent or incomplete.
func ReadCBZ(path string) (entries []CBZEntry, info *ComicInfo, err error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()

	imageFiles := make([]*zip.File, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if f.Name == comicInfoFilename {
			rc, oErr := f.Open()
			if oErr != nil {
				continue
			}
			info, _ = parseComicInfo(rc)
			_ = rc.Close()
			continue
		}
		if isImageFilename(f.Name) {
			imageFiles = append(imageFiles, f)
		}
	}

	sort.Slice(imageFiles, func(i, j int) bool { return imageFiles[i].Name < imageFiles[j].Name })

	dimsByPosition := make(map[int]ComicInfoPage)
	if info != nil && info.Pages != nil {
		for _, page := range info.Pages.Pages {
			dimsByPosition[page.Image] = page
		}
	}

	entries = make([]CBZEntry, 0, len(imageFiles))
	for position, f := range imageFiles {
		entry := CBZEntry{Name: f.Name}
		if page, ok := dimsByPosition[position]; ok && page.ImageWidth > 0 && page.ImageHeight > 0 {
			entry.Width = page.ImageWidth
			entry.Height = page.ImageHeight
		} else if rc, oErr := f.Open(); oErr == nil {
			if config, _, dErr := image.DecodeConfig(rc); dErr == nil {
				entry.Width = config.Width
				entry.Height = config.Height
			}
			_ = rc.Close()
		}
		entries = append(entries, entry)
	}

	return entries, info, nil
}

// ScanDownloadDir walks the download directory and returns every downloaded
// chapter, mapped to its slash-separated path relative to downloadDir.
// Both layouts are recognized:
//   - new: {provider}_{mediaId}/{chapterNumber}_{escapedChapterId}.cbz
//   - legacy: {provider}_{mediaId}_{escapedChapterId}_{chapterNumber}/ (loose images)
//
// Dot-prefixed entries (staging dirs) and *.tmp files are skipped.
func ScanDownloadDir(downloadDir string) map[DownloadID]string {
	ret := make(map[DownloadID]string)

	topEntries, err := os.ReadDir(downloadDir)
	if err != nil {
		return ret
	}

	for _, topEntry := range topEntries {
		name := topEntry.Name()
		if !topEntry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}

		// Legacy chapter directory (must be tried before ParseSeriesDirName,
		// which would also match a 4-part legacy name).
		if id, ok := ParseChapterDirName(name); ok {
			ret[id] = name
			continue
		}

		provider, mediaId, ok := ParseSeriesDirName(name)
		if !ok {
			continue
		}

		chapterFiles, err := os.ReadDir(filepath.Join(downloadDir, name))
		if err != nil {
			continue
		}
		for _, chapterFile := range chapterFiles {
			fileName := chapterFile.Name()
			if chapterFile.IsDir() || strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
				continue
			}
			chapterId, chapterNumber, ok := ParseChapterFileName(fileName)
			if !ok {
				continue
			}
			ret[DownloadID{
				Provider:      provider,
				MediaId:       mediaId,
				ChapterId:     chapterId,
				ChapterNumber: chapterNumber,
			}] = name + "/" + fileName
		}
	}

	return ret
}
