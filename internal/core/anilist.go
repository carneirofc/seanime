package core

import (
	"context"
	"errors"
	"seanime/internal/api/anilist"
	"seanime/internal/database/models"
	"seanime/internal/events"
	"seanime/internal/platforms/anilist_platform"
	"seanime/internal/platforms/platform"
	"seanime/internal/platforms/simulated_platform"
	"seanime/internal/user"
	"seanime/internal/util"
	"slices"
	"time"

	"github.com/goccy/go-json"
)

// GetUser returns the currently logged-in user or a simulated one.
func (a *App) GetUser() *user.User {
	if a.user == nil {
		return user.NewSimulatedUser()
	}
	return a.user
}

// GetUsername returns the username of the currently logged-in user
func (a *App) GetUsername() string {
	if a.user == nil {
		return ""
	}
	if a.user.Viewer == nil {
		return ""
	}
	return a.user.Viewer.GetName()
}

func (a *App) GetUserAnilistToken() string {
	if a.user == nil || a.user.Token == user.SimulatedUserToken {
		return ""
	}

	return a.user.Token
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// UpdatePlatform changes the current platform to the provided one.
func (a *App) UpdatePlatform(platform platform.Platform) {
	if a.AnilistPlatformRef.IsPresent() {
		a.AnilistPlatformRef.Get().Close()
	}
	a.AnilistPlatformRef.Set(platform)
	a.AddOnRefreshAnilistCollectionFunc("anilist-platform", func() {
		a.AnilistPlatformRef.Get().ClearCache()
	})
}

// UpdateAnilistClientToken will update the Anilist Client Wrapper token.
// This function should be called when a user logs in
func (a *App) UpdateAnilistClientToken(token string) {
	ac := anilist.NewAnilistClient(token, a.AnilistCacheDir)
	a.AnilistClientRef.Set(ac)
}

func (a *App) UseOfficialAnilistClient() error {
	previousProvider := anilist.CurrentRequestProvider()
	anilist.UseOfficialAPI()

	if err := a.applyRuntimeAnilistClient(anilist.NewAnilistClient(a.Database.GetAnilistToken(), a.AnilistCacheDir)); err != nil {
		_ = anilist.SetRequestProvider(previousProvider)
		return err
	}

	return nil
}

func (a *App) UseCustomAnimeClient(config anilist.CustomClientConfig) error {
	previousProvider := anilist.CurrentRequestProvider()
	if err := anilist.UseCustomAPI(config); err != nil {
		return err
	}

	if err := a.applyRuntimeAnilistClient(anilist.NewAnilistClient(config.Token, a.AnilistCacheDir)); err != nil {
		_ = anilist.SetRequestProvider(previousProvider)
		return err
	}

	return nil
}

func (a *App) applyRuntimeAnilistClient(client anilist.AnilistClient) error {
	if a.IsOffline() {
		return errors.New("anilist runtime switch is unavailable in offline mode")
	}

	nextUser := user.NewSimulatedUser()
	provider := anilist.CurrentRequestProviderName()

	if client.IsAuthenticated() {
		viewer, err := fetchRuntimeViewer(provider, func() (*anilist.GetViewer, error) {
			return client.GetViewer(context.Background())
		}, time.Sleep)
		if err != nil {
			return err
		}

		token := ""
		if provider == anilist.OfficialRequestProviderName {
			token = a.Database.GetAnilistToken()
		}

		nextUser = &user.User{
			Viewer: viewer.GetViewer(),
			Token:  token,
		}
	}

	a.AnilistClientRef.Set(client)

	var nextPlatform platform.Platform
	var err error
	if client.IsAuthenticated() {
		nextPlatform = anilist_platform.NewAnilistPlatform(a.AnilistClientRef, a.ExtensionBankRef, a.Logger, a.Database, a.LogoutFromAnilist)
	} else {
		nextPlatform, err = simulated_platform.NewSimulatedPlatform(a.LocalManager, a.AnilistClientRef, a.ExtensionBankRef, a.Logger, a.Database)
		if err != nil {
			return err
		}
	}

	a.UpdatePlatform(nextPlatform)
	a.user = nextUser
	a.AnilistPlatformRef.Get().SetUsername(nextUser.Viewer.Name)
	a.InitOrRefreshModules()

	if a.DiscordPresence != nil {
		a.DiscordPresence.SetUsername(nextUser.Viewer.Name)
	}

	if _, err := a.RefreshAnimeCollection(); err != nil {
		return err
	}

	if _, err := a.RefreshMangaCollection(); err != nil {
		return err
	}

	return nil
}

var customClientViewerRetryDelays = []time.Duration{
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
	3 * time.Second,
}

func fetchRuntimeViewer(provider string, getViewer func() (*anilist.GetViewer, error), sleep func(time.Duration)) (*anilist.GetViewer, error) {
	viewer, err := getViewer()
	if err == nil || provider == anilist.OfficialRequestProviderName {
		return viewer, err
	}

	for _, delay := range customClientViewerRetryDelays {
		sleep(delay)
		viewer, err = getViewer()
		if err == nil {
			return viewer, nil
		}
	}

	return nil, err
}

// viewerRefreshInterval bounds how often the stored viewer is re-fetched from AniList.
// The account row is written at login and read back by every /api/v1/status call, so without
// a refresh an avatar, banner or username changed on AniList never reaches the client. The
// interval is deliberate: the collection cron driving this runs every ten minutes, and cutting
// AniList requests to stay under the rate limit is a recurring concern in this client.
const viewerRefreshInterval = time.Hour

// shouldRefreshViewer reports whether a viewer stored at lastUpdated is stale enough to refetch.
// A nil lastUpdated means the viewer was stored before the timestamp was recorded, so it is
// always considered stale. A timestamp in the future is left alone rather than trusted.
func shouldRefreshViewer(lastUpdated *time.Time, now time.Time, interval time.Duration) bool {
	if lastUpdated == nil {
		return true
	}
	if lastUpdated.After(now) {
		return false
	}
	return now.Sub(*lastUpdated) >= interval
}

// RefreshViewer refetches the AniList viewer and persists it, so a profile picture, banner or
// username changed on AniList appears without the user signing out and back in. Unless force is
// set, it does nothing until viewerRefreshInterval has passed since the last fetch.
func (a *App) RefreshViewer(force bool) error {
	if a.IsOffline() || a.GetUser().IsSimulated {
		return nil
	}

	// A custom AniList API serves a viewer that belongs to a different service, while the
	// stored token is still the official one. applyRuntimeAnilistClient keeps that viewer in
	// memory only, so persisting it here would mislabel the account row.
	if anilist.CurrentRequestProviderName() != anilist.OfficialRequestProviderName {
		return nil
	}

	// Advisory: a logout starting right after this passes would be undone by the upsert below.
	// The window needs a whole AniList round-trip to land inside it, and the next logout blanks
	// the row again. Taking the flag instead would make a refresh swallow a user's logout.
	if a.logoutInProgress.Load() {
		return nil
	}

	if !a.AnilistClientRef.IsPresent() || !a.AnilistClientRef.Get().IsAuthenticated() {
		return nil
	}

	acc, err := a.Database.GetAccount()
	if err != nil || acc == nil || acc.Token == "" {
		return nil
	}

	if !force && !shouldRefreshViewer(acc.ViewerUpdatedAt, time.Now(), viewerRefreshInterval) {
		return nil
	}

	// The raw client rather than the platform's cache layer: that layer serves the viewer from
	// an unkeyed cache entry, and logs the user out on any error whose text resembles an auth
	// failure. A background refresh must not be able to end the session.
	getViewer, err := a.AnilistClientRef.Get().GetViewer(context.Background())
	if err != nil {
		a.Logger.Warn().Err(err).Msg("app: Could not refresh the AniList viewer")
		return err
	}

	if getViewer == nil || getViewer.Viewer == nil || len(getViewer.Viewer.Name) == 0 {
		return errors.New("could not find user")
	}

	_, err = a.persistRefreshedViewer(acc, getViewer.Viewer)
	return err
}

// persistRefreshedViewer stores a freshly fetched viewer under the account's existing token and
// propagates it, reporting whether it differed from the stored one. It is separate from
// RefreshViewer so the persistence can be exercised without an AniList client.
func (a *App) persistRefreshedViewer(acc *models.Account, viewer *anilist.GetViewer_Viewer) (bool, error) {
	viewerBytes, err := json.Marshal(viewer)
	if err != nil {
		a.Logger.Warn().Err(err).Msg("app: Could not marshal the AniList viewer")
		return false, err
	}

	changed := !slices.Equal(viewerBytes, acc.Viewer)
	previousName := acc.Username
	now := time.Now()

	// UpsertAccount overwrites every column, so the stored token has to be carried forward.
	// acc is the shared account cache entry, so build a new row rather than mutating it.
	if _, err = a.Database.UpsertAccount(&models.Account{
		BaseModel: models.BaseModel{
			ID:        1,
			UpdatedAt: now,
		},
		Username:        viewer.Name,
		Token:           acc.Token,
		Viewer:          viewerBytes,
		ViewerUpdatedAt: &now,
	}); err != nil {
		return false, err
	}

	// The timestamp is bumped either way, so an unchanged profile still resets the interval.
	if !changed {
		return false, nil
	}

	a.user = &user.User{Viewer: viewer, Token: acc.Token}

	if previousName != viewer.Name {
		a.AnilistPlatformRef.Get().SetUsername(viewer.Name)
		if a.DiscordPresence != nil {
			a.DiscordPresence.SetUsername(viewer.Name)
		}
	}

	a.Logger.Info().Msg("app: Refreshed the AniList viewer")
	a.WSEventManager.SendEvent(events.InvalidateQueries, []string{events.GetStatusEndpoint})

	return true, nil
}

func (a *App) LoginToAnilist(token string) error {
	if token == "" {
		return errors.New("token is empty")
	}

	a.UpdateAnilistClientToken(token)

	getViewer, err := a.AnilistClientRef.Get().GetViewer(context.Background())
	if err != nil {
		a.Logger.Error().Msg("Could not authenticate to AniList")
		return err
	}

	if len(getViewer.Viewer.Name) == 0 {
		return errors.New("could not find user")
	}

	bytes, err := json.Marshal(getViewer.Viewer)
	if err != nil {
		a.Logger.Err(err).Msg("scan: could not save local files")
	}

	loggedInAt := time.Now()
	_, err = a.Database.UpsertAccount(&models.Account{
		BaseModel: models.BaseModel{
			ID:        1,
			UpdatedAt: loggedInAt,
		},
		Username:        getViewer.Viewer.Name,
		Token:           token,
		Viewer:          bytes,
		ViewerUpdatedAt: &loggedInAt,
	})
	if err != nil {
		return err
	}

	a.Logger.Info().Msg("app: Authenticated to AniList")

	anilistPlatform := anilist_platform.NewAnilistPlatform(a.AnilistClientRef, a.ExtensionBankRef, a.Logger, a.Database, a.LogoutFromAnilist)
	a.UpdatePlatform(anilistPlatform)

	a.InitOrRefreshAnilistData()
	a.InitOrRefreshModules()

	go func() {
		defer util.HandlePanicThen(func() {})
		a.InitOrRefreshTorrentstreamSettings()
		a.InitOrRefreshMediastreamSettings()
		a.InitOrRefreshDebridSettings()
	}()

	return nil
}

// LogoutFromAnilist clears the AniList token and switches to the simulated platform.
// This is called internally when the token is detected as invalid.
func (a *App) LogoutFromAnilist() {
	// prevent multiple concurrent calls (e.g. from parallel failing requests)
	if !a.logoutInProgress.CompareAndSwap(false, true) {
		return
	}
	defer a.logoutInProgress.Store(false)

	a.UpdateAnilistClientToken("")

	simulatedPlatform, err := simulated_platform.NewSimulatedPlatform(a.LocalManager, a.AnilistClientRef, a.ExtensionBankRef, a.Logger, a.Database)
	if err != nil {
		a.Logger.Error().Err(err).Msg("app: Failed to create simulated platform during auto-logout")
	} else {
		a.UpdatePlatform(simulatedPlatform)
	}

	_, _ = a.Database.UpsertAccount(&models.Account{
		BaseModel: models.BaseModel{
			ID:        1,
			UpdatedAt: time.Now(),
		},
		Username:        "",
		Token:           "",
		Viewer:          nil,
		ViewerUpdatedAt: nil,
	})

	a.Logger.Debug().Msg("app: Logged out from AniList, switched to simulated platform")

	a.InitOrRefreshModules()
	a.InitOrRefreshAnilistData()
}

// GetAnimeCollection returns the user's Anilist collection if it in the cache, otherwise it queries Anilist for the user's collection.
// When bypassCache is true, it will always query Anilist for the user's collection
func (a *App) GetAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatformRef.Get().GetAnimeCollection(context.Background(), bypassCache)
}

// GetRawAnimeCollection is the same as GetAnimeCollection but returns the raw collection that includes custom lists
func (a *App) GetRawAnimeCollection(bypassCache bool) (*anilist.AnimeCollection, error) {
	return a.AnilistPlatformRef.Get().GetRawAnimeCollection(context.Background(), bypassCache)
}

func (a *App) SyncAnilistToSimulatedCollection() {
	if a.LocalManager != nil &&
		!a.GetUser().IsSimulated &&
		a.Settings != nil &&
		a.Settings.Library != nil &&
		a.Settings.Library.AutoSyncToLocalAccount {
		_ = a.LocalManager.SynchronizeAnilistToSimulatedCollection()
	}
}

// RefreshAnimeCollection queries Anilist for the user's collection
func (a *App) RefreshAnimeCollection() (*anilist.AnimeCollection, error) {
	go func() {
		a.OnRefreshAnilistCollectionFuncs.Range(func(key string, f func()) bool {
			go f()
			return true
		})
	}()

	ret, err := a.AnilistPlatformRef.Get().RefreshAnimeCollection(context.Background())

	if err != nil {
		return nil, err
	}

	// Save the collection to PlaybackManager
	a.PlaybackManager.SetAnimeCollection(ret)

	// Save the collection to AutoDownloader
	a.AutoDownloader.SetAnimeCollection(ret)

	// Save the collection to LocalManager
	a.LocalManager.SetAnimeCollection(ret)

	// Save the collection to DirectStreamManager
	a.DirectStreamManager.SetAnimeCollection(ret)

	// Save the collection to LibraryExplorer
	a.LibraryExplorer.SetAnimeCollection(ret)

	a.AutoScanner.SetAnimeCollection(ret)

	//a.SyncAnilistToSimulatedCollection()

	a.WSEventManager.SendEvent(events.RefreshedAnilistAnimeCollection, nil)

	return ret, nil
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GetMangaCollection is the same as GetAnimeCollection but for manga
func (a *App) GetMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatformRef.Get().GetMangaCollection(context.Background(), bypassCache)
}

// GetRawMangaCollection does not exclude custom lists
func (a *App) GetRawMangaCollection(bypassCache bool) (*anilist.MangaCollection, error) {
	return a.AnilistPlatformRef.Get().GetRawMangaCollection(context.Background(), bypassCache)
}

// RefreshMangaCollection queries Anilist for the user's manga collection
func (a *App) RefreshMangaCollection() (*anilist.MangaCollection, error) {
	mc, err := a.AnilistPlatformRef.Get().RefreshMangaCollection(context.Background())

	if err != nil {
		return nil, err
	}

	a.LocalManager.SetMangaCollection(mc)

	a.WSEventManager.SendEvent(events.RefreshedAnilistMangaCollection, nil)

	return mc, nil
}
