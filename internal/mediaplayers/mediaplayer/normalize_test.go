package mediaplayer

import (
	mpchc2 "seanime/internal/mediaplayers/mpchc"
	"seanime/internal/mediaplayers/mpv"
	vlc2 "seanime/internal/mediaplayers/vlc"
	"testing"
)

// normalizeStatus is the single seam through which every player's status is turned into a
// PlaybackStatus. These tests exercise that seam directly — the behaviour that used to be
// duplicated (and only reachable through a live player) across processStatus and
// processStreamStatus.
func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		name         string
		player       string
		status       any
		playbackType PlaybackType
		wantOK       bool
		wantCompl    float64
		wantPlaying  bool
		wantDuration int
		wantCurrSec  float64
		wantDurSec   float64
	}{
		{
			name:         "vlc file",
			player:       "vlc",
			status:       &vlc2.Status{State: "playing", Position: 0.3, Length: 100, Time: 30},
			playbackType: PlaybackTypeFile,
			wantOK:       true,
			wantCompl:    0.3,
			wantPlaying:  true,
			wantDuration: 100000, // Length seconds -> ms
			wantCurrSec:  30,
			wantDurSec:   100,
		},
		{
			name:         "vlc clamps position above 1",
			player:       "vlc",
			status:       &vlc2.Status{State: "paused", Position: 1.5, Length: 10, Time: 10},
			playbackType: PlaybackTypeStream,
			wantOK:       true,
			wantCompl:    1, // clamped
			wantPlaying:  false,
			wantDuration: 10000,
			wantCurrSec:  10,
			wantDurSec:   10,
		},
		{
			name:         "mpc-hc stream half done",
			player:       "mpc-hc",
			status:       &mpchc2.Variables{State: 2, Position: 5000, Duration: 10000, File: "ep.mkv", FilePath: "C:/ep.mkv"},
			playbackType: PlaybackTypeStream,
			wantOK:       true,
			wantCompl:    0.5,
			wantPlaying:  true,
			wantDuration: 10000, // ms passed through
			wantCurrSec:  5,     // ms -> s
			wantDurSec:   10,
		},
		{
			// Previously the stream path lacked this guard and divided by zero. The unified
			// seam now guards both paths and asks the caller to retry.
			name:         "mpc-hc zero duration is not usable",
			player:       "mpc-hc",
			status:       &mpchc2.Variables{State: 2, Position: 0, Duration: 0, File: "ep.mkv"},
			playbackType: PlaybackTypeStream,
			wantOK:       false,
		},
		{
			name:         "mpv file half done",
			player:       "mpv",
			status:       &mpv.Playback{IsRunning: true, Paused: false, Position: 45, Duration: 90, Filename: "ep.mkv", Filepath: "/x/ep.mkv"},
			playbackType: PlaybackTypeFile,
			wantOK:       true,
			wantCompl:    0.5,
			wantPlaying:  true,
			wantDuration: 90,
			wantCurrSec:  45,
			wantDurSec:   90,
		},
		{
			name:         "mpv not running is not usable",
			player:       "mpv",
			status:       &mpv.Playback{IsRunning: false, Duration: 90},
			playbackType: PlaybackTypeFile,
			wantOK:       false,
		},
		{
			name:         "unknown player is not usable",
			player:       "foobar",
			status:       &mpv.Playback{IsRunning: true, Duration: 90},
			playbackType: PlaybackTypeFile,
			wantOK:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeStatus(tt.player, tt.status, tt.playbackType)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				if got != nil {
					t.Fatalf("expected nil status on failure, got %+v", got)
				}
				return
			}
			if got.PlaybackType != tt.playbackType {
				t.Errorf("PlaybackType = %q, want %q", got.PlaybackType, tt.playbackType)
			}
			if got.CompletionPercentage != tt.wantCompl {
				t.Errorf("CompletionPercentage = %v, want %v", got.CompletionPercentage, tt.wantCompl)
			}
			if got.Playing != tt.wantPlaying {
				t.Errorf("Playing = %v, want %v", got.Playing, tt.wantPlaying)
			}
			if got.Duration != tt.wantDuration {
				t.Errorf("Duration = %v, want %v", got.Duration, tt.wantDuration)
			}
			if got.CurrentTimeInSeconds != tt.wantCurrSec {
				t.Errorf("CurrentTimeInSeconds = %v, want %v", got.CurrentTimeInSeconds, tt.wantCurrSec)
			}
			if got.DurationInSeconds != tt.wantDurSec {
				t.Errorf("DurationInSeconds = %v, want %v", got.DurationInSeconds, tt.wantDurSec)
			}
		})
	}
}
