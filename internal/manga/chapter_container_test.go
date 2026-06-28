package manga

import (
	"testing"

	hibikemanga "seanime/internal/extension/hibike/manga"
)

func TestAttachAlternativeProvidersToContainer_UsesExactMatchesWhenAvailable(t *testing.T) {
	t.Parallel()

	primary := &ChapterContainer{
		Provider: "comick",
		Chapters: []*hibikemanga.ChapterDetails{
			{
				Provider:  "comick",
				ID:        "c-1",
				Chapter:   "001",
				Language:  "en",
				Scanlator: "group-a",
			},
		},
	}

	alternatives := []*ChapterContainer{
		{
			Provider: "weebcentral",
			Chapters: []*hibikemanga.ChapterDetails{
				{
					Provider:  "weebcentral",
					ID:        "w-1",
					Chapter:   "1",
					Language:  "fr",
					Scanlator: "group-a",
				},
			},
		},
		{
			Provider: "mangadex",
			Chapters: []*hibikemanga.ChapterDetails{
				{
					Provider:  "mangadex",
					ID:        "m-1",
					Chapter:   "1",
					Language:  "en",
					Scanlator: "group-a",
				},
			},
		},
	}

	attachAlternativeProvidersToContainer(primary, alternatives)

	got := primary.Chapters[0].AlternativeProviders
	if len(got) != 1 {
		t.Fatalf("expected 1 exact alternative, got %d", len(got))
	}

	if got[0].Provider != "mangadex" || got[0].ChapterID != "m-1" {
		t.Fatalf("unexpected alternative provider: %#v", got[0])
	}
}

func TestAttachAlternativeProvidersToContainer_FallsBackToChapterNumber(t *testing.T) {
	t.Parallel()

	primary := &ChapterContainer{
		Provider: "comick",
		Chapters: []*hibikemanga.ChapterDetails{
			{
				Provider:  "comick",
				ID:        "c-3",
				Chapter:   "003",
				Language:  "en",
				Scanlator: "group-a",
			},
		},
	}

	alternatives := []*ChapterContainer{
		{
			Provider: "weebcentral",
			Chapters: []*hibikemanga.ChapterDetails{
				{
					Provider: "weebcentral",
					ID:       "w-3",
					Chapter:  "3",
				},
			},
		},
	}

	attachAlternativeProvidersToContainer(primary, alternatives)

	got := primary.Chapters[0].AlternativeProviders
	if len(got) != 1 {
		t.Fatalf("expected 1 fallback alternative, got %d", len(got))
	}

	if got[0].Provider != "weebcentral" || got[0].ChapterID != "w-3" {
		t.Fatalf("unexpected fallback alternative: %#v", got[0])
	}
}

func TestAttachAlternativeProvidersToContainer_DedupesAndSortsOptions(t *testing.T) {
	t.Parallel()

	primary := &ChapterContainer{
		Provider: "comick",
		Chapters: []*hibikemanga.ChapterDetails{
			{
				Provider: "comick",
				ID:       "c-5",
				Chapter:  "5",
			},
		},
	}

	alternatives := []*ChapterContainer{
		{
			Provider: "z-provider",
			Chapters: []*hibikemanga.ChapterDetails{
				{Provider: "z-provider", ID: "z-5", Chapter: "5"},
				{Provider: "z-provider", ID: "z-5", Chapter: "5"},
			},
		},
		{
			Provider: "a-provider",
			Chapters: []*hibikemanga.ChapterDetails{
				{Provider: "a-provider", ID: "a-5", Chapter: "5"},
			},
		},
	}

	attachAlternativeProvidersToContainer(primary, alternatives)

	got := primary.Chapters[0].AlternativeProviders
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped alternatives, got %d", len(got))
	}

	if got[0].Provider != "a-provider" || got[1].Provider != "z-provider" {
		t.Fatalf("expected sorted providers, got %#v", got)
	}
}

func TestNormalizeChapterProvider_PreservesSourceProvider(t *testing.T) {
	t.Parallel()

	chapter := &hibikemanga.ChapterDetails{
		Provider: "mirror-a",
		ID:       "chapter-1",
	}

	hibikemanga.NormalizeChapterProvider(chapter, "mangapark")

	if chapter.Provider != "mangapark" {
		t.Fatalf("expected normalized provider, got %q", chapter.Provider)
	}

	if chapter.SourceProvider != "mirror-a" {
		t.Fatalf("expected source provider to be preserved, got %q", chapter.SourceProvider)
	}
}
