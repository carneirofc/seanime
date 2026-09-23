package extension_repo

import (
	"context"
	"errors"
	"seanime/internal/events"
	"sync"
	"time"
)

// failingThreshold is the number of consecutive failed provider calls after
// which an extension is reported as failing.
const failingThreshold = 5

// ExtensionHealth is the runtime health of a loaded provider extension.
// It is kept in memory only and resets when the extension is reloaded.
type ExtensionHealth struct {
	Calls               int        `json:"calls"`
	Failures            int        `json:"failures"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	LastError           string     `json:"lastError,omitempty"`
	LastErrorAt         *time.Time `json:"lastErrorAt,omitempty"`
	LastSuccessAt       *time.Time `json:"lastSuccessAt,omitempty"`
	// Failing is true once ConsecutiveFailures reaches failingThreshold,
	// and stays true until a call succeeds or the extension is reloaded.
	Failing bool `json:"failing"`
}

// ExtensionFailingEvent is the payload of events.ExtensionFailing.
type ExtensionFailingEvent struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	LastError    string `json:"lastError"`
	AutoDisabled bool   `json:"autoDisabled"`
}

// HealthTracker records the outcome of provider calls per extension.
// A nil *HealthTracker is valid and records nothing.
type HealthTracker struct {
	mu        sync.Mutex
	entries   map[string]*ExtensionHealth
	onFailing func(id string, lastError string)
}

// NewHealthTracker creates a tracker. onFailing, if set, is called in its own
// goroutine each time an extension transitions into the failing state.
func NewHealthTracker(onFailing func(id string, lastError string)) *HealthTracker {
	return &HealthTracker{
		entries:   make(map[string]*ExtensionHealth),
		onFailing: onFailing,
	}
}

// Record registers the outcome of one provider call. Cancelled calls are
// ignored: they mean the caller went away, not that the extension failed.
// It reports whether this call moved the extension into the failing state.
func (h *HealthTracker) Record(id string, err error) (justBecameFailing bool) {
	if h == nil || id == "" || errors.Is(err, context.Canceled) {
		return false
	}

	h.mu.Lock()
	entry, ok := h.entries[id]
	if !ok {
		entry = &ExtensionHealth{}
		h.entries[id] = entry
	}

	now := time.Now()
	entry.Calls++
	if err == nil {
		entry.ConsecutiveFailures = 0
		entry.Failing = false
		entry.LastSuccessAt = &now
	} else {
		entry.Failures++
		entry.ConsecutiveFailures++
		entry.LastError = err.Error()
		entry.LastErrorAt = &now
		if !entry.Failing && entry.ConsecutiveFailures >= failingThreshold {
			entry.Failing = true
			justBecameFailing = true
		}
	}
	lastError := entry.LastError
	h.mu.Unlock()

	if justBecameFailing && h.onFailing != nil {
		go h.onFailing(id, lastError)
	}
	return justBecameFailing
}

// Reset forgets the health of an extension, e.g. after it was reloaded.
func (h *HealthTracker) Reset(id string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	delete(h.entries, id)
	h.mu.Unlock()
}

// Snapshot returns a copy of every tracked entry.
func (h *HealthTracker) Snapshot() map[string]*ExtensionHealth {
	ret := make(map[string]*ExtensionHealth)
	if h == nil {
		return ret
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, entry := range h.entries {
		cp := *entry
		ret[id] = &cp
	}
	return ret
}

// onExtensionFailing is called once each time an extension becomes failing.
// It notifies the client and, when enabled, disables the extension.
func (r *Repository) onExtensionFailing(id string, lastError string) {
	name := r.GetExtensionName(id)
	r.logger.Warn().Str("id", id).Str("lastError", lastError).Msg("extensions: Extension is failing")

	autoDisabled := false
	if r.isAutoDisableFailingEnabled() && !r.builtinExtensions.Has(id) {
		if err := r.SetExternalExtensionDisabled(id, true); err != nil {
			r.logger.Error().Err(err).Str("id", id).Msg("extensions: Failed to auto-disable failing extension")
		} else {
			autoDisabled = true
			r.logger.Info().Str("id", id).Msg("extensions: Auto-disabled failing extension")
		}
	}

	r.wsEventManager.SendEvent(events.ExtensionFailing, ExtensionFailingEvent{
		ID:           id,
		Name:         name,
		LastError:    lastError,
		AutoDisabled: autoDisabled,
	})
}
