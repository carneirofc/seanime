package mcp

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/database/db_bridge"
	"seanime/internal/library/anime"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools wires up the read-only tools exposed to MCP clients. Input
// schemas are inferred by the SDK from the typed argument structs (json +
// jsonschema tags); fields tagged omitempty are optional.
func (s *Server) registerTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_anime",
		Description: "Search AniList for anime by title. Returns a compact list of matches (id, title, format, status, year, episodes, score).",
	}, s.searchAnime)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search_manga",
		Description: "Search AniList for manga by title. Returns a compact list of matches.",
	}, s.searchManga)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_anime",
		Description: "Get base AniList information for an anime by its media id.",
	}, s.getAnime)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_anime_details",
		Description: "Get extended AniList details for an anime (characters, relations, recommendations) by its media id.",
	}, s.getAnimeDetails)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_anime_collection",
		Description: "Get the signed-in user's anime collection (watch lists with progress, status and score).",
	}, s.getAnimeCollection)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_viewer_stats",
		Description: "Get the signed-in user's AniList viewer statistics.",
	}, s.getViewerStats)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "get_library_files",
		Description: "Get the local library contents scanned from disk. Returns mapped files " +
			"(matched to an AniList media, grouped by media id with title and episode) and unmapped " +
			"files (not yet matched, mediaId == 0). Use the filter argument to return only one group.",
	}, s.getLibraryFiles)
}

//////////////////////////////////////////////////////////////////////////////
// Tool inputs
//////////////////////////////////////////////////////////////////////////////

type searchAnimeInput struct {
	Query        string `json:"query" jsonschema:"title to search for"`
	PerPage      int    `json:"perPage,omitempty" jsonschema:"number of results to return (1-50, default 10)"`
	IncludeAdult bool   `json:"includeAdult,omitempty" jsonschema:"include adult content (default false)"`
}

type searchMangaInput struct {
	Query   string `json:"query" jsonschema:"title to search for"`
	PerPage int    `json:"perPage,omitempty" jsonschema:"number of results to return (1-50, default 10)"`
}

type mediaIDInput struct {
	MediaID int `json:"mediaId" jsonschema:"AniList media id"`
}

type libraryFilesInput struct {
	Filter string `json:"filter,omitempty" jsonschema:"which files to return: \"all\" (default), \"mapped\", or \"unmapped\""`
}

type emptyInput struct{}

//////////////////////////////////////////////////////////////////////////////
// Tool handlers
//////////////////////////////////////////////////////////////////////////////

type animeSummary struct {
	ID         int     `json:"id"`
	Title      string  `json:"title"`
	Format     string  `json:"format,omitempty"`
	Status     string  `json:"status,omitempty"`
	SeasonYear *int    `json:"seasonYear,omitempty"`
	Episodes   *int    `json:"episodes,omitempty"`
	MeanScore  *int    `json:"meanScore,omitempty"`
	SiteURL    *string `json:"siteUrl,omitempty"`
}

func (s *Server) searchAnime(ctx context.Context, _ *mcp.CallToolRequest, in searchAnimeInput) (*mcp.CallToolResult, any, error) {
	if in.Query == "" {
		return nil, nil, errors.New("query is required")
	}

	page := 1
	perPage := clampPerPage(in.PerPage)
	sort := []*anilist.MediaSort{ptr(anilist.MediaSortSearchMatch)}

	res, err := s.platform().GetAnilistClient().ListAnime(
		ctx, &page, &in.Query, &perPage, sort, nil, nil, nil, nil, nil, nil, nil, &in.IncludeAdult,
	)
	if err != nil {
		return nil, nil, err
	}

	summaries := make([]animeSummary, 0)
	if res.GetPage() != nil {
		for _, m := range res.GetPage().Media {
			summaries = append(summaries, summarizeAnime(m))
		}
	}
	return jsonResult(summaries)
}

func (s *Server) searchManga(ctx context.Context, _ *mcp.CallToolRequest, in searchMangaInput) (*mcp.CallToolResult, any, error) {
	if in.Query == "" {
		return nil, nil, errors.New("query is required")
	}

	page := 1
	perPage := clampPerPage(in.PerPage)

	res, err := s.platform().GetAnilistClient().SearchBaseManga(
		ctx, &page, &perPage, []*anilist.MediaSort{ptr(anilist.MediaSortSearchMatch)}, &in.Query, nil,
	)
	if err != nil {
		return nil, nil, err
	}

	type mangaSummary struct {
		ID      int     `json:"id"`
		Title   string  `json:"title"`
		Format  string  `json:"format,omitempty"`
		Status  string  `json:"status,omitempty"`
		SiteURL *string `json:"siteUrl,omitempty"`
	}
	summaries := make([]mangaSummary, 0)
	if res.GetPage() != nil {
		for _, m := range res.GetPage().Media {
			if m == nil {
				continue
			}
			summaries = append(summaries, mangaSummary{
				ID:      m.GetID(),
				Title:   m.GetRomajiTitleSafe(),
				Format:  derefEnum(m.Format),
				Status:  derefEnum(m.Status),
				SiteURL: m.SiteURL,
			})
		}
	}
	return jsonResult(summaries)
}

func (s *Server) getAnime(ctx context.Context, _ *mcp.CallToolRequest, in mediaIDInput) (*mcp.CallToolResult, any, error) {
	if in.MediaID <= 0 {
		return nil, nil, errors.New("mediaId is required")
	}
	anime, err := s.platform().GetAnime(ctx, in.MediaID)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(anime)
}

func (s *Server) getAnimeDetails(ctx context.Context, _ *mcp.CallToolRequest, in mediaIDInput) (*mcp.CallToolResult, any, error) {
	if in.MediaID <= 0 {
		return nil, nil, errors.New("mediaId is required")
	}
	details, err := s.platform().GetAnimeDetails(ctx, in.MediaID)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(details)
}

func (s *Server) getAnimeCollection(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	collection, err := s.platform().GetAnimeCollection(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	type entry struct {
		MediaID  int     `json:"mediaId"`
		Title    string  `json:"title"`
		Status   string  `json:"status,omitempty"`
		Progress *int    `json:"progress,omitempty"`
		Score    float64 `json:"score,omitempty"`
		Episodes *int    `json:"episodes,omitempty"`
	}
	type list struct {
		Name    string  `json:"name"`
		Status  string  `json:"status,omitempty"`
		Entries []entry `json:"entries"`
	}

	lists := make([]list, 0)
	if collection.GetMediaListCollection() != nil {
		for _, l := range collection.GetMediaListCollection().GetLists() {
			if l == nil {
				continue
			}
			entries := make([]entry, 0, len(l.Entries))
			for _, e := range l.Entries {
				if e == nil || e.Media == nil {
					continue
				}
				entries = append(entries, entry{
					MediaID:  e.Media.GetID(),
					Title:    e.Media.GetRomajiTitleSafe(),
					Status:   derefEnum(e.Status),
					Progress: e.Progress,
					Score:    deref(e.Score),
					Episodes: e.Media.Episodes,
				})
			}
			lists = append(lists, list{
				Name:    deref(l.Name),
				Status:  derefEnum(l.Status),
				Entries: entries,
			})
		}
	}
	return jsonResult(lists)
}

func (s *Server) getViewerStats(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	stats, err := s.platform().GetViewerStats(ctx)
	if err != nil {
		return nil, nil, err
	}
	return jsonResult(stats)
}

type libraryFile struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Episode int    `json:"episode,omitempty"`
	Type    string `json:"type,omitempty"`
	Locked  bool   `json:"locked,omitempty"`
}

type mappedLibraryEntry struct {
	MediaID   int           `json:"mediaId"`
	Title     string        `json:"title,omitempty"`
	FileCount int           `json:"fileCount"`
	Files     []libraryFile `json:"files"`
}

type libraryFilesResult struct {
	TotalFiles    int                  `json:"totalFiles"`
	MappedCount   int                  `json:"mappedCount"`
	UnmappedCount int                  `json:"unmappedCount"`
	Mapped        []mappedLibraryEntry `json:"mapped,omitempty"`
	Unmapped      []libraryFile        `json:"unmapped,omitempty"`
}

func (s *Server) getLibraryFiles(ctx context.Context, _ *mcp.CallToolRequest, in libraryFilesInput) (*mcp.CallToolResult, any, error) {
	if s.app.Database == nil {
		return nil, nil, errors.New("database is not available")
	}

	filter := in.Filter
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "mapped" && filter != "unmapped" {
		return nil, nil, errors.New(`filter must be one of "all", "mapped", or "unmapped"`)
	}

	lfs, _, err := db_bridge.GetLocalFiles(s.app.Database)
	if err != nil {
		return nil, nil, err
	}

	result := libraryFilesResult{TotalFiles: len(lfs)}

	// Resolve mapped media titles from the user's anime collection (best effort).
	titles := s.libraryTitleLookup(ctx)

	grouped := make(map[int]*mappedLibraryEntry)
	for _, lf := range lfs {
		if lf == nil {
			continue
		}
		if lf.MediaId == 0 {
			result.UnmappedCount++
			if filter == "mapped" {
				continue
			}
			result.Unmapped = append(result.Unmapped, toLibraryFile(lf))
			continue
		}

		result.MappedCount++
		if filter == "unmapped" {
			continue
		}
		entry, ok := grouped[lf.MediaId]
		if !ok {
			entry = &mappedLibraryEntry{MediaID: lf.MediaId, Title: titles[lf.MediaId]}
			grouped[lf.MediaId] = entry
		}
		entry.Files = append(entry.Files, toLibraryFile(lf))
	}

	if filter != "unmapped" {
		result.Mapped = make([]mappedLibraryEntry, 0, len(grouped))
		for _, entry := range grouped {
			entry.FileCount = len(entry.Files)
			sort.Slice(entry.Files, func(i, j int) bool { return entry.Files[i].Path < entry.Files[j].Path })
			result.Mapped = append(result.Mapped, *entry)
		}
		sort.Slice(result.Mapped, func(i, j int) bool {
			if result.Mapped[i].Title != result.Mapped[j].Title {
				return result.Mapped[i].Title < result.Mapped[j].Title
			}
			return result.Mapped[i].MediaID < result.Mapped[j].MediaID
		})
	}

	if filter != "mapped" {
		sort.Slice(result.Unmapped, func(i, j int) bool { return result.Unmapped[i].Path < result.Unmapped[j].Path })
	}

	return jsonResult(result)
}

// libraryTitleLookup builds a mediaId -> romaji title map from the user's anime
// collection. It is best effort: on error it returns an empty map so library
// files are still returned (without titles).
func (s *Server) libraryTitleLookup(ctx context.Context) map[int]string {
	titles := make(map[int]string)
	collection, err := s.platform().GetAnimeCollection(ctx, false)
	if err != nil || collection.GetMediaListCollection() == nil {
		return titles
	}
	for _, l := range collection.GetMediaListCollection().GetLists() {
		if l == nil {
			continue
		}
		for _, e := range l.Entries {
			if e == nil || e.Media == nil {
				continue
			}
			titles[e.Media.GetID()] = e.Media.GetRomajiTitleSafe()
		}
	}
	return titles
}

func toLibraryFile(lf *anime.LocalFile) libraryFile {
	f := libraryFile{
		Path:   lf.Path,
		Name:   lf.Name,
		Locked: lf.Locked,
	}
	if lf.Metadata != nil {
		f.Episode = lf.Metadata.Episode
		f.Type = string(lf.Metadata.Type)
	}
	return f
}

//////////////////////////////////////////////////////////////////////////////
// Shared helpers
//////////////////////////////////////////////////////////////////////////////

func summarizeAnime(m *anilist.BaseAnime) animeSummary {
	if m == nil {
		return animeSummary{}
	}
	return animeSummary{
		ID:         m.GetID(),
		Title:      m.GetRomajiTitleSafe(),
		Format:     derefEnum(m.Format),
		Status:     derefEnum(m.Status),
		SeasonYear: m.SeasonYear,
		Episodes:   m.Episodes,
		MeanScore:  m.MeanScore,
		SiteURL:    m.SiteURL,
	}
}
