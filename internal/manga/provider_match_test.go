package manga

import (
	"testing"

	hibikemanga "seanime/internal/extension/hibike/manga"
)

func TestDedupeAndSortProviderMatches(t *testing.T) {
	t.Parallel()

	results := dedupeAndSortProviderMatches("mangadex", []*hibikemanga.SearchResult{
		{ID: "b", Title: "Beta", SearchRating: 0.65},
		{ID: "a", Title: "Alpha", SearchRating: 0.9},
		{ID: "a", Title: "Alpha", SearchRating: 0.7},
		{ID: "c", Title: "Gamma", SearchRating: 0.9},
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 unique results, got %d", len(results))
	}

	if results[0].ID != "a" || results[1].ID != "c" || results[2].ID != "b" {
		t.Fatalf("unexpected order: %#v", []string{results[0].ID, results[1].ID, results[2].ID})
	}

	if results[0].SearchRating != 0.9 {
		t.Fatalf("expected dedupe to keep highest rating, got %f", results[0].SearchRating)
	}

	for _, res := range results {
		if res.Provider != "mangadex" {
			t.Fatalf("expected provider override to be applied, got %q", res.Provider)
		}
	}
}
