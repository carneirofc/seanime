package extension_repo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// IsLocalFileURI reports whether uri points to a file on the local filesystem,
// i.e. a "file://" URL or an absolute filesystem path. Extensions can be
// installed from local sources for development and offline/self-hosted use.
func IsLocalFileURI(uri string) bool {
	_, ok := localFileFromURI(uri)
	return ok
}

// localFileFromURI resolves uri to a local filesystem path when it refers to a
// local file, returning the path and true. It matches:
//   - "file://" URLs (the path component is used), and
//   - absolute filesystem paths with no URL scheme (e.g. "/home/me/ext.json",
//     or "C:\\ext\\ext.json" on Windows).
//
// Relative paths are intentionally not treated as local: they are ambiguous
// with remote references and would depend on the process working directory.
func localFileFromURI(uri string) (string, bool) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", false
	}

	if strings.HasPrefix(strings.ToLower(uri), "file://") {
		u, err := url.Parse(uri)
		if err != nil {
			return "", false
		}
		p := u.Path
		if p == "" {
			// e.g. "file://C:/..." where the drive lands in Host on some parsers
			p = u.Host + u.Path
		}
		p = filepath.FromSlash(p)
		// Strip the leading slash before a Windows drive letter ("/C:/x" -> "C:/x").
		if len(p) >= 3 && p[0] == filepath.Separator && p[2] == ':' {
			p = p[1:]
		}
		if p == "" {
			return "", false
		}
		return p, true
	}

	// Absolute filesystem path with no URL scheme (e.g. "http:", "https:").
	if u, err := url.Parse(uri); err == nil && u.Scheme != "" {
		return "", false
	}
	if filepath.IsAbs(uri) {
		return uri, true
	}

	return "", false
}

// resolveExtensionURI resolves ref against base. If ref is already absolute (has
// a URL scheme like http/https/file, or is an absolute filesystem path) it is
// returned unchanged. A relative ref is resolved against base's directory: for a
// local base against the parent folder on disk, for a remote base via URL
// reference resolution. This lets a local (or remote) repository/manifest
// reference sibling files by relative path.
func resolveExtensionURI(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref
	}

	// Already absolute: a URL with a scheme (http://, https://, file://) or an
	// absolute filesystem path.
	if u, err := url.Parse(ref); err == nil && u.Scheme != "" {
		return ref
	}
	if filepath.IsAbs(ref) {
		return ref
	}

	// Relative ref, resolved against a local base directory.
	if basePath, ok := localFileFromURI(base); ok {
		return filepath.ToSlash(filepath.Join(filepath.Dir(basePath), ref))
	}

	// Relative ref, resolved against a remote base URL.
	if bu, err := url.Parse(base); err == nil && bu.Scheme != "" {
		if ru, err := url.Parse(ref); err == nil {
			return bu.ResolveReference(ru).String()
		}
	}

	return ref
}

// fetchExtensionBytes reads an extension-related resource (marketplace listing,
// manifest, payload, repository JSON) from either the local filesystem or over
// HTTP, supporting private GitHub repositories via newExtensionRequest.
func (r *Repository) fetchExtensionBytes(ctx context.Context, uri string) ([]byte, error) {
	if path, ok := localFileFromURI(uri); ok {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read local file: %w", err)
		}
		return b, nil
	}

	req, err := newExtensionRequest(ctx, uri, r.getGitTokens())
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request, %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resource, %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		hint := ""
		if isGitHubHost(req.URL.Hostname()) && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound) {
			hint = ", private GitHub repositories require a token"
		}
		return nil, fmt.Errorf("failed to fetch resource (status %d)%s: %s", resp.StatusCode, hint, uri)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource, %w", err)
	}
	return b, nil
}
