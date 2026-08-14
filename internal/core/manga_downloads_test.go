package core

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func newMangaDownloadsTestServer(t *testing.T) (*echo.Echo, string) {
	t.Helper()

	downloadDir := t.TempDir()

	// New layout: comick_1/0001_ch-1.cbz containing one page + ComicInfo.xml
	seriesDir := filepath.Join(downloadDir, "comick_1")
	require.NoError(t, os.MkdirAll(seriesDir, os.ModePerm))

	cbzFile, err := os.Create(filepath.Join(seriesDir, "0001_ch-1.cbz"))
	require.NoError(t, err)
	zw := zip.NewWriter(cbzFile)
	w, err := zw.Create("001.png")
	require.NoError(t, err)
	_, err = w.Write([]byte("png-bytes"))
	require.NoError(t, err)
	w, err = zw.Create("ComicInfo.xml")
	require.NoError(t, err)
	_, err = w.Write([]byte(`<?xml version="1.0"?><ComicInfo></ComicInfo>`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, cbzFile.Close())

	// Legacy layout: mangapill_2_ch_3/01.png
	legacyDir := filepath.Join(downloadDir, "mangapill_2_ch_3")
	require.NoError(t, os.MkdirAll(legacyDir, os.ModePerm))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "01.png"), []byte("legacy-bytes"), 0644))

	// A file outside the download directory that must never be reachable
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(downloadDir), "secret.txt"), []byte("secret"), 0644))

	e := echo.New()
	registerMangaDownloadsRoute(e, downloadDir)

	return e, downloadDir
}

func TestMangaDownloadsRouteServesCBZEntry(t *testing.T) {
	e, _ := newMangaDownloadsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/manga-downloads/comick_1/0001_ch-1.cbz/001.png", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "png-bytes", rec.Body.String())
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.NotEmpty(t, rec.Header().Get("Cache-Control"))
}

func TestMangaDownloadsRouteServesLegacyFile(t *testing.T) {
	e, _ := newMangaDownloadsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/manga-downloads/mangapill_2_ch_3/01.png", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "legacy-bytes", rec.Body.String())
}

func TestMangaDownloadsRouteServesWholeArchive(t *testing.T) {
	e, _ := newMangaDownloadsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/manga-downloads/comick_1/0001_ch-1.cbz", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, rec.Body.Bytes())
}

func TestMangaDownloadsRouteMissingEntry(t *testing.T) {
	e, _ := newMangaDownloadsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/manga-downloads/comick_1/0001_ch-1.cbz/999.png", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMangaDownloadsRouteRejectsTraversal(t *testing.T) {
	e, _ := newMangaDownloadsTestServer(t)

	for _, target := range []string{
		"/manga-downloads/../secret.txt",
		"/manga-downloads/%2E%2E/secret.txt",
		"/manga-downloads/comick_1/..%2F..%2Fsecret.txt",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		require.NotEqual(t, http.StatusOK, rec.Code, "path %s must not be served", target)
		require.NotContains(t, rec.Body.String(), "secret", "path %s must not leak file content", target)
	}
}
