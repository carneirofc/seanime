package chapter_downloader

import (
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
)

// MigrateLegacyChapterDirs converts legacy loose-image chapter directories
// ({provider}_{mediaId}_{chapterId}_{chapterNumber}/ with registry.json) into
// CBZ archives in the per-series layout, deleting each source directory only
// after its archive has been written and renamed into place.
//
// The routine is idempotent and runs on every startup: it is a no-op when no
// legacy directory exists. Failures are non-fatal — a directory that cannot be
// converted is left untouched and remains readable through the legacy code paths.
func MigrateLegacyChapterDirs(downloadDir string, logger *zerolog.Logger) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return
	}

	var migrated, failed int
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		id, ok := ParseChapterDirName(entry.Name())
		if !ok {
			// Sweep temp archives left by an interrupted writeCBZ inside series dirs
			if _, _, isSeries := ParseSeriesDirName(entry.Name()); isSeries {
				sweepTmpFiles(filepath.Join(downloadDir, entry.Name()))
			}
			continue
		}

		srcDir := filepath.Join(downloadDir, entry.Name())
		destDir := filepath.Join(downloadDir, FormatSeriesDirName(id.Provider, id.MediaId))
		destPath := filepath.Join(destDir, FormatChapterFileName(id.ChapterId, id.ChapterNumber))

		// A previous run already converted this chapter but crashed before cleanup
		if _, err := os.Stat(destPath); err == nil {
			_ = os.RemoveAll(srcDir)
			migrated++
			continue
		}

		registry, ok := readLegacyRegistry(srcDir, logger)
		if !ok {
			failed++
			continue
		}

		if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
			logger.Warn().Err(err).Str("dir", entry.Name()).Msg("chapter downloader: Migration failed to create series directory, skipping")
			failed++
			continue
		}

		// Media/chapter titles are unknown at migration time
		info := buildComicInfo(id, "", "", registry)
		if err := writeCBZ(destPath, srcDir, registry, info); err != nil {
			logger.Warn().Err(err).Str("dir", entry.Name()).Msg("chapter downloader: Migration failed to write chapter archive, skipping")
			failed++
			continue
		}

		_ = os.RemoveAll(srcDir)
		migrated++
	}

	if migrated > 0 || failed > 0 {
		logger.Info().Msgf("chapter downloader: Migrated legacy chapter directories to CBZ: migrated=%d failed=%d", migrated, failed)
	}
}

// readLegacyRegistry loads registry.json from a legacy chapter directory and
// verifies every referenced page file exists and is non-empty.
// When registry.json is missing or corrupt, a registry is synthesized from the
// image files found in the directory (sorted by filename).
func readLegacyRegistry(srcDir string, logger *zerolog.Logger) (Registry, bool) {
	registry := make(Registry)

	data, err := os.ReadFile(filepath.Join(srcDir, "registry.json"))
	if err == nil {
		err = json.Unmarshal(data, &registry)
	}
	if err != nil || len(registry) == 0 {
		var ok bool
		registry, ok = synthesizeRegistry(srcDir)
		if !ok {
			logger.Warn().Str("dir", srcDir).Msg("chapter downloader: Legacy chapter has no readable registry or images, skipping")
			return nil, false
		}
		return registry, true
	}

	for _, pageInfo := range registry {
		fi, err := os.Stat(filepath.Join(srcDir, pageInfo.Filename))
		if err != nil || fi.Size() == 0 {
			logger.Warn().Str("dir", srcDir).Str("page", pageInfo.Filename).Msg("chapter downloader: Legacy chapter is missing a page file, skipping")
			return nil, false
		}
	}

	return registry, true
}

// synthesizeRegistry builds a registry from the image files present in a legacy
// chapter directory whose registry.json is missing or unreadable.
func synthesizeRegistry(srcDir string) (Registry, bool) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, false
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && isImageFilename(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return nil, false
	}
	sort.Strings(names)

	registry := make(Registry, len(names))
	for i, name := range names {
		pageInfo := PageInfo{
			Index:    i,
			Filename: name,
		}
		if f, err := os.Open(filepath.Join(srcDir, name)); err == nil {
			if config, _, err := image.DecodeConfig(f); err == nil {
				pageInfo.Width = config.Width
				pageInfo.Height = config.Height
			}
			if fi, err := f.Stat(); err == nil {
				pageInfo.Size = fi.Size()
			}
			_ = f.Close()
		}
		registry[i] = pageInfo
	}

	return registry, true
}

func sweepTmpFiles(seriesDir string) {
	entries, err := os.ReadDir(seriesDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".cbz.tmp") {
			_ = os.Remove(filepath.Join(seriesDir, entry.Name()))
		}
	}
}
