package manga

import (
	"context"
	"fmt"
	"seanime/internal/api/anilist"
	"seanime/internal/customsource"
	hibikemanga "seanime/internal/extension/hibike/manga"
	chapter_downloader "seanime/internal/manga/downloader"
	"strings"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
)

// DownloadMetadataPlatform is the part of platform.Platform the download
// metadata resolver needs, narrowed so tests can fake it.
type DownloadMetadataPlatform interface {
	GetManga(ctx context.Context, mediaID int) (*anilist.BaseManga, error)
	GetMangaDetails(ctx context.Context, mediaID int) (*anilist.MangaDetailsById_Media, error)
	GetAnilistClient() anilist.AnilistClient
}

// DownloadSeriesMetadata is the series-level metadata shared by every chapter
// of one download batch. Resolve it once per batch with ResolveDownloadSeriesMetadata:
// it costs up to three AniList requests.
type DownloadSeriesMetadata struct {
	Title    string
	Metadata chapter_downloader.ChapterMetadata
}

// ResolveDownloadSeriesMetadata gathers what AniList knows about a manga for the
// ComicInfo.xml of its downloaded chapters: title, summary, genres, tags,
// authors and main characters.
//
// Every lookup is best-effort. A failed one only leaves its fields empty, since
// missing metadata must never block a download.
func ResolveDownloadSeriesMetadata(
	ctx context.Context,
	p DownloadMetadataPlatform,
	collection *anilist.MangaCollection,
	mediaId int,
	logger *zerolog.Logger,
) *DownloadSeriesMetadata {
	ret := &DownloadSeriesMetadata{}
	if p == nil {
		return ret
	}

	var media *anilist.BaseManga
	if collection != nil {
		if entry, ok := collection.GetListEntryFromMangaId(mediaId); ok {
			media = entry.GetMedia()
		}
	}
	if media == nil {
		media, _ = p.GetManga(ctx, mediaId)
	}
	if media != nil {
		series := &localMangaSeriesMetadata{}
		applyMangaMetadata(series, media)
		ret.Title = series.SeriesTitle
		ret.Metadata.Summary = series.Summary
		ret.Metadata.Genre = series.Genre
		ret.Metadata.Web = series.Web
		ret.Metadata.Year = series.Year
		ret.Metadata.Count = series.Count
		ret.Metadata.AgeRating = series.AgeRating
		// series.LanguageISO is deliberately dropped: it is the original
		// language, and a scraped chapter is usually a translation.
	}

	if details, err := p.GetMangaDetails(ctx, mediaId); err == nil && details != nil {
		ret.Metadata.Tags = mangaTagNames(details.Tags)
		ret.Metadata.Characters = mainCharacterNames(details.Characters)
		if ret.Metadata.Web == "" && details.SiteURL != nil {
			ret.Metadata.Web = *details.SiteURL
		}
	}

	// Staff comes from AniList directly, which does not know custom-source ids.
	if !customsource.IsExtensionId(mediaId) {
		if client := p.GetAnilistClient(); client != nil && logger != nil {
			if staff, err := fetchMangaStaff(client, mediaId, logger); err == nil {
				ret.Metadata.Writer, ret.Metadata.Penciller = splitMangaStaff(staff)
			}
		}
	}

	return ret
}

// chapterDownloadMetadata adds what the manga source scraped for one chapter
// to the series metadata.
func chapterDownloadMetadata(
	series *DownloadSeriesMetadata,
	provider string,
	chapter *hibikemanga.ChapterDetails,
) *chapter_downloader.ChapterMetadata {
	ret := chapter_downloader.ChapterMetadata{}
	if series != nil {
		ret = series.Metadata
	}

	ret.Translator = strings.TrimSpace(chapter.Scanlator)
	ret.LanguageISO = strings.ToLower(strings.TrimSpace(chapter.Language))

	// The chapter page goes first: it is the source of this archive, the
	// AniList page only describes the series.
	if url := strings.TrimSpace(chapter.URL); url != "" {
		ret.Web = strings.TrimSpace(url + " " + ret.Web)
	}

	ret.Notes = fmt.Sprintf("Downloaded by Seanime from %s (chapter id %s).", provider, chapter.ID)

	return &ret
}

// mangaTagNames lists the tags of a manga in AniList's relevance order,
// skipping spoilers: ComicInfo tags are shown to anyone browsing the library.
func mangaTagNames(tags []*anilist.MTagsSlim) string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag == nil || strings.TrimSpace(tag.Name) == "" {
			continue
		}
		if (tag.IsMediaSpoiler != nil && *tag.IsMediaSpoiler) || (tag.IsGeneralSpoiler != nil && *tag.IsGeneralSpoiler) {
			continue
		}
		names = append(names, strings.TrimSpace(tag.Name))
	}
	return strings.Join(names, ", ")
}

func mainCharacterNames(characters *anilist.MangaDetailsById_Media_Characters) string {
	if characters == nil {
		return ""
	}
	names := make([]string, 0)
	for _, edge := range characters.Edges {
		if edge == nil || edge.Role == nil || *edge.Role != anilist.CharacterRoleMain {
			continue
		}
		if edge.Node == nil || edge.Node.Name == nil || edge.Node.Name.Full == nil {
			continue
		}
		if name := strings.TrimSpace(*edge.Node.Name.Full); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

type mangaStaffMember struct {
	Name string
	Role string
}

// mangaStaffQuery is sent as a custom query because the generated
// MangaDetailsById query does not select staff, and regenerating the AniList
// client would conflict with every upstream pull.
const mangaStaffQuery = `query ($id: Int) {
  Media(id: $id, type: MANGA) {
    staff(sort: [RELEVANCE], perPage: 25) {
      edges {
        role
        node { name { full } }
      }
    }
  }
}`

func fetchMangaStaff(client anilist.AnilistClient, mediaId int, logger *zerolog.Logger) ([]mangaStaffMember, error) {
	body, err := json.Marshal(map[string]any{
		"query":     mangaStaffQuery,
		"variables": map[string]any{"id": mediaId},
	})
	if err != nil {
		return nil, err
	}

	data, err := client.CustomQuery(body, logger)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var res struct {
		Media *struct {
			Staff *struct {
				Edges []*struct {
					Role *string `json:"role"`
					Node *struct {
						Name *struct {
							Full *string `json:"full"`
						} `json:"name"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"staff"`
		} `json:"Media"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	if res.Media == nil || res.Media.Staff == nil {
		return nil, nil
	}

	ret := make([]mangaStaffMember, 0, len(res.Media.Staff.Edges))
	for _, edge := range res.Media.Staff.Edges {
		if edge == nil || edge.Role == nil || edge.Node == nil || edge.Node.Name == nil || edge.Node.Name.Full == nil {
			continue
		}
		ret = append(ret, mangaStaffMember{Name: *edge.Node.Name.Full, Role: *edge.Role})
	}
	return ret, nil
}

// splitMangaStaff maps AniList staff roles onto ComicInfo's Writer and
// Penciller. AniList roles are free text ("Story & Art", "Art", "Original
// Creator", "Translator (English)"), so they are matched by keyword; support
// roles that merely mention art or story are dropped.
func splitMangaStaff(staff []mangaStaffMember) (writers string, artists string) {
	var writerNames, artistNames []string
	seenWriter := make(map[string]bool)
	seenArtist := make(map[string]bool)

	for _, member := range staff {
		name := strings.TrimSpace(member.Name)
		if name == "" {
			continue
		}

		role := strings.ToLower(member.Role)
		if i := strings.Index(role, "("); i >= 0 {
			role = role[:i]
		}
		if containsAny(role, "assistant", "touch-up", "translat", "letter", "edit", "design", "cover") {
			continue
		}

		if containsAny(role, "story", "original creator", "original work", "writer", "script") && !seenWriter[name] {
			seenWriter[name] = true
			writerNames = append(writerNames, name)
		}
		if containsAny(role, "art", "illustrat") && !seenArtist[name] {
			seenArtist[name] = true
			artistNames = append(artistNames, name)
		}
	}

	return strings.Join(writerNames, ", "), strings.Join(artistNames, ", ")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
