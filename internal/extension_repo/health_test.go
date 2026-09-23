package extension_repo

import (
	"context"
	"errors"
	"path/filepath"
	"seanime/internal/events"
	"seanime/internal/extension"
	hibikemanga "seanime/internal/extension/hibike/manga"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthTrackerThreshold(t *testing.T) {
	var fired atomic.Int32
	h := NewHealthTracker(func(string, string) { fired.Add(1) })

	for i := range failingThreshold - 1 {
		require.Falsef(t, h.Record("ext", errors.New("boom")), "call %d", i)
	}
	require.True(t, h.Record("ext", errors.New("boom")))
	// Further failures do not fire the transition again
	require.False(t, h.Record("ext", errors.New("boom")))

	snap := h.Snapshot()["ext"]
	require.True(t, snap.Failing)
	require.Equal(t, failingThreshold+1, snap.ConsecutiveFailures)
	require.Equal(t, "boom", snap.LastError)
	require.Eventually(t, func() bool { return fired.Load() == 1 }, time.Second, 10*time.Millisecond)

	// A success clears the failing state but keeps the totals
	require.False(t, h.Record("ext", nil))
	snap = h.Snapshot()["ext"]
	require.False(t, snap.Failing)
	require.Zero(t, snap.ConsecutiveFailures)
	require.Equal(t, failingThreshold+1, snap.Failures)
	require.Equal(t, failingThreshold+2, snap.Calls)
	require.NotNil(t, snap.LastSuccessAt)
}

func TestHealthTrackerIgnoresCancellation(t *testing.T) {
	h := NewHealthTracker(nil)
	for range failingThreshold * 2 {
		h.Record("ext", context.Canceled)
		h.Record("ext", errors.Join(errors.New("wrapped"), context.Canceled))
	}
	require.Empty(t, h.Snapshot())
}

func TestHealthTrackerResetAndNil(t *testing.T) {
	h := NewHealthTracker(nil)
	h.Record("ext", errors.New("boom"))
	h.Reset("ext")
	require.Empty(t, h.Snapshot())

	var nilTracker *HealthTracker
	require.False(t, nilTracker.Record("ext", errors.New("boom")))
	nilTracker.Reset("ext")
	require.Empty(t, nilTracker.Snapshot())
}

// failingMangaExtension is a manga provider whose search always throws.
func failingMangaExtension() extension.Extension {
	ext := testExternalExtension()
	ext.ID = "failing-provider"
	ext.Payload = `
class Provider {
  getSettings() { return { supportsMultiScanlator: false, supportsMultiLanguage: false } }
  async search() { throw new Error("source is down") }
  async findChapters() { return [] }
  async findChapterPages() { return [] }
}`
	return ext
}

func loadFailingMangaExtension(t *testing.T, repo *Repository, extensionDir string) hibikemanga.Provider {
	t.Helper()
	ext := failingMangaExtension()
	writeTestExternalExtension(t, extensionDir, ext)
	repo.loadExternalExtension(filepath.Join(extensionDir, ext.ID+".json"))

	provider, ok := repo.GetMangaProviderExtensionByID(ext.ID)
	require.True(t, ok, "extension should load")
	return provider.GetProvider()
}

func failingEvents(repo *Repository) []ExtensionFailingEvent {
	var ret []ExtensionFailingEvent
	for _, e := range repo.wsEventManager.(*events.MockWSEventManager).Events() {
		if e.Type == events.ExtensionFailing {
			ret = append(ret, e.Payload.(ExtensionFailingEvent))
		}
	}
	return ret
}

func TestFailingExtensionIsReportedWithoutAutoDisable(t *testing.T) {
	repo, extensionDir := newExternalExtensionTestRepository(t)
	provider := loadFailingMangaExtension(t, repo, extensionDir)

	for range failingThreshold {
		_, err := provider.Search(hibikemanga.SearchOptions{Query: "x"})
		require.ErrorContains(t, err, "source is down")
	}

	require.Eventually(t, func() bool { return len(failingEvents(repo)) == 1 }, 2*time.Second, 10*time.Millisecond)
	require.False(t, failingEvents(repo)[0].AutoDisabled)

	all := repo.GetAllExtensions(false)
	require.True(t, all.Health["failing-provider"].Failing)
	require.Empty(t, all.DisabledExtensions)
}

func TestFailingExtensionIsAutoDisabled(t *testing.T) {
	repo, extensionDir := newExternalExtensionTestRepository(t)
	require.NoError(t, repo.SetAutoDisableFailing(true))
	provider := loadFailingMangaExtension(t, repo, extensionDir)

	for range failingThreshold {
		_, _ = provider.Search(hibikemanga.SearchOptions{Query: "x"})
	}

	require.Eventually(t, func() bool { return len(failingEvents(repo)) == 1 }, 2*time.Second, 10*time.Millisecond)
	require.True(t, failingEvents(repo)[0].AutoDisabled)

	all := repo.GetAllExtensions(false)
	require.True(t, all.AutoDisableFailing)
	require.Len(t, all.DisabledExtensions, 1)
	// Reloading (here: disabling) clears the health record
	require.NotContains(t, all.Health, "failing-provider")
}
