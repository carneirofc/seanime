package scanner

import (
	"path"
	"seanime/internal/library/anime"
	"seanime/internal/util"
	"strings"

	"github.com/5rahim/habari"
	"github.com/rs/zerolog"
)

// A "collection folder" is a folder that does not name an entry of its own but groups several
// distinct ones together, e.g.
//
//	Monogatari Series/{Bakemonogatari, Nisemonogatari, Kizumonogatari, ...}
//	JoJo's Bizarre Adventure/{Part 1 Phantom Blood, Part 3 Stardust Crusaders}
//
// Its name is often an exact AniList title, which used to let it outscore the subfolder that names
// the entry the file actually belongs to: an exact title match is worth up to 20 points while a
// subfolder whose title is only a token subset of its entry's title is worth 12. The matcher still
// uses a collection folder's title to look up candidates and to prefer entries of the same
// franchise, but it can only win a match when nothing more specific does.

// minCollectionChildren is how many self-identifying subfolders a folder needs before it is
// considered a collection rather than a series folder with a subfolder or two.
const minCollectionChildren = 2

// detectCollectionFolders returns the set of normalized directory paths that group several distinct
// entries together. extraPaths are file paths known to exist in the library but not being matched
// (locked, ignored or shelved files); they only contribute to the shape of the tree, so that such a
// file's folder does not hide a collection.
//
// The result is written once, before matching starts, and only read from then on.
func detectCollectionFolders(lfs []*anime.LocalFile, extraPaths []string) map[string]struct{} {
	// dir -> the immediate subdirectories that contain media files (at any depth) and name
	// something of their own
	namedChildren := make(map[string]map[string]struct{})
	// dir -> whether its name reads as a media title, memoized since many files share a directory
	named := make(map[string]bool)

	isNamed := func(dir string) bool {
		if v, ok := named[dir]; ok {
			return v
		}
		v := folderNameIsTitle(path.Base(dir))
		named[dir] = v
		return v
	}

	// record walks a file's ancestor directories, noting for each one which of its children name
	// something of their own.
	record := func(filePath string) {
		dir := path.Dir(util.NormalizePath(filePath))
		for {
			parent := path.Dir(dir)
			if parent == dir {
				// Reached the filesystem root
				break
			}
			if isNamed(dir) {
				children, ok := namedChildren[parent]
				if !ok {
					children = make(map[string]struct{})
					namedChildren[parent] = children
				}
				children[dir] = struct{}{}
			}
			dir = parent
		}
	}

	for _, lf := range lfs {
		if lf == nil {
			continue
		}
		record(lf.Path)
	}
	for _, p := range extraPaths {
		record(p)
	}

	collections := make(map[string]struct{})
	for dir, children := range namedChildren {
		if len(children) < minCollectionChildren {
			continue
		}
		collections[dir] = struct{}{}
	}

	return collections
}

// folderNameIsTitle reports whether a folder name reads as a media title of its own, rather than
// being empty of one ("Season 1") or a keyword ("Extras", "Specials").
func folderNameIsTitle(name string) bool {
	if name == "" {
		return false
	}
	return anime.IsUsableFolderTitle(habari.Parse(name).FormattedTitle, name)
}

// folderPathAt returns the normalized directory path of the i-th entry of lf.ParsedFolderData.
// ParsedFolderData holds the trailing path segments of the file's directory, in order, so the i-th
// entry is reached by walking up from the file's own directory.
// It returns false when the path does not line up with the parsed data, in which case the caller
// should leave the title alone rather than guess.
func folderPathAt(lf *anime.LocalFile, i int) (string, bool) {
	if i < 0 || i >= len(lf.ParsedFolderData) {
		return "", false
	}

	dir := path.Dir(util.NormalizePath(lf.Path))
	for k := len(lf.ParsedFolderData) - 1; k > i; k-- {
		parent := path.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}

	if !strings.EqualFold(path.Base(dir), strings.TrimSpace(lf.ParsedFolderData[i].Original)) {
		return "", false
	}

	return dir, true
}

// titlesAreRelated reports whether two titles plausibly name the same thing, so that a parent
// folder's title is not demoted when the subfolder merely abbreviates or extends it —
// "Re Zero kara Hajimeru Isekai Seikatsu" vs "ReZero", or "One Punch Man Series" vs "One Punch Man".
//
// The test is deliberately token-based rather than fuzzy: string similarity reads
// "Monogatari Series" and "Bakemonogatari" as the same thing, which is exactly the case this has to
// tell apart.
func titlesAreRelated(a, b *NormalizedTitle) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Normalized == "" || b.Normalized == "" {
		return false
	}
	if a.Normalized == b.Normalized {
		return true
	}

	aTokens := GetSignificantTokens(a.Tokens)
	bTokens := GetSignificantTokens(b.Tokens)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return false
	}
	if ContainsAllTokens(aTokens, bTokens) || ContainsAllTokens(bTokens, aTokens) {
		return true
	}
	// Compound tokens, so that "ReZero" is related to "Re Zero kara ...".
	// This mirrors the compound indexing NewMediaContainer does.
	return hasCompoundTokenMatch(aTokens, bTokens) || hasCompoundTokenMatch(bTokens, aTokens)
}

// hasCompoundTokenMatch reports whether any token of tokens is the concatenation of two adjacent
// short tokens of other.
func hasCompoundTokenMatch(tokens, other []string) bool {
	if len(tokens) == 0 || len(other) < 2 {
		return false
	}
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	for i := 0; i < len(other)-1; i++ {
		if len(other[i]) > 5 || len(other[i+1]) > 5 {
			continue
		}
		if _, ok := set[other[i]+other[i+1]]; ok {
			return true
		}
	}
	return false
}

// collectionTitles returns the titles of lf's folders that name a collection rather than the entry
// the file belongs to, keyed by title so the caller can flag the matching title variations.
//
// Only titles that are already "extra" (i.e. some closer folder or the filename identified the
// entry) are eligible: when nothing deeper identifies it, the collection folder's title is all we
// have and demoting it would lose the file. A title related to the file's own — an abbreviation or
// an extension of it, like "ReZero" under "Re Zero kara Hajimeru Isekai Seikatsu" — is left alone
// too, since there the parent is what actually identifies the entry.
func (m *Matcher) collectionTitles(
	lf *anime.LocalFile,
	primaryTitles map[string]struct{},
	extraTitles map[string]struct{},
) map[string]struct{} {
	if len(m.collectionDirs) == 0 || len(extraTitles) == 0 {
		return nil
	}

	primaryNormalized := make([]*NormalizedTitle, 0, len(primaryTitles))
	for t := range primaryTitles {
		primaryNormalized = append(primaryNormalized, NormalizeTitle(t))
	}

	var containers map[string]struct{}
	for _, idx := range lf.ValidFolderTitleIndexes() {
		title := lf.ParsedFolderData[idx].Title
		if _, isExtra := extraTitles[title]; !isExtra {
			continue
		}
		if _, already := containers[title]; already {
			continue
		}

		dir, ok := folderPathAt(lf, idx)
		if !ok {
			continue
		}
		if _, isCollection := m.collectionDirs[dir]; !isCollection {
			continue
		}

		normalized := NormalizeTitle(title)
		related := false
		for _, pt := range primaryNormalized {
			if titlesAreRelated(normalized, pt) {
				related = true
				break
			}
		}
		if related {
			continue
		}

		if containers == nil {
			containers = make(map[string]struct{}, 1)
		}
		containers[title] = struct{}{}

		if m.ScanLogger != nil {
			m.ScanLogger.LogMatcher(zerolog.DebugLevel).
				Str("filename", lf.Name).
				Str("folder", title).
				Msg("Demoting collection folder title to a hint")
		}
	}

	return containers
}
