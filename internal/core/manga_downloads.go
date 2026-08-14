package core

import (
	"archive/zip"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// registerMangaDownloadsRoute serves downloaded manga chapters from the download
// directory. It replaces a plain static mount because chapters are stored as CBZ
// archives: a request path of the form {series}/{chapter}.cbz/{entry} streams a
// single page image out of the archive without extracting it.
// Paths without a ".cbz/" segment fall through to regular file serving
// (legacy loose-image chapters, or downloading a whole .cbz archive).
func registerMangaDownloadsRoute(e *echo.Echo, downloadDir string) {
	absBase, err := filepath.Abs(downloadDir)
	if err != nil {
		absBase = downloadDir
	}

	e.GET("/manga-downloads/*", func(c echo.Context) error {
		reqPath, err := url.PathUnescape(c.Param("*"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid path")
		}
		reqPath = filepath.ToSlash(reqPath)

		// Replacing echo's static handler means replacing its path traversal
		// protection as well: resolve and verify the target stays inside the
		// download directory.
		resolve := func(rel string) (string, bool) {
			full := filepath.Join(absBase, filepath.FromSlash(rel))
			return full, strings.HasPrefix(full, absBase+string(os.PathSeparator))
		}

		if idx := strings.Index(strings.ToLower(reqPath), ".cbz/"); idx != -1 {
			cbzRel := reqPath[:idx+len(".cbz")]
			entryName := reqPath[idx+len(".cbz/"):]

			cbzPath, ok := resolve(cbzRel)
			if !ok || entryName == "" || strings.Contains(entryName, "/") {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid path")
			}

			zr, err := zip.OpenReader(cbzPath)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotFound)
			}
			defer zr.Close()

			for _, f := range zr.File {
				if f.Name != entryName {
					continue
				}
				rc, err := f.Open()
				if err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError)
				}
				defer rc.Close()

				contentType := mime.TypeByExtension(filepath.Ext(entryName))
				if contentType == "" {
					contentType = "application/octet-stream"
				}
				// Chapter archives are immutable once written (re-downloads replace the file)
				c.Response().Header().Set("Cache-Control", "private, max-age=604800")
				return c.Stream(http.StatusOK, contentType, rc)
			}

			return echo.NewHTTPError(http.StatusNotFound)
		}

		fullPath, ok := resolve(reqPath)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid path")
		}

		return c.File(fullPath)
	})
}
