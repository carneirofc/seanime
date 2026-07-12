package handlers

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
)

type FileEntryInfo struct {
	FullPath string `json:"fullPath"`
	Name     string `json:"name"`
	IsDir    bool   `json:"isDir"`
}

type FileSelectorResponse struct {
	// The directory currently being listed.
	FullPath string `json:"fullPath"`
	// Whether the input path exists on disk.
	Exists bool `json:"exists"`
	// The parent of the listed directory, for "up" navigation.
	BasePath string          `json:"basePath"`
	Content  []FileEntryInfo `json:"content"`
}

// HandleFileSelector
//
//	@summary returns directory content (subdirectories and files) based on the input path.
//	@desc This is used by the file selector component to browse the local filesystem and pick a file.
//	@desc Files can be filtered by extension (e.g. [".json"]); directories are always returned so the user can navigate.
//	@desc It returns a 500 error if the directory cannot be accessed.
//	@route /api/v1/file-selector [POST]
//	@returns handlers.FileSelectorResponse
func (h *Handler) HandleFileSelector(c echo.Context) error {

	type body struct {
		Input      string   `json:"input"`
		Extensions []string `json:"extensions"`
	}
	var request body

	if err := c.Bind(&request); err != nil {
		return h.RespondWithError(c, err)
	}

	if err := h.guardStrictLocalOnlyAction(c); err != nil {
		return err
	}

	// An empty input starts browsing from the user's home directory.
	if strings.TrimSpace(request.Input) == "" {
		if home, err := os.UserHomeDir(); err == nil {
			request.Input = home
		}
	}

	input := filepath.ToSlash(filepath.Clean(request.Input))

	// Determine which directory to list: the input itself when it is an existing
	// directory, otherwise its parent (so a typed/selected file path lists its folder).
	inputExists := false
	listDir := input
	if info, err := os.Stat(input); err == nil {
		inputExists = true
		if !info.IsDir() {
			listDir = filepath.ToSlash(filepath.Dir(input))
		}
	} else {
		listDir = filepath.ToSlash(filepath.Dir(input))
	}

	content, err := getFileSelectorContent(listDir, request.Extensions)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, FileSelectorResponse{
		FullPath: listDir,
		BasePath: filepath.ToSlash(filepath.Dir(listDir)),
		Exists:   inputExists,
		Content:  content,
	})
}

// getFileSelectorContent lists subdirectories and matching files in path.
// Directories are always included; files are included when their extension
// matches one of extensions (case-insensitive). An empty/nil extensions slice
// includes every file.
func getFileSelectorContent(path string, extensions []string) ([]FileEntryInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	normalizedExts := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalizedExts = append(normalizedExts, ext)
	}

	content := make([]FileEntryInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			content = append(content, FileEntryInfo{
				FullPath: filepath.ToSlash(filepath.Join(path, entry.Name())),
				Name:     entry.Name(),
				IsDir:    true,
			})
			continue
		}

		if len(normalizedExts) > 0 {
			entryExt := strings.ToLower(filepath.Ext(entry.Name()))
			matched := false
			for _, ext := range normalizedExts {
				if entryExt == ext {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		content = append(content, FileEntryInfo{
			FullPath: filepath.ToSlash(filepath.Join(path, entry.Name())),
			Name:     entry.Name(),
			IsDir:    false,
		})
	}

	// Directories first, then files, each alphabetical (case-insensitive).
	sort.SliceStable(content, func(i, j int) bool {
		if content[i].IsDir != content[j].IsDir {
			return content[i].IsDir
		}
		return strings.ToLower(content[i].Name) < strings.ToLower(content[j].Name)
	})

	return content, nil
}
