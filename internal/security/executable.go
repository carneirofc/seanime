package security

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// Executable path validation.
//
// Several settings name a binary to spawn — ffmpeg, ffprobe, the media player, the
// torrent client. They arrive over HTTP, which makes them a choice of what code runs
// on the host rather than a preference.
//
// The invariant enforced here is narrow on purpose, so it does not break the many
// legitimate install layouts (system packages, Homebrew, Program Files, Flatpak):
// a binary may not live anywhere Seanime itself can write. Those directories — the
// data dir, caches, download and library roots — are exactly the places an attacker
// can plant a file through the ordinary download, extract and library-write paths,
// and turning one of them into an executable is what converts a file write into code
// execution.

var (
	untrustedExecutableRootsMu sync.RWMutex
	untrustedExecutableRoots   []string
)

// SetUntrustedExecutableRoots registers the directories Seanime can write to.
// Anything under them is refused as an executable.
func SetUntrustedExecutableRoots(roots []string) {
	normalized := make([]string, 0, len(roots))
	for _, root := range roots {
		if cleaned := normalizeExecutablePath(root); cleaned != "" {
			normalized = append(normalized, cleaned)
		}
	}

	slices.Sort(normalized)

	untrustedExecutableRootsMu.Lock()
	untrustedExecutableRoots = slices.Compact(normalized)
	untrustedExecutableRootsMu.Unlock()
}

// GetUntrustedExecutableRoots returns the registered writable roots.
func GetUntrustedExecutableRoots() []string {
	untrustedExecutableRootsMu.RLock()
	defer untrustedExecutableRootsMu.RUnlock()

	return slices.Clone(untrustedExecutableRoots)
}

// ValidateExecutablePath resolves a configured binary and reports whether it is
// safe to spawn. It returns the resolved absolute path so callers can execute
// exactly what was checked rather than re-resolving (and possibly resolving
// differently) afterwards.
//
// An empty path is not an error: callers treat it as "use the default", and the
// default is resolved through PATH by the same rules.
func ValidateExecutablePath(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", nil
	}

	resolved := trimmed
	if !strings.ContainsRune(trimmed, os.PathSeparator) && !strings.ContainsRune(trimmed, '/') {
		// A bare name: let PATH decide, then validate what PATH produced.
		found, err := exec.LookPath(trimmed)
		if err != nil {
			return "", fmt.Errorf("executable %q was not found in PATH: %w", trimmed, err)
		}
		resolved = found
	}

	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("could not resolve executable %q: %w", rawPath, err)
	}

	// Resolve symlinks so a link planted in a writable directory cannot be used to
	// point at (or away from) a checked location.
	if real, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = real
	}

	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("executable %q is not accessible: %w", rawPath, err)
	}

	if info.IsDir() || !info.Mode().IsRegular() {
		return "", fmt.Errorf("executable %q is not a regular file", rawPath)
	}

	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("executable %q is not executable", rawPath)
	}

	if root, blocked := isUnderUntrustedRoot(absolute); blocked {
		return "", fmt.Errorf("executable %q is refused: %q is inside the writable directory %q", rawPath, absolute, root)
	}

	return absolute, nil
}

func isUnderUntrustedRoot(path string) (string, bool) {
	normalizedPath := normalizeExecutablePath(path)
	if normalizedPath == "" {
		return "", false
	}

	for _, root := range GetUntrustedExecutableRoots() {
		if normalizedPath == root {
			return root, true
		}

		prefix := root
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		if strings.HasPrefix(normalizedPath, prefix) {
			return root, true
		}
	}

	return "", false
}

func normalizeExecutablePath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return ""
	}

	absolute, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return ""
	}

	if real, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = real
	}

	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.Clean(absolute))
	}

	return filepath.Clean(absolute)
}
