package manga

import (
	"errors"
	"os"
	"path/filepath"
	"seanime/internal/api/anilist"
	"seanime/internal/extension"
	manga_providers "seanime/internal/manga/providers"
	"seanime/internal/util"
	"slices"
	"strings"
)

var (
	// ErrLocalMangaProviderUnavailable is returned when the built-in local provider
	// is not registered — every local-library operation goes through it.
	ErrLocalMangaProviderUnavailable = errors.New("the local manga provider is not available")
	ErrLocalMangaSeriesNotFound      = errors.New("local manga series not found")
)

type (
	// LocalMangaSeries is one series directory of the local manga library, paired
	// with the AniList entry it currently maps to (if any).
	//
	// No host path is exposed: DirName is relative to the library root, which is
	// the only directory the local-library endpoints ever touch.
	LocalMangaSeries struct {
		DirName      string `json:"dirName"`
		Title        string `json:"title"`
		ChapterCount int    `json:"chapterCount"`
		Size         int64  `json:"size"`
		MediaId      *int   `json:"mediaId"`
		MediaTitle   string `json:"mediaTitle"`
	}

	// LocalMangaLibrary is the whole local manga library as the client sees it.
	LocalMangaLibrary struct {
		// Configured reports whether the user pointed the local provider at a
		// directory of their own. When false the library lives in Seanime's own
		// data directory, which is still writable but usually empty.
		Configured bool                `json:"configured"`
		Series     []*LocalMangaSeries `json:"series"`
		// UnmappedCount is how many series have no AniList entry mapped to them.
		UnmappedCount int `json:"unmappedCount"`
	}
)

// LocalMangaDirectory returns the directory the local manga library lives in:
// the user-configured source directory when set, Seanime's own manga directory
// otherwise.
//
// The second return value distinguishes the two, because the client presents
// them differently — an unconfigured library is a valid upload target but not
// somewhere the user expects to find existing manga.
func (r *Repository) LocalMangaDirectory() (dir string, configured bool) {
	r.mu.Lock()
	settings := r.settings
	r.mu.Unlock()

	if settings != nil && settings.Manga != nil && strings.TrimSpace(settings.Manga.LocalSourceDirectory) != "" {
		return strings.TrimSpace(settings.Manga.LocalSourceDirectory), true
	}

	return r.localDir, false
}

// localMangaProvider returns the built-in local provider, pointed at the
// currently configured library directory.
func (r *Repository) localMangaProvider() (*manga_providers.Local, error) {
	providerExtension, ok := extension.GetExtension[extension.MangaProviderExtension](
		r.extensionBankRef.Get(),
		manga_providers.LocalProvider,
	)
	if !ok {
		return nil, ErrLocalMangaProviderUnavailable
	}

	provider, ok := providerExtension.GetProvider().(*manga_providers.Local)
	if !ok {
		return nil, ErrLocalMangaProviderUnavailable
	}

	dir, _ := r.LocalMangaDirectory()
	if dir == "" {
		return nil, ErrLocalMangaProviderUnavailable
	}
	provider.SetSourceDirectory(dir)

	return provider, nil
}

// GetLocalMangaLibrary lists the local manga library and resolves each series to
// the AniList entry mapped to it.
func (r *Repository) GetLocalMangaLibrary(collection *anilist.MangaCollection) (ret *LocalMangaLibrary, err error) {
	defer util.HandlePanicInModuleWithError("manga/GetLocalMangaLibrary", &err)

	dir, configured := r.LocalMangaDirectory()

	provider, err := r.localMangaProvider()
	if err != nil {
		return nil, err
	}

	// The library root is created on demand so a fresh install can be uploaded to
	// without the user having to prepare anything first.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	series, err := provider.ListSeries()
	if err != nil {
		return nil, err
	}

	mappedMediaIds := r.localMangaMappings()
	titlesByMediaId := mangaTitlesByMediaId(collection)

	ret = &LocalMangaLibrary{
		Configured: configured,
		Series:     make([]*LocalMangaSeries, 0, len(series)),
	}

	for _, info := range series {
		entry := &LocalMangaSeries{
			DirName:      info.DirName,
			Title:        NormalizeLocalMangaTitle(info.DirName),
			ChapterCount: info.ChapterCount,
			Size:         info.Size,
		}

		if mediaId, ok := mappedMediaIds[info.DirName]; ok {
			entry.MediaId = &mediaId
			entry.MediaTitle = titlesByMediaId[mediaId]
		} else {
			ret.UnmappedCount++
		}

		ret.Series = append(ret.Series, entry)
	}

	return ret, nil
}

// localMangaMappings returns the series directory -> media ID mappings currently
// recorded for the local provider.
//
// A directory can legitimately be claimed by only one entry; when the database
// holds more (hand-edited, or mapped before the entry was renamed) the lowest
// media ID wins so the listing stays stable between calls.
func (r *Repository) localMangaMappings() map[string]int {
	ret := make(map[string]int)

	mappings, err := r.db.GetMangaMappingsByProvider(manga_providers.LocalProvider)
	if err != nil {
		r.logger.Warn().Err(err).Msg("manga: Failed to read local manga mappings")
		return ret
	}

	for _, mapping := range mappings {
		if mapping == nil || mapping.MangaID == "" {
			continue
		}
		if existing, ok := ret[mapping.MangaID]; ok && existing <= mapping.MediaID {
			continue
		}
		ret[mapping.MangaID] = mapping.MediaID
	}

	return ret
}

func mangaTitlesByMediaId(collection *anilist.MangaCollection) map[int]string {
	ret := make(map[int]string)
	if collection == nil || collection.MediaListCollection == nil {
		return ret
	}

	for _, list := range collection.MediaListCollection.Lists {
		for _, entry := range list.GetEntries() {
			if entry == nil || entry.GetMedia() == nil {
				continue
			}
			ret[entry.GetMedia().ID] = entry.GetMedia().GetPreferredTitle()
		}
	}

	return ret
}

// localMangaArchiveExtensions are the extensions a chapter file may carry.
var localMangaArchiveExtensions = []string{".cbz", ".cbr", ".zip", ".rar", ".7z", ".pdf"}

// NormalizeLocalMangaTitle turns a series directory name into something worth
// matching against AniList titles: separators become spaces and the trailing
// metadata scene releases carry — bracketed groups, a release year — is dropped.
//
// It deliberately keeps the words themselves untouched; the actual matching is
// fuzzy, so over-cleaning loses more than it gains.
func NormalizeLocalMangaTitle(dirName string) string {
	title := strings.TrimSpace(dirName)
	if title == "" {
		return ""
	}

	// Drop a file extension if the caller passed a filename. Only known archive
	// extensions count: titles contain dots of their own ("Vinland.Saga"), and
	// treating those as extensions would eat the last word.
	if ext := strings.ToLower(filepath.Ext(title)); slices.Contains(localMangaArchiveExtensions, ext) {
		title = title[:len(title)-len(ext)]
	}

	title = removeEnclosedGroups(title)

	// Separators folders are named with. Hyphens included: "One-Piece" is the
	// same series as "One Piece", and no manga title depends on a hyphen to mean
	// something a space would not.
	title = strings.Map(func(r rune) rune {
		switch r {
		case '_', '.', '+', '-':
			return ' '
		}
		return r
	}, title)

	title = strings.Join(strings.Fields(title), " ")
	title = strings.Trim(title, " -–—")

	return title
}

// removeEnclosedGroups strips [...], (...) and {...} runs, which in a manga
// directory name hold the scanlation group, the year or the source, never the
// title itself.
func removeEnclosedGroups(value string) string {
	var b strings.Builder
	depth := 0

	for _, r := range value {
		switch r {
		case '[', '(', '{':
			depth++
			continue
		case ']', ')', '}':
			if depth > 0 {
				depth--
				b.WriteRune(' ')
			}
			continue
		}
		if depth == 0 {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// resolveLocalMangaSeriesDir validates a caller-supplied series directory name
// and returns its absolute path inside the library root.
//
// This is the only place a client-supplied name becomes a path, so it is where
// traversal is stopped: the name must be a single, non-special path element, and
// the resolved path must still sit under the root.
func (r *Repository) resolveLocalMangaSeriesDir(name string) (string, error) {
	dir, _ := r.LocalMangaDirectory()
	if dir == "" {
		return "", ErrLocalMangaProviderUnavailable
	}

	sanitized, err := SanitizeLocalMangaName(name)
	if err != nil {
		return "", err
	}

	root, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	full := filepath.Join(root, sanitized)
	if !util.IsSubdirectory(root, full) {
		return "", ErrInvalidLocalMangaName
	}

	return full, nil
}

// ErrInvalidLocalMangaName is returned for a series or file name that cannot be
// safely used as a path element.
var ErrInvalidLocalMangaName = errors.New("invalid name: use a plain name without path separators")

// windowsReservedNames are rejected on every platform, not just Windows: a
// library directory is routinely synced or served across machines, and a file
// called "CON.cbz" turns into a problem the moment it lands on one.
var windowsReservedNames = []string{
	"con", "prn", "aux", "nul",
	"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
	"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9",
}

// SanitizeLocalMangaName validates that a client-supplied name is usable as a
// single path element, returning it trimmed.
//
// It rejects rather than rewrites: a silently mangled series name would map to a
// directory the user did not ask for, and they would have no way to tell.
func SanitizeLocalMangaName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	trimmed = strings.Trim(trimmed, ".")
	trimmed = strings.TrimSpace(trimmed)

	if trimmed == "" || len(trimmed) > 200 {
		return "", ErrInvalidLocalMangaName
	}

	if strings.ContainsAny(trimmed, `/\:*?"<>|`) {
		return "", ErrInvalidLocalMangaName
	}

	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return "", ErrInvalidLocalMangaName
		}
	}

	base := strings.ToLower(trimmed)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if slices.Contains(windowsReservedNames, base) {
		return "", ErrInvalidLocalMangaName
	}

	return trimmed, nil
}
