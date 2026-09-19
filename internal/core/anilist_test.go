package core

import (
	"seanime/internal/api/anilist"
	"seanime/internal/database/db"
	"seanime/internal/database/models"
	"seanime/internal/events"
	"seanime/internal/platforms/platform"
	"seanime/internal/testmocks"
	"seanime/internal/util"
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestShouldRefreshViewer(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time {
		ts := now.Add(-d)
		return &ts
	}

	tests := []struct {
		name        string
		lastUpdated *time.Time
		want        bool
	}{
		// Rows written before the column was used carry no timestamp, so they are the ones
		// most in need of a refresh.
		{"never recorded", nil, true},
		{"just refreshed", ago(0), false},
		{"inside the interval", ago(59 * time.Minute), false},
		{"exactly at the interval", ago(time.Hour), true},
		{"well past the interval", ago(6 * time.Hour), true},
		// A clock change must not turn the throttle into a permanent refresh loop.
		{"stamped in the future", func() *time.Time { ts := now.Add(time.Hour); return &ts }(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRefreshViewer(tt.lastUpdated, now, viewerRefreshInterval); got != tt.want {
				t.Fatalf("shouldRefreshViewer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRefreshViewerNoOpsWhenOffline(t *testing.T) {
	// Every other field is nil: the offline guard has to return before anything is touched.
	app := &App{isOfflineRef: util.NewRef(true)}

	if err := app.RefreshViewer(true); err != nil {
		t.Fatalf("RefreshViewer() while offline returned %v, want nil", err)
	}
}

func TestRefreshViewerNoOpsDuringLogout(t *testing.T) {
	// A refresh landing mid-logout would resurrect the account row it just blanked.
	app := &App{isOfflineRef: util.NewRef(false)}
	app.logoutInProgress.Store(true)

	if err := app.RefreshViewer(true); err != nil {
		t.Fatalf("RefreshViewer() during logout returned %v, want nil", err)
	}
}

func TestRefreshViewerNoOpsWithoutAnilistClient(t *testing.T) {
	app := &App{isOfflineRef: util.NewRef(false)}

	if err := app.RefreshViewer(true); err != nil {
		t.Fatalf("RefreshViewer() without a client returned %v, want nil", err)
	}
}

// newViewerTestApp builds an App with just enough wired up to persist a viewer.
func newViewerTestApp(t *testing.T) *App {
	t.Helper()

	logger := util.NewLogger()
	database, err := db.NewDatabase(t.TempDir(), "viewer_test", logger)
	if err != nil {
		t.Fatalf("could not create the test database: %v", err)
	}

	var fakePlatform platform.Platform = testmocks.NewFakePlatformBuilder().Build()

	return &App{
		Database:           database,
		Logger:             logger,
		WSEventManager:     events.NewWSEventManager(logger),
		AnilistPlatformRef: util.NewRef(fakePlatform),
		isOfflineRef:       util.NewRef(false),
	}
}

func testViewer(name, avatar string) *anilist.GetViewer_Viewer {
	return &anilist.GetViewer_Viewer{
		Name:   name,
		Avatar: &anilist.GetViewer_Viewer_Avatar{Large: &avatar},
	}
}

// seedAccount writes the starting account row. It goes through UpsertAccount so the package
// level account cache in db matches the row, as it would at runtime.
func seedAccount(t *testing.T, app *App, viewer *anilist.GetViewer_Viewer, token string) *models.Account {
	t.Helper()

	acc, err := app.Database.UpsertAccount(&models.Account{
		BaseModel: models.BaseModel{ID: 1},
		Username:  viewer.Name,
		Token:     token,
		Viewer:    mustMarshalViewer(t, viewer),
	})
	if err != nil {
		t.Fatalf("could not seed the account: %v", err)
	}
	return acc
}

func mustMarshalViewer(t *testing.T, viewer *anilist.GetViewer_Viewer) []byte {
	t.Helper()

	b, err := json.Marshal(viewer)
	if err != nil {
		t.Fatalf("could not marshal the viewer: %v", err)
	}
	return b
}

func TestPersistRefreshedViewerKeepsToken(t *testing.T) {
	// UpsertAccount overwrites every column, so a refresh that forgot to carry the token
	// through would silently sign the user out.
	app := newViewerTestApp(t)
	acc := seedAccount(t, app, testViewer("hikari", "https://cdn/avatar/old.png"), "secret-token")

	changed, err := app.persistRefreshedViewer(acc, testViewer("hikari", "https://cdn/avatar/new.png"))
	if err != nil {
		t.Fatalf("persistRefreshedViewer() returned %v", err)
	}
	if !changed {
		t.Fatal("a changed avatar was reported as unchanged")
	}

	stored, err := app.Database.GetAccount()
	if err != nil {
		t.Fatalf("could not read the account back: %v", err)
	}
	if stored.Token != "secret-token" {
		t.Fatalf("stored token = %q, want secret-token", stored.Token)
	}
	if stored.ViewerUpdatedAt == nil {
		t.Fatal("ViewerUpdatedAt was not stamped, so the refresh would never throttle")
	}
}

func TestPersistRefreshedViewerReportsUnchanged(t *testing.T) {
	// An unchanged profile must still advance the throttle, or every tick refetches.
	app := newViewerTestApp(t)
	viewer := testViewer("hikari", "https://cdn/avatar/old.png")
	acc := seedAccount(t, app, viewer, "secret-token")

	changed, err := app.persistRefreshedViewer(acc, testViewer("hikari", "https://cdn/avatar/old.png"))
	if err != nil {
		t.Fatalf("persistRefreshedViewer() returned %v", err)
	}
	if changed {
		t.Fatal("an identical viewer was reported as changed")
	}

	stored, err := app.Database.GetAccount()
	if err != nil {
		t.Fatalf("could not read the account back: %v", err)
	}
	if stored.ViewerUpdatedAt == nil {
		t.Fatal("ViewerUpdatedAt was not stamped for an unchanged viewer")
	}
}

func TestPersistRefreshedViewerUpdatesUsername(t *testing.T) {
	app := newViewerTestApp(t)
	acc := seedAccount(t, app, testViewer("hikari", "https://cdn/avatar/old.png"), "secret-token")

	if _, err := app.persistRefreshedViewer(acc, testViewer("akari", "https://cdn/avatar/old.png")); err != nil {
		t.Fatalf("persistRefreshedViewer() returned %v", err)
	}

	if got := app.GetUsername(); got != "akari" {
		t.Fatalf("GetUsername() = %q, want akari", got)
	}

	stored, err := app.Database.GetAccount()
	if err != nil {
		t.Fatalf("could not read the account back: %v", err)
	}
	if stored.Username != "akari" {
		t.Fatalf("stored username = %q, want akari", stored.Username)
	}
}
