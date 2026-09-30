package manga

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type fakeDownloadMetadataPlatform struct {
	manga   *anilist.BaseManga
	details *anilist.MangaDetailsById_Media
}

func (f *fakeDownloadMetadataPlatform) GetManga(context.Context, int) (*anilist.BaseManga, error) {
	if f.manga == nil {
		return nil, errors.New("not found")
	}
	return f.manga, nil
}

func (f *fakeDownloadMetadataPlatform) GetMangaDetails(context.Context, int) (*anilist.MangaDetailsById_Media, error) {
	if f.details == nil {
		return nil, errors.New("not found")
	}
	return f.details, nil
}

func (f *fakeDownloadMetadataPlatform) GetAnilistClient() anilist.AnilistClient {
	return nil
}

func ptr[T any](v T) *T { return &v }

func TestResolveDownloadSeriesMetadata(t *testing.T) {
	mainRole := anilist.CharacterRoleMain
	supportingRole := anilist.CharacterRoleSupporting
	p := &fakeDownloadMetadataPlatform{
		manga: &anilist.BaseManga{
			ID:              1,
			Title:           &anilist.BaseManga_Title{UserPreferred: ptr("Series Title")},
			Description:     ptr("First line.<br>Second <i>line</i>."),
			SiteURL:         ptr("https://anilist.co/manga/1"),
			Chapters:        ptr(120),
			StartDate:       &anilist.BaseManga_StartDate{Year: ptr(2020)},
			IsAdult:         ptr(false),
			CountryOfOrigin: ptr("JP"),
			Genres:          []*string{ptr("Action"), ptr("Drama")},
		},
		details: &anilist.MangaDetailsById_Media{
			Tags: []*anilist.MTagsSlim{
				{Name: "Shounen"},
				{Name: "Twist Ending", IsMediaSpoiler: ptr(true)},
				{Name: "Swordplay"},
			},
			Characters: &anilist.MangaDetailsById_Media_Characters{
				Edges: []*anilist.MangaDetailsById_Media_Characters_Edges{
					{Role: &mainRole, Node: &anilist.BaseCharacter{Name: &anilist.BaseCharacter_Name{Full: ptr("Hero")}}},
					{Role: &supportingRole, Node: &anilist.BaseCharacter{Name: &anilist.BaseCharacter_Name{Full: ptr("Sidekick")}}},
				},
			},
		},
	}

	logger := zerolog.Nop()
	series := ResolveDownloadSeriesMetadata(context.Background(), p, nil, 1, &logger)

	require.Equal(t, "Series Title", series.Title)
	require.Equal(t, "First line.\nSecond line.", series.Metadata.Summary)
	require.Equal(t, "Action, Drama", series.Metadata.Genre)
	require.Equal(t, "Shounen, Swordplay", series.Metadata.Tags)
	require.Equal(t, "Hero", series.Metadata.Characters)
	require.Equal(t, 120, series.Metadata.Count)
	require.Equal(t, 2020, series.Metadata.Year)
	require.Equal(t, "https://anilist.co/manga/1", series.Metadata.Web)
	// Original language is not the language of a scraped translation.
	require.Empty(t, series.Metadata.LanguageISO)
}

func TestResolveDownloadSeriesMetadataToleratesFailures(t *testing.T) {
	logger := zerolog.Nop()
	series := ResolveDownloadSeriesMetadata(context.Background(), &fakeDownloadMetadataPlatform{}, nil, 1, &logger)
	require.NotNil(t, series)
	require.Empty(t, series.Title)
}

func TestChapterDownloadMetadata(t *testing.T) {
	series := &DownloadSeriesMetadata{Title: "Series Title"}
	series.Metadata.Web = "https://anilist.co/manga/1"
	series.Metadata.Tags = "Shounen"

	got := chapterDownloadMetadata(series, "comick", &hibikemanga.ChapterDetails{
		ID:        "ch-1",
		URL:       "https://example.com/ch-1",
		Scanlator: " Scan Group ",
		Language:  "EN",
	})

	require.Equal(t, "Scan Group", got.Translator)
	require.Equal(t, "en", got.LanguageISO)
	require.Equal(t, "https://example.com/ch-1 https://anilist.co/manga/1", got.Web)
	require.Equal(t, "Shounen", got.Tags)
	require.Contains(t, got.Notes, "comick")
	// The series metadata is shared across chapters and must not be mutated.
	require.Equal(t, "https://anilist.co/manga/1", series.Metadata.Web)

	require.NotNil(t, chapterDownloadMetadata(nil, "comick", &hibikemanga.ChapterDetails{ID: "ch-2"}))
}

func TestSplitMangaStaff(t *testing.T) {
	writers, artists := splitMangaStaff([]mangaStaffMember{
		{Name: "Mangaka", Role: "Story & Art"},
		{Name: "Author", Role: "Original Story"},
		{Name: "Illustrator", Role: "Art"},
		{Name: "Helper", Role: "Assistant"},
		{Name: "Retoucher", Role: "Touch-up Art & Lettering"},
		{Name: "Localizer", Role: "Translator (English)"},
		{Name: "Mangaka", Role: "Original Creator"},
	})

	require.Equal(t, "Mangaka, Author", writers)
	require.Equal(t, "Mangaka, Illustrator", artists)
}
