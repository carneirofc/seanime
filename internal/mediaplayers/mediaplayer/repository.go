package mediaplayer

import (
	"context"
	"errors"
	"fmt"
	"seanime/internal/continuity"
	"seanime/internal/events"
	"seanime/internal/hook"
	"seanime/internal/mediaplayers/iina"
	mpchc2 "seanime/internal/mediaplayers/mpchc"
	"seanime/internal/mediaplayers/mpv"
	vlc2 "seanime/internal/mediaplayers/vlc"
	"seanime/internal/util/result"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	PlayerClosedEvent = "Player closed"
)

type PlaybackType string

const (
	PlaybackTypeFile   PlaybackType = "file"
	PlaybackTypeStream PlaybackType = "stream"
)

type (
	// Repository provides a common interface to interact with media players
	Repository struct {
		Logger                *zerolog.Logger
		Default               string
		VLC                   *vlc2.VLC
		MpcHc                 *mpchc2.MpcHc
		Mpv                   *mpv.Mpv
		Iina                  *iina.Iina
		wsEventManager        events.WSEventManagerInterface
		continuityManager     *continuity.Manager
		playerInUse           string
		completionThreshold   float64
		mu                    sync.RWMutex
		isRunning             bool
		trackingType          string // "file" or "stream" - tracks which type of tracking is active
		currentPlaybackStatus *PlaybackStatus
		subscribers           *result.Map[string, *RepositorySubscriber]
		cancel                context.CancelFunc
		exitedCh              chan struct{} // Closed when the media player exits
	}

	NewRepositoryOptions struct {
		Logger            *zerolog.Logger
		Default           string
		VLC               *vlc2.VLC
		MpcHc             *mpchc2.MpcHc
		Mpv               *mpv.Mpv
		Iina              *iina.Iina
		WSEventManager    events.WSEventManagerInterface
		ContinuityManager *continuity.Manager
	}

	// RepositorySubscriber provides a single event channel for all media player events
	RepositorySubscriber struct {
		EventCh chan MediaPlayerEvent
	}

	// MediaPlayerEvent is the base interface for all media player events
	MediaPlayerEvent interface {
		Type() string
	}

	// Local file playback events
	TrackingStartedEvent struct {
		Status *PlaybackStatus
	}

	TrackingRetryEvent struct {
		Reason string
	}

	VideoCompletedEvent struct {
		Status *PlaybackStatus
	}

	TrackingStoppedEvent struct {
		Reason string
	}

	PlaybackStatusEvent struct {
		Status *PlaybackStatus
	}

	// Streaming playback events
	StreamingTrackingStartedEvent struct {
		Status *PlaybackStatus
	}

	StreamingTrackingRetryEvent struct {
		Reason string
	}

	StreamingVideoCompletedEvent struct {
		Status *PlaybackStatus
	}

	StreamingTrackingStoppedEvent struct {
		Reason string
	}

	StreamingPlaybackStatusEvent struct {
		Status *PlaybackStatus
	}

	PlaybackStatus struct {
		// CompletionPercentage (not actually a percentage, but a float between 0 and 1)
		CompletionPercentage float64 `json:"completionPercentage"`
		Playing              bool    `json:"playing"`
		Filename             string  `json:"filename"`
		Path                 string  `json:"path"`
		Duration             int     `json:"duration"` // in ms
		Filepath             string  `json:"filepath"`

		CurrentTimeInSeconds float64 `json:"currentTimeInSeconds"` // in seconds
		DurationInSeconds    float64 `json:"durationInSeconds"`    // in seconds

		PlaybackType PlaybackType `json:"playbackType"` // "file", "stream"
	}
)

func (e TrackingStartedEvent) Type() string          { return "tracking_started" }
func (e TrackingRetryEvent) Type() string            { return "tracking_retry" }
func (e VideoCompletedEvent) Type() string           { return "video_completed" }
func (e TrackingStoppedEvent) Type() string          { return "tracking_stopped" }
func (e PlaybackStatusEvent) Type() string           { return "playback_status" }
func (e StreamingTrackingStartedEvent) Type() string { return "streaming_tracking_started" }
func (e StreamingTrackingRetryEvent) Type() string   { return "streaming_tracking_retry" }
func (e StreamingVideoCompletedEvent) Type() string  { return "streaming_video_completed" }
func (e StreamingTrackingStoppedEvent) Type() string { return "streaming_tracking_stopped" }
func (e StreamingPlaybackStatusEvent) Type() string  { return "streaming_playback_status" }

func NewRepository(opts *NewRepositoryOptions) *Repository {

	return &Repository{
		Logger:                opts.Logger,
		Default:               opts.Default,
		VLC:                   opts.VLC,
		MpcHc:                 opts.MpcHc,
		Mpv:                   opts.Mpv,
		Iina:                  opts.Iina,
		wsEventManager:        opts.WSEventManager,
		continuityManager:     opts.ContinuityManager,
		completionThreshold:   0.8,
		subscribers:           result.NewMap[string, *RepositorySubscriber](),
		currentPlaybackStatus: &PlaybackStatus{},
		exitedCh:              make(chan struct{}),
	}
}

func (m *Repository) Subscribe(id string) *RepositorySubscriber {
	sub := &RepositorySubscriber{
		EventCh: make(chan MediaPlayerEvent, 10), // Buffered channel to avoid blocking
	}
	m.subscribers.Set(id, sub)
	return sub
}

func (m *Repository) Unsubscribe(id string) {
	m.subscribers.Delete(id)
}

func (m *Repository) GetStatus() *PlaybackStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentPlaybackStatus
}

// PullStatus returns the current playback status directly from the media player.
func (m *Repository) PullStatus() (*PlaybackStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, err := m.getStatus()
	if err != nil {
		return nil, false
	}

	if m.currentPlaybackStatus == nil {
		return nil, false
	}

	// Preserve the playback type currently being tracked; only "file" maps to file
	// tracking, everything else (including the zero value) is treated as a stream.
	playbackType := PlaybackTypeStream
	if m.currentPlaybackStatus.PlaybackType == PlaybackTypeFile {
		playbackType = PlaybackTypeFile
	}

	ps, ok := normalizeStatus(m.Default, status, playbackType)
	if ok {
		m.currentPlaybackStatus = ps
	}
	return m.currentPlaybackStatus, ok
}

func (m *Repository) IsRunning() bool {
	return m.isRunning
}

func (m *Repository) GetExecutablePath() string {
	switch m.Default {
	case "vlc":
		return m.VLC.GetExecutablePath()
	case "mpc-hc":
		return m.MpcHc.GetExecutablePath()
	case "mpv":
		return m.Mpv.GetExecutablePath()
	case "iina":
		return m.Iina.GetExecutablePath()
	}
	return ""
}

func (m *Repository) GetDefault() string {
	return m.Default
}

// Play will start the media player and load the video at the given path.
// The implementation of the specific media player is handled by the respective media player package.
// Calling it multiple *should* not open multiple instances of the media player -- subsequent calls should just load a new video if the media player is already open.
func (m *Repository) Play(path string) error {

	m.Logger.Debug().Str("path", path).Msg("media player: Media requested")

	lastWatched := m.continuityManager.GetExternalPlayerEpisodeWatchHistoryItem(path, false, 0, 0)

	switch m.Default {
	case "vlc":
		err := m.VLC.Start()
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not start media player using VLC")
			return fmt.Errorf("could not start VLC, %w", err)
		}

		err = m.VLC.AddAndPlay(path)
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not open and play video using VLC")
			if m.VLC.Path != "" {
				return fmt.Errorf("could not open and play video, %w", err)
			} else {
				return fmt.Errorf("could not open and play video, %w", err)
			}
		}

		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			if lastWatched.Found {
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.ForcePause()
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.SeekTo(fmt.Sprintf("%d", int(lastWatched.Item.CurrentTime)))
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.Resume()
			}
		}

		return nil
	case "mpc-hc":
		err := m.MpcHc.Start()
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not start media player using MPC-HC")
			return fmt.Errorf("could not start MPC-HC, %w", err)
		}
		_, err = m.MpcHc.OpenAndPlay(path)
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not open and play video using MPC-HC")
			return fmt.Errorf("could not open and play video, %w", err)
		}

		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			if lastWatched.Found {
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.Pause()
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.SeekTo(int(lastWatched.Item.CurrentTime))
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.Play()
			}
		}

		return nil
	case "mpv":
		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			var args []string
			if lastWatched.Found {
				//args = append(args, "--no-resume-playback", fmt.Sprintf("--start=+%d", int(lastWatched.Item.CurrentTime)))
				args = append(args, "--no-resume-playback")
			}
			err := m.Mpv.OpenAndPlay(path, args...)
			if err != nil {
				m.Logger.Error().Err(err).Msg("media player: Could not open and play video using MPV")
				return fmt.Errorf("could not open and play video, %w", err)
			}
			if lastWatched.Found {
				_ = m.Mpv.SeekToSlow(lastWatched.Item.CurrentTime)
			}
		} else {
			err := m.Mpv.OpenAndPlay(path)
			if err != nil {
				m.Logger.Error().Err(err).Msg("media player: Could not open and play video using MPV")
				return fmt.Errorf("could not open and play video, %w", err)
			}
		}

		return nil
	case "iina":
		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			var args []string
			if lastWatched.Found {
				//args = append(args, "--mpv-no-resume-playback", fmt.Sprintf("--mpv-start=+%d", int(lastWatched.Item.CurrentTime)))
				args = append(args, "--mpv-no-resume-playback")
			}
			err := m.Iina.OpenAndPlay(path, args...)
			if err != nil {
				m.Logger.Error().Err(err).Msg("media player: Could not open and play video using IINA")
				return fmt.Errorf("could not open and play video, %w", err)
			}
			if lastWatched.Found {
				_ = m.Iina.SeekToSlow(lastWatched.Item.CurrentTime)
			}
		} else {
			err := m.Iina.OpenAndPlay(path)
			if err != nil {
				m.Logger.Error().Err(err).Msg("media player: Could not open and play video using IINA")
				return fmt.Errorf("could not open and play video, %w", err)
			}
		}

		return nil
	default:
		return errors.New("no default media player set")
	}

}

func (m *Repository) Append(path string) error {
	switch m.Default {
	case "mpv":
		err := m.Mpv.Append(path)
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not append video on MPV")
			return fmt.Errorf("could not append video, %w", err)
		}
	case "iina":
		err := m.Iina.Append(path)
		if err != nil {
			m.Logger.Error().Err(err).Msg("media player: Could not append video on IINA")
			return fmt.Errorf("could not append video, %w", err)
		}
	default:
		m.Logger.Trace().Str("player", m.Default).Msg("media player: Appending is not supported by the player")
	}

	return nil
}

func (m *Repository) Pause() error {
	switch m.Default {
	case "vlc":
		return m.VLC.Pause()
	case "mpc-hc":
		return m.MpcHc.Pause()
	case "mpv":
		return m.Mpv.Pause()
	case "iina":
		return m.Iina.Pause()
	default:
		return errors.New("no default media player set")
	}
}

func (m *Repository) Resume() error {
	switch m.Default {
	case "vlc":
		return m.VLC.Resume()
	case "mpc-hc":
		return m.MpcHc.Play()
	case "mpv":
		return m.Mpv.Resume()
	case "iina":
		return m.Iina.Resume()
	default:
		return errors.New("no default media player set")
	}
}

func (m *Repository) SeekTo(seconds float64) error {
	switch m.Default {
	case "vlc":
		return m.VLC.SeekTo(fmt.Sprintf("%d", int(seconds)))
	case "mpc-hc":
		return m.MpcHc.SeekTo(int(seconds * 1000))
	case "mpv":
		return m.Mpv.SeekTo(seconds)
	case "iina":
		return m.Iina.SeekTo(seconds)
	default:
		return errors.New("no default media player set")
	}
}

func (m *Repository) Stream(streamUrl string, episode int, mediaId int, windowTitle string) error {

	m.Logger.Debug().Str("streamUrl", streamUrl).Msg("media player: Stream requested")
	var err error

	switch m.Default {
	case "vlc":
		err = m.VLC.Start()
	case "mpc-hc":
		err = m.MpcHc.Start()
		_, err = m.MpcHc.OpenAndPlay(streamUrl)
	case "mpv":
		// MPV does not need to be started
	case "iina":
		// IINA does not need to be started
	default:
		return errors.New("no default media player set")
	}

	if err != nil {
		m.Logger.Error().Err(err).Msg("media player: Could not start media player for stream")
		return fmt.Errorf("could not open media player, %w", err)
	}

	lastWatched := m.continuityManager.GetExternalPlayerEpisodeWatchHistoryItem("", true, episode, mediaId)

	switch m.Default {
	case "vlc":
		err = m.VLC.AddAndPlay(streamUrl)

		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			if lastWatched.Found {
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.ForcePause()
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.SeekTo(fmt.Sprintf("%d", int(lastWatched.Item.CurrentTime)))
				time.Sleep(400 * time.Millisecond)
				_ = m.VLC.Resume()
			}
		}

	case "mpc-hc":
		_, err = m.MpcHc.OpenAndPlay(streamUrl)

		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			if lastWatched.Found {
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.Pause()
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.SeekTo(int(lastWatched.Item.CurrentTime))
				time.Sleep(400 * time.Millisecond)
				_ = m.MpcHc.Play()
			}
		}

	case "mpv":
		args := []string{}
		if windowTitle != "" {
			args = append(args, fmt.Sprintf("--title=%s", windowTitle))
		}
		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			err = m.Mpv.OpenAndPlay(streamUrl, args...)
			if lastWatched.Found {
				_ = m.Mpv.SeekToSlow(lastWatched.Item.CurrentTime)
			}
		} else {
			err = m.Mpv.OpenAndPlay(streamUrl, args...)
		}

	case "iina":
		args := []string{}
		if windowTitle != "" {
			args = append(args, fmt.Sprintf("--mpv-title=%s", windowTitle))
		}
		if m.continuityManager.GetSettings().WatchContinuityEnabled {
			err = m.Iina.OpenAndPlay(streamUrl, args...)
			if lastWatched.Found {
				_ = m.Iina.SeekToSlow(lastWatched.Item.CurrentTime)
			}
		} else {
			err = m.Iina.OpenAndPlay(streamUrl, args...)
		}

	}

	if err != nil {
		m.Logger.Error().Err(err).Msg("media player: Could not open and play stream")
		return fmt.Errorf("could not open and play stream, %w", err)
	}

	return nil
}

// Cancel will stop the tracking process and publish an "abnormal" event
func (m *Repository) Cancel() {
	m.mu.Lock()
	if m.cancel != nil {
		m.Logger.Debug().Msg("media player: Cancel request received")
		m.cancel()
		m.emitTrackingStopped(PlaybackTypeFile, "Something went wrong, tracking cancelled")
	}
	// Close MPV if it's the default player
	switch m.Default {
	case "mpv":
		go m.Mpv.CloseAll()
	case "iina":
		go m.Iina.CloseAll()
	}
	m.mu.Unlock()
}

// Stop will stop the tracking process and publish a "normal" event
func (m *Repository) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.Logger.Debug().Msg("media player: Stop request received")
		m.cancel()
		m.cancel = nil
		m.emitTrackingStopped(PlaybackTypeFile, "Tracking stopped")
	}
	switch m.Default {
	case "mpv":
		m.Mpv.CloseAll()
	case "iina":
		m.Iina.CloseAll()
	}
	m.mu.Unlock()
}

// StartTrackingTorrentStream will start tracking media player status for torrent streaming
func (m *Repository) StartTrackingTorrentStream() {
	m.mu.Lock()

	// Check if tracking is already running
	if m.isRunning {
		m.Logger.Debug().Str("currentTrackingType", m.trackingType).Msg("media player: Tracking already running, cancelling previous tracking")
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.isRunning = false
		m.trackingType = ""
	}

	// Create a new context
	var trackingCtx context.Context
	trackingCtx, m.cancel = context.WithCancel(context.Background())

	done := make(chan struct{})
	var filename string
	var completed bool
	var retries int

	hookEvent := &MediaPlayerStreamTrackingRequestedEvent{
		StartRefreshDelay:    3,
		RefreshDelay:         1,
		MaxRetries:           5,
		MaxRetriesAfterStart: 5,
	}
	_ = hook.GlobalHookManager.OnMediaPlayerStreamTrackingRequested().Trigger(hookEvent)
	startRefreshDelay := hookEvent.StartRefreshDelay
	maxTries := hookEvent.MaxRetries
	refreshDelay := hookEvent.RefreshDelay
	maxRetriesAfterStart := hookEvent.MaxRetriesAfterStart

	// Default prevented, do not track
	if hookEvent.DefaultPrevented {
		m.Logger.Debug().Msg("media player: Tracking cancelled by hook")
		m.mu.Unlock()
		return
	}

	// Unlike normal tracking when the file is downloaded, we may need to wait a bit before we can get the status,
	// so we won't count retries until it's confirmed that the file has started playing.
	var trackingStarted bool
	var waitInSeconds int

	m.isRunning = true
	m.trackingType = "stream"
	gotFirstStatus := false

	m.mu.Unlock()

	go func(trackingCtx context.Context) {
		defer func() {
			m.mu.Lock()
			m.isRunning = false
			m.trackingType = ""
			m.mu.Unlock()
		}()

		for {
			select {
			case <-done:
				m.mu.Lock()
				m.Logger.Debug().Msg("media player: Connection lost")
				m.mu.Unlock()
				return
			case <-trackingCtx.Done():
				m.mu.Lock()
				m.Logger.Debug().Msg("media player: Context cancelled")
				m.mu.Unlock()
				return
			//case <-m.exitedCh:
			//	m.mu.Lock()
			//	m.Logger.Debug().Msg("media player: Player exited")
			//	m.streamingTrackingStopped(PlayerClosedEvent)
			//	m.mu.Unlock()
			//	return
			default:
				// Wait at least 3 seconds before we start checking the status
				if !gotFirstStatus {
					time.Sleep(time.Duration(startRefreshDelay) * time.Second)
				} else {
					time.Sleep(time.Duration(refreshDelay) * time.Second)
				}
				status, err := m.getStatus()
				if err != nil {
					if !trackingStarted {
						if waitInSeconds > 60 {
							m.Logger.Warn().Msg("media player: Ending goroutine, waited too long")
							return
						}
						m.Logger.Trace().Msgf("media player: Waiting for stream, %d seconds", waitInSeconds)
						waitInSeconds += refreshDelay
						continue
					} else {
						m.emitTrackingRetry(PlaybackTypeStream, "Failed to get player status")
						m.Logger.Error().Msgf("media player: Failed to get player status, retrying (%d/%d)", retries+1, maxTries)

						// Video is completed, and we are unable to get the status
						// We can safely assume that the player has been closed
						if retries == 1 && (completed || m.continuityManager.GetSettings().WatchContinuityEnabled) {
							m.Logger.Debug().Msg("media player: Sending player closed event")
							m.emitTrackingStopped(PlaybackTypeStream, PlayerClosedEvent)
							close(done)
							break
						}

						if retries >= maxTries-1 {
							m.Logger.Debug().Msg("media player: Sending failed status query event")
							m.emitTrackingStopped(PlaybackTypeStream, "Failed to get player status")
							close(done)
							break
						}
						retries++
						continue
					}
				}

				trackingStarted = true
				retries = 0
				ps, ok := normalizeStatus(m.Default, status, PlaybackTypeStream)

				if !ok {
					m.emitTrackingRetry(PlaybackTypeStream, "Failed to get player status")
					m.Logger.Error().Interface("status", status).Msgf("media player: Failed to process status, retrying (%d/%d)", retries+1, maxRetriesAfterStart)
					if retries >= maxRetriesAfterStart-1 {
						m.Logger.Debug().Msg("media player: Sending failed status query event")
						m.emitTrackingStopped(PlaybackTypeStream, "Failed to process status")
						close(done)
						break
					}
					retries++
					continue
				}

				m.currentPlaybackStatus = ps

				if m.handleTrackedStatus(PlaybackTypeStream, &filename, &completed) {
					continue
				}
			}
		}
	}(trackingCtx)
}

// StartTracking will start tracking media player status.
// This method is safe to call multiple times -- it will cancel the previous context and start a new one.
func (m *Repository) StartTracking() {
	m.mu.Lock()

	// Check if tracking is already running
	if m.isRunning {
		m.Logger.Debug().Str("currentTrackingType", m.trackingType).Msg("media player: Tracking already running, cancelling previous tracking")
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		m.isRunning = false
		m.trackingType = ""
	}

	// Create a new context
	var trackingCtx context.Context
	trackingCtx, m.cancel = context.WithCancel(context.Background())

	done := make(chan struct{})
	var filename string
	var completed bool
	var retries int

	hookEvent := &MediaPlayerLocalFileTrackingRequestedEvent{
		StartRefreshDelay: 3,
		RefreshDelay:      1,
		MaxRetries:        5,
	}
	_ = hook.GlobalHookManager.OnMediaPlayerLocalFileTrackingRequested().Trigger(hookEvent)
	startRefreshDelay := hookEvent.StartRefreshDelay
	maxTries := hookEvent.MaxRetries
	refreshDelay := hookEvent.RefreshDelay

	// Default prevented, do not track
	if hookEvent.DefaultPrevented {
		m.Logger.Debug().Msg("media player: Tracking cancelled by hook")
		m.mu.Unlock()
		return
	}

	m.isRunning = true
	m.trackingType = "file"
	gotFirstStatus := false

	m.mu.Unlock()

	go func(trackingCtx context.Context) {
		defer func() {
			m.mu.Lock()
			m.isRunning = false
			m.trackingType = ""
			m.mu.Unlock()
		}()

		for {
			select {
			case <-done:
				m.mu.Lock()
				m.Logger.Debug().Msg("media player: Connection lost")
				m.isRunning = false
				m.mu.Unlock()
				return
			case <-trackingCtx.Done():
				m.mu.Lock()
				m.Logger.Debug().Msg("media player: Context cancelled")
				m.isRunning = false
				m.mu.Unlock()
				return
			//case <-m.exitedCh:
			//	m.mu.Lock()
			//	m.Logger.Debug().Msg("media player: Player exited")
			//	m.isRunning = false
			//	m.trackingStopped(PlayerClosedEvent)
			//	m.mu.Unlock()
			//	return
			default:
				// Wait at least X seconds before we start checking the status
				if !gotFirstStatus {
					time.Sleep(time.Duration(startRefreshDelay) * time.Second)
				} else {
					time.Sleep(time.Duration(refreshDelay) * time.Second)
				}
				status, err := m.getStatus()
				if err != nil {
					m.emitTrackingRetry(PlaybackTypeFile, "Failed to get player status")
					m.Logger.Error().Msgf("media player: Failed to get player status, retrying (%d/%d)", retries+1, maxTries)

					// Video is completed, and we are unable to get the status
					// We can safely assume that the player has been closed
					if retries == 1 && (completed || m.continuityManager.GetSettings().WatchContinuityEnabled) {
						m.emitTrackingStopped(PlaybackTypeFile, PlayerClosedEvent)
						close(done)
						break
					}

					if retries >= maxTries-1 {
						m.emitTrackingStopped(PlaybackTypeFile, "Failed to get player status")
						close(done)
						break
					}
					retries++
					continue
				}

				gotFirstStatus = true
				retries = 0

				ps, ok := normalizeStatus(m.Default, status, PlaybackTypeFile)

				if !ok {
					m.emitTrackingRetry(PlaybackTypeFile, "Failed to get player status")
					m.Logger.Error().Interface("status", status).Msgf("media player: Failed to process status, retrying (%d/%d)", retries+1, maxTries)
					if retries >= maxTries-1 {
						m.emitTrackingStopped(PlaybackTypeFile, "Failed to process status")
						close(done)
						break
					}
					retries++
					continue
				}

				m.currentPlaybackStatus = ps

				if m.handleTrackedStatus(PlaybackTypeFile, &filename, &completed) {
					continue
				}
			}
		}
	}(trackingCtx)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// publish fans a single event out to every subscriber.
func (m *Repository) publish(event MediaPlayerEvent) {
	m.subscribers.Range(func(key string, value *RepositorySubscriber) bool {
		value.EventCh <- event
		return true
	})
}

// The emit* helpers select the file or stream flavour of each event based on the
// playback type. File and stream tracking emit distinct event types because subscribers
// (e.g. torrentstream) type-switch on the streaming variants; the choice lives here so
// each tracking loop no longer duplicates its own set of dispatch methods.

func (m *Repository) emitTrackingStopped(pt PlaybackType, reason string) {
	if pt == PlaybackTypeStream {
		m.publish(StreamingTrackingStoppedEvent{Reason: reason})
		return
	}
	m.publish(TrackingStoppedEvent{Reason: reason})
}

func (m *Repository) emitTrackingStarted(pt PlaybackType, status *PlaybackStatus) {
	if pt == PlaybackTypeStream {
		m.publish(StreamingTrackingStartedEvent{Status: status})
		return
	}
	m.publish(TrackingStartedEvent{Status: status})
}

func (m *Repository) emitTrackingRetry(pt PlaybackType, reason string) {
	if pt == PlaybackTypeStream {
		m.publish(StreamingTrackingRetryEvent{Reason: reason})
		return
	}
	m.publish(TrackingRetryEvent{Reason: reason})
}

func (m *Repository) emitVideoCompleted(pt PlaybackType, status *PlaybackStatus) {
	if pt == PlaybackTypeStream {
		m.publish(StreamingVideoCompletedEvent{Status: status})
		return
	}
	m.publish(VideoCompletedEvent{Status: status})
}

func (m *Repository) emitPlaybackStatus(pt PlaybackType, status *PlaybackStatus) {
	if pt == PlaybackTypeStream {
		m.publish(StreamingPlaybackStatusEvent{Status: status})
		return
	}
	m.publish(PlaybackStatusEvent{Status: status})
}

// handleTrackedStatus emits the started / completed / status events for the current
// playback status and owns the completion-threshold decision for both file and stream
// tracking. It returns true when a new file/stream has just started, in which case the
// caller should skip the rest of the tick (the completion check is intentionally deferred
// to the next reading to avoid a stale CompletionPercentage triggering a false completion).
func (m *Repository) handleTrackedStatus(pt PlaybackType, lastFilename *string, completed *bool) (newMedia bool) {
	status := m.currentPlaybackStatus

	if *lastFilename == "" || *lastFilename != status.Filename {
		m.Logger.Debug().Str("previousFilename", *lastFilename).Str("newFilename", status.Filename).Msg("media player: New media started playing")
		m.emitTrackingStarted(pt, status)
		*lastFilename = status.Filename
		*completed = false
		m.emitPlaybackStatus(pt, status)
		return true
	}

	if status.CompletionPercentage > m.completionThreshold && !*completed {
		m.Logger.Debug().Interface("status", status).Msg("media player: Video completed")
		m.emitVideoCompleted(pt, status)
		*completed = true
	}

	m.emitPlaybackStatus(pt, status)
	return false
}

func (m *Repository) getStatus() (any, error) {
	switch m.Default {
	case "vlc":
		return m.VLC.GetStatus()
	case "mpc-hc":
		return m.MpcHc.GetVariables()
	case "mpv":
		return m.Mpv.GetPlaybackStatus()
	case "iina":
		return m.Iina.GetPlaybackStatus()
	}
	return nil, errors.New("unsupported media player")
}

// clampPercentage ensures CompletionPercentage stays within [0, 1]
func clampPercentage(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// normalizeStatus converts a player-specific status value into the common PlaybackStatus.
// Each player reports position and duration in its own units and shape (VLC over HTTP,
// MPC-HC scraped from HTML, MPV/IINA over IPC); normalization lives here so every caller
// consumes one shape. It returns (nil, false) when the status is not yet usable, in which
// case the caller should retry rather than act on a partial reading.
//
// This is the single seam that used to be duplicated across processStatus and
// processStreamStatus — the only difference between file and stream tracking is the
// PlaybackType stamped on the result.
func normalizeStatus(player string, status any, playbackType PlaybackType) (*PlaybackStatus, bool) {
	ps := &PlaybackStatus{PlaybackType: playbackType}
	switch player {
	case "vlc":
		st, ok := status.(*vlc2.Status)
		if !ok || st == nil {
			return nil, false
		}
		ps.CompletionPercentage = clampPercentage(st.Position)
		ps.Playing = st.State == "playing"
		ps.Filename = st.Information.Category["meta"].Filename
		ps.Duration = int(st.Length * 1000)
		ps.Filepath = st.Information.Category["meta"].Filename // VLC does not provide the filepath, use filename
		ps.CurrentTimeInSeconds = float64(st.Time)
		ps.DurationInSeconds = float64(st.Length)
		return ps, true
	case "mpc-hc":
		st, ok := status.(*mpchc2.Variables)
		if !ok || st == nil || st.Duration == 0 {
			return nil, false
		}
		ps.CompletionPercentage = clampPercentage(st.Position / st.Duration)
		ps.Playing = st.State == 2
		ps.Filename = st.File
		ps.Duration = int(st.Duration)
		ps.Filepath = st.FilePath
		ps.CurrentTimeInSeconds = st.Position / 1000
		ps.DurationInSeconds = st.Duration / 1000
		return ps, true
	case "mpv":
		st, ok := status.(*mpv.Playback)
		if !ok || st == nil || st.Duration == 0 || st.IsRunning == false {
			return nil, false
		}
		ps.CompletionPercentage = clampPercentage(st.Position / st.Duration)
		ps.Playing = !st.Paused
		ps.Filename = st.Filename
		ps.Duration = int(st.Duration)
		ps.Filepath = st.Filepath
		ps.CurrentTimeInSeconds = st.Position
		ps.DurationInSeconds = st.Duration
		return ps, true
	case "iina":
		st, ok := status.(*iina.Playback)
		if !ok || st == nil || st.Duration == 0 || st.IsRunning == false {
			return nil, false
		}
		ps.CompletionPercentage = clampPercentage(st.Position / st.Duration)
		ps.Playing = !st.Paused
		ps.Filename = st.Filename
		ps.Duration = int(st.Duration)
		ps.Filepath = st.Filepath
		ps.CurrentTimeInSeconds = st.Position
		ps.DurationInSeconds = st.Duration
		return ps, true
	default:
		return nil, false
	}
}
