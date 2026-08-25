package manga

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // Register GIF so page dimensions can be read
	"io"
	"os"
	"path"
	"path/filepath"
	"seanime/internal/api/anilist"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"slices"
	"sort"
	"strings"
)

const (
	// MaxLocalMangaUploadSize caps the archive a client may push. Manga volumes
	// get large, but an upload this size is already far past anything a single
	// series archive needs.
	MaxLocalMangaUploadSize int64 = 2 << 30 // 2 GiB

	// maxLocalMangaUploadEntries and maxLocalMangaPageSize bound what the archive
	// may expand into. The upload is re-packaged rather than extracted, but it is
	// still read end to end, so a decompression bomb has to be stopped here.
	maxLocalMangaUploadEntries           = 20000
	maxLocalMangaPageSize         uint64 = 256 << 20  // 256 MiB
	maxLocalMangaUploadUnpacked   uint64 = 8 << 30    // 8 GiB
	localMangaUploadStagingPrefix        = ".upload_" //nolint:gosec // not a credential
)

var (
	ErrLocalMangaUploadNotAnArchive = errors.New("the uploaded file is not a readable zip/cbz archive")
	ErrLocalMangaUploadNoPages      = errors.New("the archive contains no page images")
	ErrLocalMangaUploadUnsafeEntry  = errors.New("the archive contains an entry with an unsafe path")
	ErrLocalMangaUploadTooLarge     = errors.New("the archive expands to more data than allowed")
	ErrLocalMangaChapterExists      = errors.New("a chapter with this name already exists")
)

type (
	// LocalMangaUploadOptions describes one archive upload into the local library.
	LocalMangaUploadOptions struct {
		// Series is the target series directory name, relative to the library root.
		Series string
		// Filename is the uploaded file's original name. It names the stored
		// chapter when the archive holds a single one.
		Filename string
		// Source streams the archive bytes.
		Source io.Reader
		// Overwrite replaces chapters that already exist instead of failing.
		Overwrite bool
		// MediaId, when greater than zero, maps the series to that AniList entry
		// once the upload lands.
		MediaId int
		// Collection supplies the AniList metadata written into the archives.
		// Optional: without it the chapters still get a ComicInfo document, just
		// one built from the folder name alone.
		Collection *anilist.MangaCollection
	}

	// LocalMangaUploadResult reports what an upload produced.
	LocalMangaUploadResult struct {
		Series string `json:"series"`
		// Chapters are the CBZ filenames written into the series directory.
		Chapters []string `json:"chapters"`
		// PageCount is the total number of page images stored.
		PageCount int  `json:"pageCount"`
		MediaId   *int `json:"mediaId"`
	}
)

// UploadLocalMangaArchive stores an uploaded zip/cbz archive as one or more
// chapters of a local manga series.
//
// The archive is re-packaged rather than copied: every output is a flat CBZ with
// its pages renamed in reading order. That is what makes an arbitrary upload
// readable — the reader addresses pages by filename, so nested or oddly ordered
// entries would otherwise load as an empty or shuffled chapter — and it means
// nothing from the archive is ever used as a path on disk.
//
// An archive whose images sit in sibling folders is split into one chapter per
// folder, which is how full-series downloads are usually packaged.
func (r *Repository) UploadLocalMangaArchive(opts *LocalMangaUploadOptions) (ret *LocalMangaUploadResult, err error) {
	defer util.HandlePanicInModuleWithError("manga/UploadLocalMangaArchive", &err)

	if opts == nil || opts.Source == nil {
		return nil, errors.New("no archive was uploaded")
	}

	seriesDir, err := r.resolveLocalMangaSeriesDir(opts.Series)
	if err != nil {
		return nil, err
	}

	root, _ := r.LocalMangaDirectory()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}

	// Staging next to the destination keeps the final rename on one filesystem.
	// The dot prefix keeps the partial file out of the library listing, which
	// only reports directories.
	staged, err := os.CreateTemp(root, localMangaUploadStagingPrefix+"*.tmp")
	if err != nil {
		return nil, err
	}
	stagedPath := staged.Name()
	defer func() {
		_ = staged.Close()
		_ = os.Remove(stagedPath)
	}()

	written, err := io.Copy(staged, io.LimitReader(opts.Source, MaxLocalMangaUploadSize+1))
	if err != nil {
		return nil, err
	}
	if written > MaxLocalMangaUploadSize {
		return nil, ErrLocalMangaUploadTooLarge
	}
	if err := staged.Close(); err != nil {
		return nil, err
	}

	reader, err := zip.OpenReader(stagedPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLocalMangaUploadNotAnArchive, err)
	}
	defer reader.Close()

	chapters, err := planLocalMangaChapters(&reader.Reader, opts.Filename)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(seriesDir, 0o755); err != nil {
		return nil, err
	}

	if !opts.Overwrite {
		if conflicts := existingChapterFiles(seriesDir, chapters); len(conflicts) > 0 {
			return nil, fmt.Errorf("%w: %s", ErrLocalMangaChapterExists, strings.Join(conflicts, ", "))
		}
	}

	ret = &LocalMangaUploadResult{
		Series:   filepath.Base(seriesDir),
		Chapters: make([]string, 0, len(chapters)),
	}

	// Mapping first: the metadata written into the archives is drawn from the
	// entry the series maps to, so the mapping has to exist before the write.
	if opts.MediaId > 0 {
		if err := r.MapLocalMangaSeries(opts.MediaId, ret.Series); err != nil {
			return nil, err
		}
		ret.MediaId = &opts.MediaId
	}

	metadata := r.resolveLocalMangaSeriesMetadata(ret.Series, opts.Collection)

	for _, chapter := range chapters {
		if err := writeNormalizedCBZ(filepath.Join(seriesDir, chapter.filename), chapter.pages, metadata); err != nil {
			return nil, err
		}
		ret.Chapters = append(ret.Chapters, chapter.filename)
		ret.PageCount += len(chapter.pages)
	}

	r.invalidateLocalMangaSeriesCache(ret.Series)

	r.logger.Info().
		Str("series", ret.Series).
		Int("chapters", len(ret.Chapters)).
		Int("pages", ret.PageCount).
		Msg("manga: Stored uploaded local manga archive")

	return ret, nil
}

// plannedChapter is one CBZ to be written, with the archive entries that make up
// its pages already in reading order.
type plannedChapter struct {
	filename string
	pages    []*zip.File
}

// planLocalMangaChapters decides how an archive maps onto chapters.
//
// Images are grouped by the folder they sit in. A wrapper folder shared by
// everything is ignored, so "One Piece/Chapter 1/001.jpg" groups the same way
// "Chapter 1/001.jpg" does.
func planLocalMangaChapters(reader *zip.Reader, uploadName string) ([]*plannedChapter, error) {
	if len(reader.File) > maxLocalMangaUploadEntries {
		return nil, ErrLocalMangaUploadTooLarge
	}

	groups := make(map[string][]*zip.File)
	var unpacked uint64

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		name, ok := normalizeArchiveEntryName(file.Name)
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrLocalMangaUploadUnsafeEntry, file.Name)
		}

		if !isArchivePageName(name) {
			// ComicInfo.xml, thumbnails and other bookkeeping files are dropped:
			// the CBZ is rebuilt from the pages alone.
			continue
		}

		if file.UncompressedSize64 > maxLocalMangaPageSize {
			return nil, ErrLocalMangaUploadTooLarge
		}
		unpacked += file.UncompressedSize64
		if unpacked > maxLocalMangaUploadUnpacked {
			return nil, ErrLocalMangaUploadTooLarge
		}

		dir := path.Dir(name)
		groups[dir] = append(groups[dir], file)
	}

	if len(groups) == 0 {
		return nil, ErrLocalMangaUploadNoPages
	}

	dirs := make([]string, 0, len(groups))
	for dir := range groups {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	prefix := commonArchivePrefix(dirs)

	// Everything sits in one folder: the whole archive is a single chapter, named
	// after the uploaded file so the user recognises it in the chapter list.
	if len(dirs) == 1 {
		return []*plannedChapter{{
			filename: chapterFilenameFromUpload(uploadName, dirs[0]),
			pages:    sortArchivePages(groups[dirs[0]]),
		}}, nil
	}

	ret := make([]*plannedChapter, 0, len(dirs))
	used := make(map[string]struct{}, len(dirs))

	for _, dir := range dirs {
		label := strings.TrimPrefix(strings.TrimPrefix(dir, prefix), "/")
		if label == "" || label == "." {
			// Pages loose at the archive root alongside chapter folders — keep them
			// as their own chapter rather than dropping them.
			label = strings.TrimSuffix(baseUploadName(uploadName), filepath.Ext(baseUploadName(uploadName)))
		}
		// A nested layout ("Vol 1/Chapter 3") reads better collapsed to its
		// deepest element, which is the part that names the chapter.
		label = strings.ReplaceAll(label, "/", " - ")

		filename := uniqueChapterFilename(label, used)
		ret = append(ret, &plannedChapter{filename: filename, pages: sortArchivePages(groups[dir])})
	}

	return ret, nil
}

// normalizeArchiveEntryName converts a zip entry name to a clean relative slash
// path, reporting false for anything that tries to escape.
//
// Nothing from the archive reaches the filesystem, but an entry that attempts
// traversal says the archive is hostile or corrupt, and neither is worth storing.
func normalizeArchiveEntryName(name string) (string, bool) {
	cleaned := strings.ReplaceAll(name, `\`, "/")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" || strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, "\x00") {
		return "", false
	}
	// A Windows drive letter ("C:/x") is absolute too, and path.IsAbs misses it.
	if len(cleaned) > 1 && cleaned[1] == ':' {
		return "", false
	}

	if slices.Contains(strings.Split(cleaned, "/"), "..") {
		return "", false
	}

	cleaned = path.Clean(cleaned)
	if cleaned == "." || cleaned == ".." {
		return "", false
	}

	return cleaned, true
}

var archivePageExtensions = []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tiff", ".tif", ".avif", ".jxl"}

func isArchivePageName(name string) bool {
	base := path.Base(name)
	// Resource forks that macOS puts in zips look like images but are not.
	if strings.HasPrefix(base, "._") || strings.HasPrefix(name, "__MACOSX/") {
		return false
	}
	return slices.Contains(archivePageExtensions, strings.ToLower(path.Ext(name)))
}

// commonArchivePrefix returns the folder every group shares, or "" when they
// share none.
func commonArchivePrefix(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	if len(dirs) == 1 {
		return dirs[0]
	}

	prefix := strings.Split(dirs[0], "/")
	for _, dir := range dirs[1:] {
		elements := strings.Split(dir, "/")
		count := 0
		for count < len(prefix) && count < len(elements) && prefix[count] == elements[count] {
			count++
		}
		prefix = prefix[:count]
		if len(prefix) == 0 {
			return ""
		}
	}

	joined := strings.Join(prefix, "/")
	if joined == "." {
		return ""
	}
	return joined
}

// sortArchivePages orders pages the way a reader expects: by filename, with
// embedded numbers compared numerically so "10.jpg" follows "9.jpg".
func sortArchivePages(files []*zip.File) []*zip.File {
	ret := slices.Clone(files)
	sort.SliceStable(ret, func(i, j int) bool {
		return comparePageNames(path.Base(ret[i].Name), path.Base(ret[j].Name)) < 0
	})
	return ret
}

// comparePageNames compares two page filenames naturally: digit runs are
// compared as numbers so "9.jpg" sorts before "10.jpg", which plain
// lexicographic order gets backwards and readers notice immediately.
func comparePageNames(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)

	for len(a) > 0 && len(b) > 0 {
		aDigits, bDigits := leadingDigits(a), leadingDigits(b)

		if aDigits > 0 && bDigits > 0 {
			aNum := strings.TrimLeft(a[:aDigits], "0")
			bNum := strings.TrimLeft(b[:bDigits], "0")
			// Longer number (ignoring padding) is larger; same length falls back to
			// lexicographic, which for equal-length digits is numeric order.
			if len(aNum) != len(bNum) {
				return cmpInt(len(aNum), len(bNum))
			}
			if aNum != bNum {
				return strings.Compare(aNum, bNum)
			}
			a, b = a[aDigits:], b[bDigits:]
			continue
		}

		if a[0] != b[0] {
			return cmpInt(int(a[0]), int(b[0]))
		}
		a, b = a[1:], b[1:]
	}

	return cmpInt(len(a), len(b))
}

func leadingDigits(value string) int {
	count := 0
	for count < len(value) && value[count] >= '0' && value[count] <= '9' {
		count++
	}
	return count
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func containsDigit(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool { return r >= '0' && r <= '9' })
}

func baseUploadName(uploadName string) string {
	base := path.Base(strings.ReplaceAll(strings.TrimSpace(uploadName), `\`, "/"))
	if base == "" || base == "." || base == "/" {
		return "Chapter 1"
	}
	return base
}

// chapterFilenameFromUpload names a single-chapter upload, preferring the folder
// the pages came from and falling back to the uploaded filename.
func chapterFilenameFromUpload(uploadName string, dir string) string {
	base := baseUploadName(uploadName)
	label := strings.TrimSuffix(base, path.Ext(base))

	// "Chapter 12/001.jpg" uploaded as "download.zip" should still read as
	// chapter 12. Only a folder that looks like a chapter is used, though: a
	// series wrapper folder ("One Piece/001.jpg") names the series, not the
	// chapter, and the uploaded filename is the better guess there.
	if folder := path.Base(dir); folder != "." && containsDigit(folder) {
		label = folder
	}

	if sanitized, err := SanitizeLocalMangaName(label); err == nil {
		label = sanitized
	} else {
		label = "Chapter 1"
	}

	return label + ".cbz"
}

func uniqueChapterFilename(label string, used map[string]struct{}) string {
	sanitized, err := SanitizeLocalMangaName(label)
	if err != nil {
		sanitized = "Chapter"
	}

	filename := sanitized + ".cbz"
	for suffix := 2; ; suffix++ {
		if _, taken := used[strings.ToLower(filename)]; !taken {
			break
		}
		filename = fmt.Sprintf("%s (%d).cbz", sanitized, suffix)
	}

	used[strings.ToLower(filename)] = struct{}{}
	return filename
}

func existingChapterFiles(seriesDir string, chapters []*plannedChapter) []string {
	conflicts := make([]string, 0)
	for _, chapter := range chapters {
		if _, err := os.Stat(filepath.Join(seriesDir, chapter.filename)); err == nil {
			conflicts = append(conflicts, chapter.filename)
		}
	}
	return conflicts
}

// writeNormalizedCBZ writes pages into destPath as a proper CBZ: a flat archive
// of images renamed to zero-padded positions, followed by the ComicInfo.xml
// document that describes them.
//
// The metadata is built from the pages as they are written, so the recorded
// dimensions and sizes describe what actually landed in the archive rather than
// what the source claimed.
//
// The write goes to a temp file that is renamed into place only once complete,
// so a failed write never leaves a half-built chapter for the scanner to find.
func writeNormalizedCBZ(destPath string, pages []*zip.File, metadata *localMangaSeriesMetadata) (err error) {
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
	pageInfos := make([]chapter_downloader.ComicInfoPage, 0, len(pages))

	for position, page := range pages {
		// Page images are already compressed; Store avoids pointless deflate work.
		w, wErr := zw.CreateHeader(&zip.FileHeader{
			Name:   formatPageEntryName(position, page.Name),
			Method: zip.Store,
		})
		if wErr != nil {
			return wErr
		}

		pageInfo, cErr := copyPageEntry(w, page)
		if cErr != nil {
			return cErr
		}
		pageInfo.Image = position
		pageInfos = append(pageInfos, pageInfo)
	}

	if metadata != nil {
		info := buildLocalChapterComicInfo(metadata, filepath.Base(destPath), pageInfos)
		data, mErr := info.Marshal()
		if mErr != nil {
			return mErr
		}
		w, wErr := zw.Create(chapter_downloader.ComicInfoFilename)
		if wErr != nil {
			return wErr
		}
		if _, wErr = w.Write(data); wErr != nil {
			return wErr
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

// copyPageEntry streams one page of an archive into another, measuring it on the
// way past.
func copyPageEntry(w io.Writer, page *zip.File) (chapter_downloader.ComicInfoPage, error) {
	src, err := page.Open()
	if err != nil {
		return chapter_downloader.ComicInfoPage{}, err
	}
	defer src.Close()

	return copyPageReader(w, src)
}

// copyPageReader streams one page into the archive, measuring it on the way past.
//
// The image header is decoded from the same pass that copies the bytes — a tee
// hands the leading bytes to the decoder and to the archive at once — so
// recording page dimensions costs no extra read of the data.
func copyPageReader(w io.Writer, src io.Reader) (chapter_downloader.ComicInfoPage, error) {
	ret := chapter_downloader.ComicInfoPage{}

	counter := &countingWriter{w: w}
	// The planning pass already rejected oversized pages by their declared size.
	// Reading one byte past the limit catches an entry whose header understated
	// it, which would otherwise be silently truncated into a corrupt page.
	tee := io.TeeReader(io.LimitReader(src, int64(maxLocalMangaPageSize)+1), counter)

	// A format Go cannot decode (AVIF, JXL) is still a valid page; it just goes
	// in without recorded dimensions.
	if config, _, cfgErr := image.DecodeConfig(tee); cfgErr == nil {
		ret.ImageWidth = config.Width
		ret.ImageHeight = config.Height
	}

	// Whatever the decoder did not consume still has to reach the archive, and
	// the tee writes everything it reads.
	if _, err := io.Copy(io.Discard, tee); err != nil {
		return ret, err
	}

	if counter.written > int64(maxLocalMangaPageSize) {
		return ret, ErrLocalMangaUploadTooLarge
	}

	ret.ImageSize = counter.written
	return ret, nil
}

type countingWriter struct {
	w       io.Writer
	written int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.written += int64(n)
	return n, err
}

// DeleteLocalMangaSeries removes a series directory and any mapping pointing at
// it.
func (r *Repository) DeleteLocalMangaSeries(name string) (err error) {
	defer util.HandlePanicInModuleWithError("manga/DeleteLocalMangaSeries", &err)

	seriesDir, err := r.resolveLocalMangaSeriesDir(name)
	if err != nil {
		return err
	}

	info, err := os.Stat(seriesDir)
	if err != nil || !info.IsDir() {
		return ErrLocalMangaSeriesNotFound
	}

	series := filepath.Base(seriesDir)
	if mediaId, ok := r.localMangaMappings()[series]; ok {
		if err := r.RemoveMapping(manga_providers.LocalProvider, mediaId); err != nil {
			r.logger.Warn().Err(err).Int("mediaId", mediaId).Msg("manga: Failed to remove local mapping")
		}
	}

	if err := os.RemoveAll(seriesDir); err != nil {
		return err
	}

	r.logger.Info().Str("series", series).Msg("manga: Deleted local manga series")
	return nil
}

// invalidateLocalMangaSeriesCache drops the cached chapter list of whichever
// entry maps to the series, so an upload shows up without a manual refresh.
func (r *Repository) invalidateLocalMangaSeriesCache(series string) {
	mediaId, ok := r.localMangaMappings()[series]
	if !ok {
		return
	}

	bucket := r.getFcProviderBucket(manga_providers.LocalProvider, mediaId, bucketTypeChapter)
	_ = r.fileCacher.Remove(bucket.Name())
}
