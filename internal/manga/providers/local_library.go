package manga_providers

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SeriesInfo describes one series directory of the local manga library.
//
// DirName doubles as the provider's manga ID: it is what [Local.FindChapters]
// takes and what a manga mapping stores, so it is always a single path element
// relative to the library root.
type SeriesInfo struct {
	DirName      string `json:"dirName"`
	ChapterCount int    `json:"chapterCount"`
	Size         int64  `json:"size"`
}

// Directory returns the library root the provider currently scans.
func (p *Local) Directory() string {
	return p.dir
}

// ChapterNaming is what a local chapter filename says about the chapter.
type ChapterNaming struct {
	// Number is the chapter number, normalized ("0001" -> "1"). Empty when the
	// filename carries none.
	Number string
	// Title is the part of the filename that follows the chapter number.
	Title string
	// Volume is the volume the chapter belongs to, when the filename says.
	Volume string
}

// ParseChapterNaming reads the chapter number, title and volume out of a local
// chapter filename, using the same parser the chapter scanner uses.
//
// Exported so metadata written into an archive agrees with the chapter the
// scanner will later report for that same file.
func ParseChapterNaming(filename string) ChapterNaming {
	scanned, ok := scanChapterFilename(filename)
	if !ok || scanned == nil {
		return ChapterNaming{}
	}

	ret := ChapterNaming{Title: scanned.ChapterTitle}

	if len(scanned.Chapter) > 0 {
		// A range ("Ch 1-3") is tracked by its last chapter, matching how
		// FindChapters reports it.
		ret.Number = cleanChapter(scanned.Chapter[len(scanned.Chapter)-1])
	}
	if len(scanned.Volume) > 0 {
		ret.Volume = cleanChapter(scanned.Volume[0])
	}

	return ret
}

// ListSeries returns every series directory of the local library along with the
// number of chapters the provider can read from it and its on-disk size.
//
// Directories the chapter scanner finds nothing in are still returned, with a
// zero chapter count — the caller decides whether an empty series is worth
// reporting.
func (p *Local) ListSeries() ([]*SeriesInfo, error) {
	if p.dir == "" {
		return make([]*SeriesInfo, 0), nil
	}

	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, err
	}

	ret := make([]*SeriesInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		chapters, err := p.FindChapters(entry.Name())
		if err != nil {
			// A single unreadable series must not hide the rest of the library.
			p.logger.Warn().Err(err).Str("series", entry.Name()).Msg("manga: Failed to scan local series")
			chapters = nil
		}

		ret = append(ret, &SeriesInfo{
			DirName:      entry.Name(),
			ChapterCount: len(chapters),
			Size:         dirSize(filepath.Join(p.dir, entry.Name())),
		})
	}

	slices.SortFunc(ret, func(a, b *SeriesInfo) int {
		return strings.Compare(strings.ToLower(a.DirName), strings.ToLower(b.DirName))
	})

	return ret, nil
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable subtree just doesn't count toward the total
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr
		}
		total += info.Size()
		return nil
	})
	return total
}
