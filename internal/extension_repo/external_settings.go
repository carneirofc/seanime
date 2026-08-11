package extension_repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"seanime/internal/util/filecache"
)

const (
	ExtensionSettingsKey    = "1"
	ExtensionSettingsBucket = "extension-settings"
)

type StoredExtensionSettingsData struct {
	DisabledExtensionIds map[string]bool `json:"disabledExtensionIds"`
	// GitTokens maps a normalized git repository pattern ("host", "host/owner"
	// or "host/owner/repo") to an access token used when fetching extension
	// resources (manifests, payloads, repositories) from matching URLs.
	GitTokens map[string]string `json:"gitTokens,omitempty"`
	mu        sync.Mutex        `json:"-"`
}

func defaultExtensionSettings() *StoredExtensionSettingsData {
	return &StoredExtensionSettingsData{
		DisabledExtensionIds: map[string]bool{},
		GitTokens:            map[string]string{},
	}
}

func (r *Repository) GetExtensionSettings() *StoredExtensionSettingsData {
	bucket := filecache.NewPermanentBucket(ExtensionSettingsBucket)

	var settings StoredExtensionSettingsData
	found, _ := r.fileCacher.GetPerm(bucket, ExtensionSettingsKey, &settings)
	if !found {
		settings := defaultExtensionSettings()
		r.fileCacher.SetPerm(bucket, ExtensionSettingsKey, settings)
		return settings
	}

	if settings.DisabledExtensionIds == nil {
		settings.DisabledExtensionIds = map[string]bool{}
	}
	if settings.GitTokens == nil {
		settings.GitTokens = map[string]string{}
	}

	return &settings
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Git repository tokens
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// GitTokenInfo describes a configured git repository token with the token value masked.
type GitTokenInfo struct {
	Repository  string `json:"repository"`
	MaskedToken string `json:"maskedToken"`
}

func maskGitToken(token string) string {
	if len(token) <= 4 {
		return "••••"
	}
	return "••••" + token[len(token)-4:]
}

// getGitTokens returns a copy of the configured git repository tokens (pattern -> token).
func (r *Repository) getGitTokens() map[string]string {
	settings := r.GetExtensionSettings()
	settings.mu.Lock()
	defer settings.mu.Unlock()
	ret := make(map[string]string, len(settings.GitTokens))
	for k, v := range settings.GitTokens {
		ret[k] = v
	}
	return ret
}

// ListGitTokens returns the configured git repository tokens with masked values,
// sorted by repository pattern.
func (r *Repository) ListGitTokens() []GitTokenInfo {
	tokens := r.getGitTokens()
	ret := make([]GitTokenInfo, 0, len(tokens))
	for repo, token := range tokens {
		ret = append(ret, GitTokenInfo{Repository: repo, MaskedToken: maskGitToken(token)})
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Repository < ret[j].Repository })
	return ret
}

// SetGitToken stores an access token for the given repository pattern
// (e.g. "github.com/owner/repo", "https://gitlab.com/owner", a bare host).
func (r *Repository) SetGitToken(repository string, token string) error {
	pattern, err := normalizeGitRepoPattern(repository)
	if err != nil {
		return err
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("token is empty")
	}

	bucket := filecache.NewPermanentBucket(ExtensionSettingsBucket)
	settings := r.GetExtensionSettings()
	settings.mu.Lock()
	defer settings.mu.Unlock()
	settings.GitTokens[pattern] = strings.TrimSpace(token)
	return r.fileCacher.SetPerm(bucket, ExtensionSettingsKey, settings)
}

// RemoveGitToken removes the token stored for the given repository pattern.
func (r *Repository) RemoveGitToken(repository string) error {
	pattern, err := normalizeGitRepoPattern(repository)
	if err != nil {
		return err
	}

	bucket := filecache.NewPermanentBucket(ExtensionSettingsBucket)
	settings := r.GetExtensionSettings()
	settings.mu.Lock()
	defer settings.mu.Unlock()
	if _, ok := settings.GitTokens[pattern]; !ok {
		return fmt.Errorf("no token configured for %q", pattern)
	}
	delete(settings.GitTokens, pattern)
	return r.fileCacher.SetPerm(bucket, ExtensionSettingsKey, settings)
}

func (r *Repository) SetExternalExtensionDisabled(id string, disabled bool) error {
	if id == "" {
		return fmt.Errorf("id is empty")
	}
	if err := isValidExtensionID(id); err != nil {
		return err
	}

	if _, err := os.Stat(r.externalExtensionFilepath(id)); err != nil {
		return fmt.Errorf("extension not found")
	}

	bucket := filecache.NewPermanentBucket(ExtensionSettingsBucket)
	settings := r.GetExtensionSettings()
	if disabled {
		settings.mu.Lock()
		defer settings.mu.Unlock()
		settings.DisabledExtensionIds[id] = true
	} else {
		settings.mu.Lock()
		defer settings.mu.Unlock()
		delete(settings.DisabledExtensionIds, id)
	}

	if err := r.fileCacher.SetPerm(bucket, ExtensionSettingsKey, settings); err != nil {
		return err
	}

	r.reloadExtension(id)
	return nil
}

func (r *Repository) isExtensionDisabled(id string) bool {
	settings := r.GetExtensionSettings()
	settings.mu.Lock()
	defer settings.mu.Unlock()
	return settings.DisabledExtensionIds[id]
}

func (r *Repository) removeExtensionFromStoredSettings(id string) {
	bucket := filecache.NewPermanentBucket(ExtensionSettingsBucket)
	settings := r.GetExtensionSettings()
	settings.mu.Lock()
	defer settings.mu.Unlock()
	delete(settings.DisabledExtensionIds, id)
	r.fileCacher.SetPerm(bucket, ExtensionSettingsKey, settings)
}

func (r *Repository) externalExtensionFilepath(id string) string {
	return filepath.Join(r.extensionDir, id+".json")
}
