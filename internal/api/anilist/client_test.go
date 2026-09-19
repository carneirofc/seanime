package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"seanime/internal/util"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type testClock struct {
	now time.Time
}

func (c *testClock) Now() time.Time {
	return c.now
}

func (c *testClock) Advance(delay time.Duration) {
	c.now = c.now.Add(delay)
}

func newAniListTestResponse(statusCode int, body string, headers map[string]string) *http.Response {
	respHeaders := make(http.Header)
	for key, value := range headers {
		respHeaders.Set(key, value)
	}

	return &http.Response{
		StatusCode: statusCode,
		Header:     respHeaders,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestGetAnimeById(t *testing.T) {
	anilistClient := NewTestAnilistClient()

	tests := []struct {
		name    string
		mediaId int
	}{
		{
			name:    "Re:Zero",
			mediaId: 21355,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := anilistClient.BaseAnimeByID(context.Background(), &tt.mediaId)
			assert.NoError(t, err)
			assert.NotNil(t, res)
		})
	}
}

func TestGetAnimeByIdLive(t *testing.T) {
	anilistClient := newLiveAnilistClient(t)
	mediaID := 1

	res, err := anilistClient.BaseAnimeByID(context.Background(), &mediaID)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestListAnime(t *testing.T) {
	tests := []struct {
		name                string
		Page                *int
		Search              *string
		PerPage             *int
		Sort                []*MediaSort
		Status              []*MediaStatus
		Genres              []*string
		Tags                []*string
		AverageScoreGreater *int
		Season              *MediaSeason
		SeasonYear          *int
		Format              *MediaFormat
		IsAdult             *bool
		CountryOfOrigin     *string
	}{
		{
			name:                "Popular",
			Page:                new(1),
			Search:              nil,
			PerPage:             new(20),
			Sort:                []*MediaSort{new(MediaSortTrendingDesc)},
			Status:              nil,
			Genres:              nil,
			Tags:                nil,
			AverageScoreGreater: nil,
			Season:              nil,
			SeasonYear:          nil,
			Format:              nil,
			IsAdult:             nil,
			CountryOfOrigin:     nil,
		},
	}

	anilistClient := NewTestAnilistClient()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			cacheKey := ListAnimeCacheKey(
				tt.Page,
				tt.Search,
				tt.PerPage,
				tt.Sort,
				tt.Status,
				tt.Genres,
				tt.Tags,
				tt.AverageScoreGreater,
				tt.Season,
				tt.SeasonYear,
				tt.Format,
				tt.IsAdult,
				tt.CountryOfOrigin,
			)

			t.Log(cacheKey)

			res, err := ListAnimeM(
				anilistClient,
				tt.Page,
				tt.Search,
				tt.PerPage,
				tt.Sort,
				tt.Status,
				tt.Genres,
				tt.Tags,
				tt.AverageScoreGreater,
				tt.Season,
				tt.SeasonYear,
				tt.Format,
				tt.IsAdult,
				tt.CountryOfOrigin,
				util.NewLogger(),
				"",
			)
			assert.NoError(t, err)

			assert.Equal(t, *tt.PerPage, len(res.GetPage().GetMedia()))

			spew.Dump(res)
		})
	}
}

func TestDoAniListRequestWithRetriesWaitsBetweenRateLimitedAttempts(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)}
	rateBlocker := newAniListRateBlocker()
	rateBlocker.now = clock.Now
	requestBody := `{"query":"test"}`
	requestBodies := make([]string, 0, 2)
	sleepDurations := make([]time.Duration, 0, 1)
	rateLimitWarnings := make([]int, 0, 1)
	attempt := 0

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempt++

		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, string(body))

		if attempt == 1 {
			return newAniListTestResponse(http.StatusTooManyRequests, `{"errors":[{"message":"rate limited"}]}`, map[string]string{
				"Date":        clock.Now().Format(http.TimeFormat),
				"Retry-After": "0",
			}), nil
		}

		return newAniListTestResponse(http.StatusOK, `{"data":{"ok":true}}`, map[string]string{
			"X-Ratelimit-Remaining": "9",
		}), nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://anilist.test/graphql", bytes.NewBufferString(requestBody))
	require.NoError(t, err)

	resp, rlRemainingStr, err := doAniListRequestWithRetries(
		client,
		req,
		rateBlocker,
		func(ctx context.Context, delay time.Duration) error {
			sleepDurations = append(sleepDurations, delay)
			clock.Advance(delay)
			return nil
		},
		func(waitSeconds int) {
			rateLimitWarnings = append(rateLimitWarnings, waitSeconds)
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 2, attempt)
	assert.Equal(t, []time.Duration{time.Second}, sleepDurations)
	assert.Equal(t, []int{1}, rateLimitWarnings)
	assert.Equal(t, []string{requestBody, requestBody}, requestBodies)
	assert.Equal(t, "9", rlRemainingStr)
}

func TestDoAniListRequestWithRetriesDoesNotRetryWhenRateLimitHeadersAreMissing(t *testing.T) {
	// without explicit rate-limit headers, the response should be returned as-is.
	sleepDurations := make([]time.Duration, 0, 1)
	attempt := 0

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempt++
		return newAniListTestResponse(http.StatusOK, `{"data":{"ok":true}}`, nil), nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://anilist.test/graphql", bytes.NewBufferString(`{"query":"test"}`))
	require.NoError(t, err)

	resp, rlRemainingStr, err := doAniListRequestWithRetries(
		client,
		req,
		nil,
		func(ctx context.Context, delay time.Duration) error {
			sleepDurations = append(sleepDurations, delay)
			return nil
		},
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 1, attempt)
	assert.Empty(t, sleepDurations)
	assert.Equal(t, "", rlRemainingStr)
}

func TestDoAniListRequestWithRetriesHeaders(t *testing.T) {
	now := time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)
	attempt := 0
	warnings := 0

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempt++
		return newAniListTestResponse(http.StatusOK, `{"data":{"ok":true}}`, map[string]string{
			"Date":                  now.Format(http.TimeFormat),
			"X-Ratelimit-Remaining": "9",
			"X-Ratelimit-Reset":     strconv.FormatInt(now.Add(time.Minute).Unix(), 10),
		}), nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://anilist.test/graphql", bytes.NewBufferString(`{"query":"test"}`))
	require.NoError(t, err)

	resp, remaining, err := doAniListRequestWithRetries(
		client,
		req,
		newAniListRateBlocker(),
		nil,
		func(int) {
			warnings++
		},
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 1, attempt)
	assert.Equal(t, "9", remaining)
	assert.Zero(t, warnings)
}

func TestDoAniListRequestWithRetriesExhaustsRetries(t *testing.T) {
	clock := &testClock{now: time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)}
	rateBlocker := newAniListRateBlocker()
	rateBlocker.now = clock.Now
	requestBody := `{"query":"test"}`
	attempt := 0

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempt++
		return newAniListTestResponse(http.StatusTooManyRequests, `{"errors":[{"message":"rate limited"}]}`, map[string]string{
			"Date":        clock.Now().Format(http.TimeFormat),
			"Retry-After": "0",
		}), nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://anilist.test/graphql", bytes.NewBufferString(requestBody))
	require.NoError(t, err)

	resp, _, err := doAniListRequestWithRetries(
		client,
		req,
		rateBlocker,
		func(ctx context.Context, delay time.Duration) error {
			clock.Advance(delay)
			return nil
		},
		nil,
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded, retries exhausted")
	assert.Nil(t, resp)
	assert.Equal(t, 4, attempt)
}

func TestUseCustomAPIUsesRuntimeConfig(t *testing.T) {
	prevProvider := CurrentRequestProvider()
	t.Cleanup(func() {
		require.NoError(t, SetRequestProvider(prevProvider))
	})

	require.NoError(t, UseCustomAPI(CustomClientConfig{
		Name:     "plugin-client",
		Endpoint: "https://plugin.example.com/graphql",
		Token:    "plugin-token",
		Headers: map[string]string{
			"X-Provider": "plugin",
		},
		Authenticated: true,
	}))

	// a custom client should only need an endpoint and request parameters from the extension.
	req, err := http.NewRequest(http.MethodPost, "https://example.com/graphql", nil)
	require.NoError(t, err)
	require.NoError(t, initAnilistReq(context.Background(), req, ""))

	assert.Equal(t, "https://plugin.example.com/graphql", req.URL.String())
	assert.Equal(t, "plugin", req.Header.Get("X-Provider"))
	assert.Equal(t, "Bearer plugin-token", req.Header.Get("Authorization"))
	assert.Equal(t, "plugin-client", CurrentRequestProviderName())
	assert.True(t, NewAnilistClient("", t.TempDir()).IsAuthenticated())
}

func TestAniListRateBlockerWaitsUntilBlockExpires(t *testing.T) {
	// once blocked, later requests should wait until the shared block expires.
	clock := &testClock{now: time.Date(2026, time.April, 7, 12, 0, 10, 0, time.UTC)}
	rateBlocker := newAniListRateBlocker()
	rateBlocker.now = clock.Now
	require.True(t, rateBlocker.BlockUntil(clock.Now().Add(18*time.Second)))

	sleepDurations := make([]time.Duration, 0, 1)
	err := rateBlocker.Wait(context.Background(), func(ctx context.Context, delay time.Duration) error {
		sleepDurations = append(sleepDurations, delay)
		clock.Advance(delay)
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, []time.Duration{18 * time.Second}, sleepDurations)
}

func TestAniListRateBlockerIgnoresDuplicateOrShorterBlocks(t *testing.T) {
	// concurrent 429s with the same reset should not re-announce the same block repeatedly.
	clock := &testClock{now: time.Date(2026, time.April, 7, 12, 0, 20, 0, time.UTC)}
	rateBlocker := newAniListRateBlocker()
	rateBlocker.now = clock.Now
	blockedUntil := clock.Now().Add(18 * time.Second)

	assert.True(t, rateBlocker.BlockUntil(blockedUntil))
	assert.False(t, rateBlocker.BlockUntil(blockedUntil))
	assert.False(t, rateBlocker.BlockUntil(clock.Now().Add(5*time.Second)))
	assert.True(t, rateBlocker.BlockUntil(clock.Now().Add(25*time.Second)))
}

// capturingRequestProvider routes every AniList request to a caller-supplied transport so a test can
// read the GraphQL payload the client actually put on the wire.
type capturingRequestProvider struct {
	client *http.Client
}

func (capturingRequestProvider) Name() string               { return "capturing" }
func (capturingRequestProvider) ApiUrl() string             { return "https://anilist.test/graphql" }
func (p capturingRequestProvider) HttpClient() *http.Client { return p.client }
func (capturingRequestProvider) PrepareRequest(context.Context, *http.Request, string) error {
	return nil
}
func (capturingRequestProvider) IsAuthenticated(token string) bool { return token != "" }

func captureMutationVariables(t *testing.T, responseBody string, call func(AnilistClient) error) map[string]any {
	t.Helper()

	prevProvider := CurrentRequestProvider()
	t.Cleanup(func() {
		require.NoError(t, SetRequestProvider(prevProvider))
	})

	var captured map[string]any
	provider := capturingRequestProvider{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		var payload struct {
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.Unmarshal(body, &payload))
		captured = payload.Variables

		return newAniListTestResponse(http.StatusOK, responseBody, nil), nil
	})}}
	require.NoError(t, SetRequestProvider(provider))

	require.NoError(t, call(NewAnilistClient("test-token", t.TempDir())))
	require.NotNil(t, captured)
	return captured
}

func captureUpdateMediaListEntryVariables(t *testing.T, call func(AnilistClient) error) map[string]any {
	t.Helper()
	return captureMutationVariables(t, `{"data":{"SaveMediaListEntry":{"id":1}}}`, call)
}

func TestUpdateMediaListEntryOmitsNilVariables(t *testing.T) {
	// AniList validates scoreRaw and progress with an integer rule whenever the keys are present, so a
	// privacy-only update must leave them out entirely rather than send them as null.
	captured := captureUpdateMediaListEntryVariables(t, func(client AnilistClient) error {
		_, err := client.UpdateMediaListEntry(context.Background(), new(320), nil, nil, nil, nil, nil, new(true), new(true))
		return err
	})

	keys := slices.Sorted(maps.Keys(captured))
	assert.Equal(t, []string{"hiddenFromStatusLists", "mediaId", "private"}, keys)
	assert.Equal(t, float64(320), captured["mediaId"])
	assert.Equal(t, true, captured["private"])
	assert.Equal(t, true, captured["hiddenFromStatusLists"])
}

func TestUpdateMediaListEntrySendsEveryProvidedVariable(t *testing.T) {
	// The full edit path must be unchanged: nothing is dropped when the caller provides it.
	captured := captureUpdateMediaListEntryVariables(t, func(client AnilistClient) error {
		_, err := client.UpdateMediaListEntry(
			context.Background(),
			new(320),
			new(MediaListStatusCurrent),
			new(85),
			new(0), // a zero progress is a real value, not an absent one
			&FuzzyDateInput{Year: new(2026), Month: new(4), Day: new(7)},
			nil,
			new(false),
			new(false),
		)
		return err
	})

	keys := slices.Sorted(maps.Keys(captured))
	assert.Equal(t, []string{
		"hiddenFromStatusLists", "mediaId", "private", "progress", "scoreRaw", "startedAt", "status",
	}, keys)
	assert.Equal(t, float64(0), captured["progress"])
	assert.Equal(t, float64(85), captured["scoreRaw"])
}

func TestUpdateMediaListEntryProgressOmitsNilStatus(t *testing.T) {
	// A queued progress update carries no status (shared_platform.syncQueuedUpdate replays one),
	// and sending `status: null` made AniList reject the whole mutation with a 400. The replay is
	// unattended, so this failed silently and forever.
	captured := captureUpdateMediaListEntryVariables(t, func(client AnilistClient) error {
		_, err := client.UpdateMediaListEntryProgress(context.Background(), new(320), new(4), nil)
		return err
	})

	keys := slices.Sorted(maps.Keys(captured))
	assert.Equal(t, []string{"mediaId", "progress"}, keys)
	assert.Equal(t, float64(320), captured["mediaId"])
	assert.Equal(t, float64(4), captured["progress"])
}

func TestUpdateMediaListEntryProgressSendsEveryProvidedVariable(t *testing.T) {
	// The normal path is unchanged, and a zero progress is a real value rather than an absent one.
	captured := captureUpdateMediaListEntryVariables(t, func(client AnilistClient) error {
		_, err := client.UpdateMediaListEntryProgress(context.Background(), new(320), new(0), new(MediaListStatusCurrent))
		return err
	})

	keys := slices.Sorted(maps.Keys(captured))
	assert.Equal(t, []string{"mediaId", "progress", "status"}, keys)
	assert.Equal(t, float64(0), captured["progress"])
	assert.Equal(t, string(MediaListStatusCurrent), captured["status"])
}

func TestUpdateMediaListEntryRepeatOmitsNilVariables(t *testing.T) {
	captured := captureUpdateMediaListEntryVariables(t, func(client AnilistClient) error {
		_, err := client.UpdateMediaListEntryRepeat(context.Background(), new(320), nil)
		return err
	})

	keys := slices.Sorted(maps.Keys(captured))
	assert.Equal(t, []string{"mediaId"}, keys)
}

func TestDeleteEntrySendsEntryID(t *testing.T) {
	captured := captureMutationVariables(t, `{"data":{"DeleteMediaListEntry":{"deleted":true}}}`, func(client AnilistClient) error {
		_, err := client.DeleteEntry(context.Background(), new(99))
		return err
	})

	assert.Equal(t, []string{"mediaListEntryId"}, slices.Sorted(maps.Keys(captured)))
	assert.Equal(t, float64(99), captured["mediaListEntryId"])
}

func TestMutationsDoNotPanicOnNilArguments(t *testing.T) {
	// The mutation arguments come from plugin-visible hook events, so a plugin nulling one must not
	// take the server down on a log line.
	prevProvider := CurrentRequestProvider()
	t.Cleanup(func() {
		require.NoError(t, SetRequestProvider(prevProvider))
	})

	provider := capturingRequestProvider{client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return newAniListTestResponse(http.StatusOK, `{"data":{"SaveMediaListEntry":{"id":1}}}`, nil), nil
	})}}
	require.NoError(t, SetRequestProvider(provider))

	client := NewAnilistClient("test-token", t.TempDir())

	assert.NotPanics(t, func() {
		_, _ = client.UpdateMediaListEntryProgress(context.Background(), nil, nil, nil)
	})
	assert.NotPanics(t, func() {
		_, _ = client.UpdateMediaListEntryRepeat(context.Background(), nil, nil)
	})
	assert.NotPanics(t, func() {
		_, err := client.DeleteEntry(context.Background(), nil)
		assert.Error(t, err)
	})
}
