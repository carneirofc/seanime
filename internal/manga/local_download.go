package manga

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"seanime/internal/api/anilist"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"sort"
	"strconv"
	"strings"
)

// ErrLocalMangaChapterNotFound is returned for a chapter that is not in the
// series folder.
var ErrLocalMangaChapterNotFound = errors.New("local manga chapter not found")

// LocalMangaChapter is one chapter file of a local series, as offered to the
// client for download.
type LocalMangaChapter struct {
	// Filename is the chapter's name inside the series folder, and the handle
	// the download endpoints take.
	Filename string `json:"filename"`
	// Number is the chapter number parsed from the filename, when it has one.
	Number string `json:"number"`
	Title  string `json:"title"`
	Volume string `json:"volume"`
	Size   int64  `json:"size"`
	// Format is the on-disk format: "cbz", "zip", "cbr", "pdf" or "directory".
	Format string `json:"format"`
	// Downloadable reports whether the chapter can be served as a CBZ. False for
	// the formats Seanime cannot read, which are listed but not offered.
	Downloadable bool `json:"downloadable"`
	// HasMetadata reports whether the archive already carries a ComicInfo.xml.
	HasMetadata bool `json:"hasMetadata"`
}

// GetLocalMangaChapters lists the chapter files of a local series.
//
// This reads the folder directly rather than going through the chapter scanner:
// the scanner reports what is *readable* as a chapter, and this has to also show
// the files that are not, so the user can see what a repack would skip.
func (r *Repository) GetLocalMangaChapters(series string) (ret []*LocalMangaChapter, err error) {
	defer util.HandlePanicInModuleWithError("manga/GetLocalMangaChapters", &err)

	seriesDir, err := r.resolveLocalMangaSeriesDir(series)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(seriesDir)
	if err != nil || !info.IsDir() {
		return nil, ErrLocalMangaSeriesNotFound
	}

	entries, err := os.ReadDir(seriesDir)
	if err != nil {
		return nil, err
	}

	ret = make([]*LocalMangaChapter, 0, len(entries))
	for _, entry := range entries {
		// Partial writes are invisible until they are renamed into place.
		if strings.HasPrefix(entry.Name(), localMangaUploadStagingPrefix) || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}

		chapter := describeLocalChapter(seriesDir, entry.Name(), entry.IsDir())
		if chapter == nil {
			continue
		}
		ret = append(ret, chapter)
	}

	// Ordered by chapter number so the list reads the way the reader does, with
	// unnumbered files last rather than interleaved.
	sort.SliceStable(ret, func(i, j int) bool {
		left, right := parseChapterSortKey(ret[i].Number), parseChapterSortKey(ret[j].Number)
		if left != right {
			return left < right
		}
		return strings.ToLower(ret[i].Filename) < strings.ToLower(ret[j].Filename)
	})

	return ret, nil
}

func describeLocalChapter(seriesDir string, name string, isDir bool) *LocalMangaChapter {
	naming := manga_providers.ParseChapterNaming(name)

	ret := &LocalMangaChapter{
		Filename: name,
		Number:   naming.Number,
		Title:    naming.Title,
		Volume:   naming.Volume,
	}

	fullPath := filepath.Join(seriesDir, name)

	if isDir {
		ret.Format = "directory"
		ret.Downloadable = true
		ret.Size = dirSizeOf(fullPath)
		return ret
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".cbz":
		ret.Format = "cbz"
		ret.Downloadable = true
	case ".zip":
		ret.Format = "zip"
		ret.Downloadable = true
	case ".cbr":
		ret.Format = "cbr"
	case ".pdf":
		ret.Format = "pdf"
	default:
		return nil
	}

	if info, err := os.Stat(fullPath); err == nil {
		ret.Size = info.Size()
	}
	if ret.Downloadable {
		ret.HasMetadata = archiveHasComicInfo(fullPath)
	}

	return ret
}

func archiveHasComicInfo(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name == chapter_downloader.ComicInfoFilename {
			return true
		}
	}
	return false
}

// parseChapterSortKey turns a chapter number into something sortable, pushing
// unnumbered chapters to the end instead of treating them as chapter zero.
func parseChapterSortKey(number string) float64 {
	if number == "" {
		return 1 << 30
	}

	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 1 << 30
	}
	return value
}

// CheckLocalMangaChapterDownload reports whether a chapter can be served,
// returning the same errors the download itself would.
//
// A download is written straight to the response, so once the first byte is out
// the status code is fixed. Callers use this to fail with a proper status before
// committing to a 200.
func (r *Repository) CheckLocalMangaChapterDownload(series string, chapter string) (err error) {
	defer util.HandlePanicInModuleWithError("manga/CheckLocalMangaChapterDownload", &err)

	chapterPath, _, err := r.resolveLocalMangaChapter(series, chapter, nil)
	if err != nil {
		return err
	}

	info, err := os.Stat(chapterPath)
	if err != nil {
		return ErrLocalMangaChapterNotFound
	}

	if info.IsDir() {
		if !hasPageImages(chapterPath) {
			return ErrLocalMangaUploadNoPages
		}
		return nil
	}

	reader, err := zip.OpenReader(chapterPath)
	if err != nil {
		return ErrLocalMangaUploadNotAnArchive
	}
	defer reader.Close()

	_, err = planLocalMangaChapters(&reader.Reader, filepath.Base(chapterPath))
	return err
}

func hasPageImages(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && isArchivePageName(entry.Name()) {
			return true
		}
	}
	return false
}

// WriteLocalMangaChapterCBZ streams one chapter of a local series as a proper
// CBZ, normalizing it on the way out if it is not already one.
//
// Normalizing during the download is what lets an untouched library be exported
// without first rewriting it on disk: a loose .zip or a folder of images comes
// out as a CBZ with metadata, and the user's own files are left alone.
func (r *Repository) WriteLocalMangaChapterCBZ(
	w io.Writer,
	series string,
	chapter string,
	collection *anilist.MangaCollection,
) (err error) {
	defer util.HandlePanicInModuleWithError("manga/WriteLocalMangaChapterCBZ", &err)

	chapterPath, metadata, err := r.resolveLocalMangaChapter(series, chapter, collection)
	if err != nil {
		return err
	}

	return writeLocalChapterCBZTo(w, chapterPath, metadata)
}

// resolveLocalMangaChapter validates a series/chapter pair and returns the
// chapter's path along with the metadata its archive should carry.
func (r *Repository) resolveLocalMangaChapter(
	series string,
	chapter string,
	collection *anilist.MangaCollection,
) (string, *localMangaSeriesMetadata, error) {
	seriesDir, err := r.resolveLocalMangaSeriesDir(series)
	if err != nil {
		return "", nil, err
	}

	// The chapter name is client-supplied too, so it gets the same treatment as
	// the series name: a single, plain path element or nothing.
	sanitized, err := SanitizeLocalMangaName(chapter)
	if err != nil {
		return "", nil, err
	}

	chapterPath := filepath.Join(seriesDir, sanitized)
	if !util.IsSubdirectory(seriesDir, chapterPath) {
		return "", nil, ErrInvalidLocalMangaName
	}

	if _, statErr := os.Stat(chapterPath); statErr != nil {
		return "", nil, ErrLocalMangaChapterNotFound
	}

	metadata := r.resolveLocalMangaSeriesMetadata(filepath.Base(seriesDir), collection)
	return chapterPath, metadata, nil
}

// writeLocalChapterCBZTo writes a chapter — archive or folder of images — to w
// as a normalized CBZ.
func writeLocalChapterCBZTo(w io.Writer, path string, metadata *localMangaSeriesMetadata) error {
	info, err := os.Stat(path)
	if err != nil {
		return ErrLocalMangaChapterNotFound
	}

	if info.IsDir() {
		return writeImageDirAsCBZ(w, path, metadata)
	}

	reader, err := zip.OpenReader(path)
	if err != nil {
		return ErrLocalMangaUploadNotAnArchive
	}
	defer reader.Close()

	chapters, err := planLocalMangaChapters(&reader.Reader, filepath.Base(path))
	if err != nil {
		return err
	}

	// A chapter file that turns out to hold several chapters is flattened back
	// into one archive here; splitting it is the repack's job, not the
	// download's, and the user asked for this one file.
	pages := make([]*zip.File, 0)
	for _, chapter := range chapters {
		pages = append(pages, chapter.pages...)
	}

	return writeCBZStream(w, filepath.Base(path), pages, metadata)
}

// LocalMangaChapterDownloadName is the filename a downloaded chapter is offered
// under: always .cbz, whatever it is stored as.
func LocalMangaChapterDownloadName(chapter string) string {
	base := strings.TrimSuffix(chapter, filepath.Ext(chapter))
	if base == "" {
		base = "chapter"
	}
	return base + ".cbz"
}

// WriteLocalMangaSeriesArchive streams every downloadable chapter of a series as
// a zip of CBZ files, and returns how many it wrote.
func (r *Repository) WriteLocalMangaSeriesArchive(
	w io.Writer,
	series string,
	collection *anilist.MangaCollection,
) (count int, err error) {
	defer util.HandlePanicInModuleWithError("manga/WriteLocalMangaSeriesArchive", &err)

	seriesDir, err := r.resolveLocalMangaSeriesDir(series)
	if err != nil {
		return 0, err
	}

	chapters, err := r.GetLocalMangaChapters(series)
	if err != nil {
		return 0, err
	}

	metadata := r.resolveLocalMangaSeriesMetadata(filepath.Base(seriesDir), collection)

	zw := zip.NewWriter(w)
	for _, chapter := range chapters {
		if !chapter.Downloadable {
			continue
		}

		// Each entry is a CBZ, which is already a compressed archive; Store
		// avoids compressing it a second time for nothing.
		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:   LocalMangaChapterDownloadName(chapter.Filename),
			Method: zip.Store,
		})
		if err != nil {
			return count, err
		}

		if err := writeLocalChapterCBZTo(entry, filepath.Join(seriesDir, chapter.Filename), metadata); err != nil {
			// One unreadable chapter must not abort an otherwise good export.
			r.logger.Warn().Err(err).Str("chapter", chapter.Filename).Msg("manga: Skipping chapter in series archive")
			continue
		}
		count++
	}

	if err := zw.Close(); err != nil {
		return count, err
	}

	return count, nil
}

// writeImageDirAsCBZ packages a folder of loose page images as a CBZ.
func writeImageDirAsCBZ(w io.Writer, dir string, metadata *localMangaSeriesMetadata) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isArchivePageName(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return ErrLocalMangaUploadNoPages
	}
	sort.SliceStable(names, func(i, j int) bool { return comparePageNames(names[i], names[j]) < 0 })

	zw := zip.NewWriter(w)
	pageInfos := make([]chapter_downloader.ComicInfoPage, 0, len(names))

	for position, name := range names {
		entryName := formatPageEntryName(position, name)
		entry, err := zw.CreateHeader(&zip.FileHeader{Name: entryName, Method: zip.Store})
		if err != nil {
			return err
		}

		src, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		pageInfo, cErr := copyPageReader(entry, src)
		_ = src.Close()
		if cErr != nil {
			return cErr
		}
		pageInfo.Image = position
		pageInfos = append(pageInfos, pageInfo)
	}

	return finishCBZStream(zw, filepath.Base(dir), pageInfos, metadata)
}

// writeCBZStream writes pages from an archive into w as a CBZ. It is the
// streaming counterpart of writeNormalizedCBZ, which writes to a file it can
// rename into place.
func writeCBZStream(w io.Writer, chapterName string, pages []*zip.File, metadata *localMangaSeriesMetadata) error {
	if len(pages) == 0 {
		return ErrLocalMangaUploadNoPages
	}

	zw := zip.NewWriter(w)
	pageInfos := make([]chapter_downloader.ComicInfoPage, 0, len(pages))

	for position, page := range pages {
		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:   formatPageEntryName(position, page.Name),
			Method: zip.Store,
		})
		if err != nil {
			return err
		}

		pageInfo, cErr := copyPageEntry(entry, page)
		if cErr != nil {
			return cErr
		}
		pageInfo.Image = position
		pageInfos = append(pageInfos, pageInfo)
	}

	return finishCBZStream(zw, chapterName, pageInfos, metadata)
}

// finishCBZStream appends the metadata document and closes the archive.
func finishCBZStream(
	zw *zip.Writer,
	chapterName string,
	pages []chapter_downloader.ComicInfoPage,
	metadata *localMangaSeriesMetadata,
) error {
	if metadata != nil {
		info := buildLocalChapterComicInfo(metadata, LocalMangaChapterDownloadName(chapterName), pages)
		data, err := info.Marshal()
		if err != nil {
			return err
		}
		entry, err := zw.Create(chapter_downloader.ComicInfoFilename)
		if err != nil {
			return err
		}
		if _, err := entry.Write(data); err != nil {
			return err
		}
	}

	return zw.Close()
}

// formatPageEntryName is the name a page takes inside a generated CBZ: its
// zero-padded position, keeping the original extension so readers still know the
// image format.
func formatPageEntryName(position int, sourceName string) string {
	return fmt.Sprintf("%03d%s", position+1, strings.ToLower(path.Ext(sourceName)))
}

// dirSizeOf sums the size of the files in a directory tree.
func dirSizeOf(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // an unreadable subtree just doesn't count toward the total
		}
		if info, infoErr := entry.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}
