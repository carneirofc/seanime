package handlers

import "github.com/labstack/echo/v4"

// DiskUsageInfo contains disk usage metrics for a single path.
type DiskUsageInfo struct {
	Path    string  `json:"path"`
	TotalGB float64 `json:"totalGB"`
	UsedGB  float64 `json:"usedGB"`
	FreeGB  float64 `json:"freeGB"`
	UsedPct float64 `json:"usedPct"`
	FreePct float64 `json:"freePct"`
}

// LibraryDiskUsageResponse holds disk usage info for all library paths.
type LibraryDiskUsageResponse struct {
	Paths []DiskUsageInfo `json:"paths"`
}

// HandleGetLibraryDiskUsage
//
//	@summary returns disk usage for all configured library paths.
//	@desc Returns total, used, and free disk space (in GB and percent) for each configured library path.
//	@route /api/v1/library/disk-usage [GET]
//	@returns handlers.LibraryDiskUsageResponse
func (h *Handler) HandleGetLibraryDiskUsage(c echo.Context) error {
	result := LibraryDiskUsageResponse{Paths: []DiskUsageInfo{}}

	if h.App.Settings == nil {
		return h.RespondWithData(c, result)
	}

	lib := h.App.Settings.GetLibrary()

	seen := map[string]bool{}
	var paths []string
	if lib.LibraryPath != "" {
		seen[lib.LibraryPath] = true
		paths = append(paths, lib.LibraryPath)
	}
	for _, p := range lib.LibraryPaths {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}

	for _, path := range paths {
		info, err := getPathDiskUsage(path)
		if err != nil {
			continue
		}
		result.Paths = append(result.Paths, info)
	}

	return h.RespondWithData(c, result)
}
