package manga

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ProviderRequestLog represents a single HTTP request made by a manga provider.
// These entries are stored in-memory and exposed via the diagnostics API endpoint.
type ProviderRequestLog struct {
	Timestamp       time.Time         `json:"timestamp"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	StatusCode      int               `json:"statusCode"`
	StatusText      string            `json:"statusText"`
	DurationMs      int64             `json:"durationMs"`
	Error           string            `json:"error,omitempty"`
	RequestHeaders  map[string]string `json:"requestHeaders,omitempty"`
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`
}

// ProviderDiagnostics holds the recent request log for a single manga provider.
type ProviderDiagnostics struct {
	ProviderID string                `json:"providerId"`
	Logs       []*ProviderRequestLog `json:"logs"`
	mu         sync.Mutex
	maxLogs    int
}

func newProviderDiagnostics(providerID string, maxLogs int) *ProviderDiagnostics {
	return &ProviderDiagnostics{
		ProviderID: providerID,
		Logs:       make([]*ProviderRequestLog, 0, maxLogs),
		maxLogs:    maxLogs,
	}
}

// Append adds a request log entry. If the buffer is full, the oldest entry is evicted.
func (pd *ProviderDiagnostics) Append(entry *ProviderRequestLog) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if len(pd.Logs) >= pd.maxLogs {
		// Shift left by 1 (drop oldest)
		copy(pd.Logs, pd.Logs[1:])
		pd.Logs[len(pd.Logs)-1] = entry
	} else {
		pd.Logs = append(pd.Logs, entry)
	}
}

// GetLogs returns a copy of the current log entries (newest last).
func (pd *ProviderDiagnostics) GetLogs() []*ProviderRequestLog {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	out := make([]*ProviderRequestLog, len(pd.Logs))
	copy(out, pd.Logs)
	return out
}

// DiagnosticsStore is a global thread-safe store for per-provider request diagnostics.
type DiagnosticsStore struct {
	store   sync.Map // map[string]*ProviderDiagnostics
	maxLogs int
}

// NewDiagnosticsStore creates a new diagnostics store with the given max entries per provider.
func NewDiagnosticsStore(maxLogs int) *DiagnosticsStore {
	return &DiagnosticsStore{maxLogs: maxLogs}
}

// LogRequest appends a request log entry for the given provider.
// The Timestamp field is automatically set to the current time if not already set.
func (ds *DiagnosticsStore) LogRequest(providerID string, entry *ProviderRequestLog) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	actual, _ := ds.store.LoadOrStore(providerID, newProviderDiagnostics(providerID, ds.maxLogs))
	actual.(*ProviderDiagnostics).Append(entry)
}

// GetProviderLogs returns the recent request logs for the given provider.
func (ds *DiagnosticsStore) GetProviderLogs(providerID string) []*ProviderRequestLog {
	val, ok := ds.store.Load(providerID)
	if !ok {
		return []*ProviderRequestLog{}
	}
	return val.(*ProviderDiagnostics).GetLogs()
}

// ClearProvider removes all diagnostics for a provider.
func (ds *DiagnosticsStore) ClearProvider(providerID string) {
	ds.store.Delete(providerID)
}

// Global instance — initialized by the manga repository.
var GlobalDiagnosticsStore = NewDiagnosticsStore(50)

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Cache Info
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// ProviderCacheInfo describes a single manga cache bucket on disk for a specific provider+media.
type ProviderCacheInfo struct {
	Provider      string          `json:"provider"`
	BucketType    string          `json:"bucketType"`
	FileName      string          `json:"fileName"`
	FilePath      string          `json:"filePath"`
	FileSizeBytes int64           `json:"fileSizeBytes"`
	KeyCount      int             `json:"keyCount"`
	Keys          []*CacheKeyInfo `json:"keys"`
}

// CacheKeyInfo describes a single key within a cache bucket file.
type CacheKeyInfo struct {
	Key        string     `json:"key"`
	Expiration *time.Time `json:"expiration,omitempty"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
	IsExpired  bool       `json:"isExpired"`
}

// GetMangaCacheInfo scans the cache directory for all manga bucket files associated with the given mediaId
// and returns metadata about each — provider, bucket type, file path, size, and per-key expiration info.
func GetMangaCacheInfo(cacheDir string, mediaId int) []*ProviderCacheInfo {
	result := make([]*ProviderCacheInfo, 0)

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return result
	}

	mediaIdStr := strconv.Itoa(mediaId)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".cache") {
			continue
		}
		if !strings.HasPrefix(name, "manga_") {
			continue
		}
		if !strings.Contains(name, mediaIdStr) {
			continue
		}

		// Parse the filename to extract provider and bucket type
		provider, bt, parsedId, ok := ParseChapterContainerFileName(name)
		if !ok || parsedId != mediaId {
			continue
		}

		fullPath := filepath.Join(cacheDir, name)
		info, statErr := os.Stat(fullPath)
		var fileSize int64
		if statErr == nil {
			fileSize = info.Size()
		}

		// Read the cache file to extract key metadata
		keys := readCacheFileKeys(fullPath)

		cacheInfo := &ProviderCacheInfo{
			Provider:      provider,
			BucketType:    string(bt),
			FileName:      name,
			FilePath:      fullPath,
			FileSizeBytes: fileSize,
			KeyCount:      len(keys),
			Keys:          keys,
		}
		result = append(result, cacheInfo)
	}

	return result
}

// readCacheFileKeys reads a .cache file (JSON map of string → cacheItem) and returns metadata about each key.
func readCacheFileKeys(filePath string) []*CacheKeyInfo {
	result := make([]*CacheKeyInfo, 0)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return result
	}

	// The cache file is a JSON object: { "key": { "value": ..., "expiration": ..., "updated_at": ... } }
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return result
	}

	now := time.Now()
	for key, rawItem := range rawMap {
		var item struct {
			Expiration *time.Time `json:"expiration,omitempty"`
			UpdatedAt  *time.Time `json:"updated_at,omitempty"`
		}
		if err := json.Unmarshal(rawItem, &item); err != nil {
			result = append(result, &CacheKeyInfo{Key: key})
			continue
		}

		isExpired := false
		if item.Expiration != nil && now.After(*item.Expiration) {
			isExpired = true
		}

		result = append(result, &CacheKeyInfo{
			Key:        key,
			Expiration: item.Expiration,
			UpdatedAt:  item.UpdatedAt,
			IsExpired:  isExpired,
		})
	}

	return result
}
