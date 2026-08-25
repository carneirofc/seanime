package manga

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/util"
	"slices"
	"sort"
	"strings"
)

// repackableExtensions are the chapter formats that can be rebuilt into a CBZ.
//
// Formats Seanime cannot read are deliberately absent: .cbr needs a RAR decoder
// and .pdf a rasterizer, and rewriting either would mean losing the file to
// produce something empty.
var repackableExtensions = []string{".zip", ".cbz"}

type (
	// LocalMangaRepackedChapter is one chapter the repack rewrote.
	LocalMangaRepackedChapter struct {
		// From is the chapter file or folder that was read.
		From string `json:"from"`
		// To are the CBZ files written in its place. More than one when a chapter
		// file turned out to hold several chapters.
		To        []string `json:"to"`
		PageCount int      `json:"pageCount"`
	}

	// LocalMangaRepackSkipped is a chapter the repack left untouched.
	LocalMangaRepackSkipped struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	}

	// LocalMangaRepackResult reports what a repack did to a series.
	LocalMangaRepackResult struct {
		Series   string                       `json:"series"`
		Repacked []*LocalMangaRepackedChapter `json:"repacked"`
		Skipped  []*LocalMangaRepackSkipped   `json:"skipped"`
	}
)

// Reasons a chapter was not repacked, as reported to the client.
const (
	LocalMangaRepackUnsupported = "unsupported_format"
	LocalMangaRepackNoPages     = "no_pages"
	LocalMangaRepackConflict    = "name_conflict"
	LocalMangaRepackFailed      = "failed"
)

// RepackLocalMangaSeries rewrites every chapter of a series as a proper CBZ:
// a flat archive of pages in reading order with a current ComicInfo.xml.
//
// This is what brings a hand-assembled folder up to the same standard as an
// upload — loose .zip files become .cbz, pages inside nested folders are
// flattened, junk entries are dropped, and the metadata is rebuilt from the
// AniList entry the series maps to. It is also how metadata is refreshed after a
// series is mapped to a different entry.
//
// Chapters are rewritten one at a time and each rewrite is atomic, so a failure
// part-way through leaves every chapter either fully repacked or untouched.
func (r *Repository) RepackLocalMangaSeries(
	series string,
	collection *anilist.MangaCollection,
) (ret *LocalMangaRepackResult, err error) {
	defer util.HandlePanicInModuleWithError("manga/RepackLocalMangaSeries", &err)

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

	seriesName := filepath.Base(seriesDir)
	metadata := r.resolveLocalMangaSeriesMetadata(seriesName, collection)

	ret = &LocalMangaRepackResult{
		Series:   seriesName,
		Repacked: make([]*LocalMangaRepackedChapter, 0),
		Skipped:  make([]*LocalMangaRepackSkipped, 0),
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		// Directories of loose images are chapters too, but rebuilding one means
		// deleting a folder rather than replacing a file. Left alone: the gain
		// does not justify a destructive operation on something the user
		// assembled by hand.
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if !isRepackableChapterFile(name) {
			ret.Skipped = append(ret.Skipped, &LocalMangaRepackSkipped{Name: name, Reason: LocalMangaRepackUnsupported})
			continue
		}

		repacked, reason := r.repackChapterFile(seriesDir, name, metadata)
		if repacked == nil {
			ret.Skipped = append(ret.Skipped, &LocalMangaRepackSkipped{Name: name, Reason: reason})
			continue
		}
		ret.Repacked = append(ret.Repacked, repacked)
	}

	if len(ret.Repacked) > 0 {
		r.invalidateLocalMangaSeriesCache(seriesName)
	}

	r.logger.Info().
		Str("series", seriesName).
		Int("repacked", len(ret.Repacked)).
		Int("skipped", len(ret.Skipped)).
		Msg("manga: Repacked local manga series")

	return ret, nil
}

func isRepackableChapterFile(name string) bool {
	return slices.Contains(repackableExtensions, strings.ToLower(filepath.Ext(name)))
}

// repackChapterFile rewrites one chapter file, returning nil and a reason when
// it was left alone.
//
// The source is only removed once every replacement is safely in place, and a
// replacement that would overwrite a *different* existing chapter is refused —
// silently merging two chapters into one file would lose pages.
func (r *Repository) repackChapterFile(
	seriesDir string,
	name string,
	metadata *localMangaSeriesMetadata,
) (*LocalMangaRepackedChapter, string) {
	sourcePath := filepath.Join(seriesDir, name)

	reader, err := zip.OpenReader(sourcePath)
	if err != nil {
		r.logger.Warn().Err(err).Str("chapter", name).Msg("manga: Chapter archive is not readable")
		return nil, LocalMangaRepackUnsupported
	}

	chapters, err := planLocalMangaChapters(&reader.Reader, name)
	if err != nil {
		_ = reader.Close()
		if errors.Is(err, ErrLocalMangaUploadNoPages) {
			return nil, LocalMangaRepackNoPages
		}
		r.logger.Warn().Err(err).Str("chapter", name).Msg("manga: Failed to plan chapter repack")
		return nil, LocalMangaRepackFailed
	}

	for _, chapter := range chapters {
		// Replacing the source under its own name is the normal case. Any other
		// existing file belongs to a different chapter and must not be clobbered.
		if chapter.filename == name {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(seriesDir, chapter.filename)); statErr == nil {
			_ = reader.Close()
			return nil, LocalMangaRepackConflict
		}
	}

	// Staged next to the destination so the final move is a rename on the same
	// filesystem, and so a failure leaves the original chapter in place.
	staged := make([]string, 0, len(chapters))
	ret := &LocalMangaRepackedChapter{From: name, To: make([]string, 0, len(chapters))}

	cleanup := func() {
		for _, path := range staged {
			_ = os.Remove(path)
		}
	}

	for _, chapter := range chapters {
		stagedPath := filepath.Join(seriesDir, localMangaUploadStagingPrefix+chapter.filename)
		if err := writeNormalizedCBZ(stagedPath, chapter.pages, metadata); err != nil {
			_ = reader.Close()
			cleanup()
			r.logger.Warn().Err(err).Str("chapter", name).Msg("manga: Failed to repack chapter")
			return nil, LocalMangaRepackFailed
		}
		staged = append(staged, stagedPath)
		ret.To = append(ret.To, chapter.filename)
		ret.PageCount += len(chapter.pages)
	}

	// Windows refuses to replace a file that is still open, so the source is
	// released before anything moves.
	if err := reader.Close(); err != nil {
		cleanup()
		return nil, LocalMangaRepackFailed
	}

	// Replacements move into place before the source is touched, so a failed
	// rename cannot leave the chapter with neither file.
	for index, stagedPath := range staged {
		if err := os.Rename(stagedPath, filepath.Join(seriesDir, ret.To[index])); err != nil {
			cleanup()
			r.logger.Error().Err(err).Str("chapter", name).Msg("manga: Failed to move the repacked chapter into place")
			return nil, LocalMangaRepackFailed
		}
	}

	// A ".zip" rebuilt as ".cbz" leaves its source behind; one rebuilt under its
	// own name has already been overwritten by the rename above.
	if !slices.Contains(ret.To, name) {
		if err := os.Remove(sourcePath); err != nil {
			r.logger.Warn().Err(err).Str("chapter", name).Msg("manga: Failed to remove the repacked source")
		}
	}

	return ret, ""
}
