package manga

import (
	"regexp"
	"seanime/internal/api/anilist"
	chapter_downloader "seanime/internal/manga/downloader"
	manga_providers "seanime/internal/manga/providers"
	"strings"
)

// localMangaSeriesMetadata is everything known about a series that belongs in
// the ComicInfo document of its chapters.
//
// It is resolved once per series rather than per chapter: the AniList lookup is
// the expensive part and every chapter of a series shares the answer.
type localMangaSeriesMetadata struct {
	// SeriesTitle is what goes in ComicInfo/Series. The AniList title when the
	// series is mapped, the folder name otherwise — a reader has to be able to
	// group the chapters even for an unmapped folder.
	SeriesTitle string
	Summary     string
	Genre       string
	Web         string
	Year        int
	// Count is the total chapter count of the series, when AniList knows it.
	Count       int
	LanguageISO string
	AgeRating   string
}

// resolveLocalMangaSeriesMetadata builds the series-level metadata for a folder,
// enriching it from the AniList entry mapped to it when there is one.
func (r *Repository) resolveLocalMangaSeriesMetadata(
	series string,
	collection *anilist.MangaCollection,
) *localMangaSeriesMetadata {
	ret := &localMangaSeriesMetadata{SeriesTitle: NormalizeLocalMangaTitle(series)}
	if ret.SeriesTitle == "" {
		ret.SeriesTitle = series
	}

	mediaId, mapped := r.localMangaMappings()[series]
	if !mapped || collection == nil {
		return ret
	}

	entry, found := collection.GetListEntryFromMangaId(mediaId)
	if !found || entry.GetMedia() == nil {
		return ret
	}

	applyMangaMetadata(ret, entry.GetMedia())
	return ret
}

func applyMangaMetadata(ret *localMangaSeriesMetadata, media *anilist.BaseManga) {
	if title := media.GetPreferredTitle(); title != "" {
		ret.SeriesTitle = title
	}
	if media.Description != nil {
		ret.Summary = stripHTMLTags(*media.Description)
	}
	if media.SiteURL != nil {
		ret.Web = *media.SiteURL
	}
	if media.Chapters != nil {
		ret.Count = *media.Chapters
	}
	if media.StartDate != nil && media.StartDate.Year != nil {
		ret.Year = *media.StartDate.Year
	}
	if media.IsAdult != nil && *media.IsAdult {
		ret.AgeRating = "Adults Only 18+"
	}

	genres := make([]string, 0, len(media.Genres))
	for _, genre := range media.Genres {
		if genre != nil && strings.TrimSpace(*genre) != "" {
			genres = append(genres, strings.TrimSpace(*genre))
		}
	}
	// ComicInfo defines Genre as a single comma-separated list.
	ret.Genre = strings.Join(genres, ", ")

	ret.LanguageISO = languageISOForCountry(media.CountryOfOrigin)
}

// languageISOForCountry maps AniList's country of origin to the language the
// work was originally published in, which is what ComicInfo/LanguageISO means.
func languageISOForCountry(country *string) string {
	if country == nil {
		return ""
	}

	switch strings.ToUpper(strings.TrimSpace(*country)) {
	case "JP":
		return "ja"
	case "KR":
		return "ko"
	case "CN", "TW":
		return "zh"
	default:
		return ""
	}
}

var (
	htmlTagPattern     = regexp.MustCompile(`<[^>]*>`)
	htmlEntityReplacer = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
	)
)

// stripHTMLTags turns an AniList description into plain text. Descriptions come
// back with <br> and <i> markup that a CBZ reader would show literally.
func stripHTMLTags(value string) string {
	value = strings.ReplaceAll(value, "<br>", "\n")
	value = strings.ReplaceAll(value, "<br/>", "\n")
	value = strings.ReplaceAll(value, "<br />", "\n")
	value = htmlTagPattern.ReplaceAllString(value, "")
	value = htmlEntityReplacer.Replace(value)

	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// buildLocalChapterComicInfo assembles the metadata document stored inside a
// generated chapter archive.
//
// The chapter's own identity comes from the filename it is being stored under,
// parsed with the same parser the library scanner uses, so the metadata and the
// chapter list can never disagree.
func buildLocalChapterComicInfo(
	metadata *localMangaSeriesMetadata,
	chapterFilename string,
	pages []chapter_downloader.ComicInfoPage,
) *chapter_downloader.ComicInfo {
	naming := manga_providers.ParseChapterNaming(chapterFilename)

	info := chapter_downloader.NewComicInfo()
	info.Series = metadata.SeriesTitle
	info.Number = naming.Number
	info.Volume = naming.Volume
	info.Count = metadata.Count
	info.Summary = metadata.Summary
	info.Genre = metadata.Genre
	info.Web = metadata.Web
	info.Year = metadata.Year
	info.LanguageISO = metadata.LanguageISO
	info.AgeRating = metadata.AgeRating
	info.Manga = chapter_downloader.MangaRightToLeft

	info.Title = naming.Title
	if info.Title == "" && naming.Number != "" {
		info.Title = "Chapter " + naming.Number
	}
	if info.Title == "" {
		info.Title = strings.TrimSuffix(chapterFilename, ".cbz")
	}

	info.SetPages(pages)

	return info
}
