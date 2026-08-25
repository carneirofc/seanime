package handlers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"seanime/internal/manga"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

// maxLocalMangaUploadFormValue bounds the non-file parts of an upload form.
// They only carry a series name and a couple of flags.
const maxLocalMangaUploadFormValue = 4 << 10

var (
	errNotAMultipartUpload        = errors.New("expected a multipart upload")
	errNoLocalMangaArchive        = errors.New("no archive was uploaded")
	errMultipleLocalMangaArchives = errors.New("only one archive can be uploaded at a time")
)

// HandleGetLocalMangaLibrary
//
//	@summary returns the local manga library and the entries its series map to.
//	@desc Lists every series directory of the local manga source directory along
//	@desc with its chapter count and the AniList entry currently mapped to it.
//	@route /api/v1/manga/local/library [GET]
//	@returns manga.LocalMangaLibrary
func (h *Handler) HandleGetLocalMangaLibrary(c echo.Context) error {
	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	library, err := h.App.MangaRepository.GetLocalMangaLibrary(collection)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, library)
}

// HandleScanLocalMangaLibrary
//
//	@summary matches the local manga library against the AniList manga collection.
//	@desc Maps each series directory to the manga entry it belongs to. Series that
//	@desc cannot be matched confidently are reported back with candidates so the
//	@desc user can map them manually.
//	@route /api/v1/manga/local/scan [POST]
//	@returns manga.LocalMangaScanResult
func (h *Handler) HandleScanLocalMangaLibrary(c echo.Context) error {
	type body struct {
		Remap          bool `json:"remap"`
		SelectAsSource bool `json:"selectAsSource"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithStatusError(c, http.StatusBadRequest, err)
	}

	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		return h.RespondWithError(c, err)
	}

	result, err := h.App.MangaRepository.ScanLocalMangaLibrary(collection, &manga.LocalMangaScanOptions{
		Remap:          b.Remap,
		SelectAsSource: b.SelectAsSource,
	})
	if err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, result)
}

// HandleUploadLocalMangaArchive
//
//	@summary stores an uploaded zip/cbz archive in the local manga library.
//	@desc Accepts a multipart form with a "file" archive plus an optional "series"
//	@desc directory name, "mediaId" to map the series to, and "overwrite" flag.
//	@desc An archive holding several chapter folders is split into one chapter each.
//	@desc Every chapter is stored as a proper CBZ: pages flattened into reading
//	@desc order plus a ComicInfo.xml built from the AniList entry the series maps to.
//	@desc The archive is streamed to disk as it arrives, so the "file" part must
//	@desc come last: parts sent after it are read too late to apply. One archive
//	@desc per request — a second "file" part is rejected once the first has landed.
//	@route /api/v1/manga/local/upload [POST]
//	@returns manga.LocalMangaUploadResult
func (h *Handler) HandleUploadLocalMangaArchive(c echo.Context) error {
	// The archive is streamed straight to disk rather than buffered into memory,
	// so the form is read part by part instead of through c.FormFile.
	reader, err := c.Request().MultipartReader()
	if err != nil {
		return h.RespondWithStatusError(c, http.StatusBadRequest, errNotAMultipartUpload)
	}

	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		// Metadata is an enrichment, not a prerequisite: the upload still produces
		// valid archives from the folder name alone.
		h.App.Logger.Warn().Err(err).Msg("manga: Storing an upload without AniList metadata")
	}

	result, err := readLocalMangaUpload(reader, func(opts *manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error) {
		opts.Collection = collection
		return h.App.MangaRepository.UploadLocalMangaArchive(opts)
	})
	if err != nil {
		return respondWithLocalMangaUploadError(h, c, err)
	}

	return h.RespondWithData(c, result)
}

// readLocalMangaUpload walks an upload form, collecting the metadata fields and
// handing the archive part to store as it streams in.
//
// The archive is never buffered, so the fields that describe it have to arrive
// first: a part read after the file is read too late to affect it. Clients append
// the file last, and the doc comment above says so.
func readLocalMangaUpload(
	reader *multipart.Reader,
	store func(*manga.LocalMangaUploadOptions) (*manga.LocalMangaUploadResult, error),
) (*manga.LocalMangaUploadResult, error) {
	opts := &manga.LocalMangaUploadOptions{}
	var result *manga.LocalMangaUploadResult

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		if part.FormName() != "file" {
			value, err := readMultipartValue(part)
			if err != nil {
				return nil, err
			}
			applyLocalMangaUploadField(opts, part.FormName(), value)
			continue
		}

		if result != nil {
			_ = part.Close()
			return nil, errMultipleLocalMangaArchives
		}

		opts.Filename = part.FileName()
		if strings.TrimSpace(opts.Series) == "" {
			// Falling back to the filename means a bare upload still lands
			// somewhere sensible instead of being rejected.
			opts.Series = manga.NormalizeLocalMangaTitle(part.FileName())
		}
		opts.Source = part

		result, err = store(opts)
		_ = part.Close()
		if err != nil {
			return nil, err
		}
	}

	if result == nil {
		return nil, errNoLocalMangaArchive
	}

	return result, nil
}

// HandleRepackLocalMangaSeries
//
//	@summary rewrites a local series' chapters as proper CBZ archives.
//	@desc Rebuilds every readable chapter file of the series into a flat CBZ with
//	@desc pages in reading order and a current ComicInfo.xml built from the AniList
//	@desc entry the series maps to. Loose ".zip" files become ".cbz". Formats
//	@desc Seanime cannot read (cbr, pdf) and folders of loose images are reported
//	@desc as skipped rather than touched.
//	@route /api/v1/manga/local/repack [POST]
//	@returns manga.LocalMangaRepackResult
func (h *Handler) HandleRepackLocalMangaSeries(c echo.Context) error {
	type body struct {
		Series string `json:"series"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithStatusError(c, http.StatusBadRequest, err)
	}

	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		// Metadata is an enrichment, not a prerequisite: the repack still produces
		// valid archives from the folder name alone.
		h.App.Logger.Warn().Err(err).Msg("manga: Repacking without AniList metadata")
	}

	result, err := h.App.MangaRepository.RepackLocalMangaSeries(b.Series, collection)
	if err != nil {
		return respondWithLocalMangaUploadError(h, c, err)
	}

	return h.RespondWithData(c, result)
}

// HandleGetLocalMangaChapters
//
//	@summary lists the chapter files of a local manga series.
//	@route /api/v1/manga/local/chapters [GET]
//	@param series - string - true - "Series directory name"
//	@returns []manga.LocalMangaChapter
func (h *Handler) HandleGetLocalMangaChapters(c echo.Context) error {
	chapters, err := h.App.MangaRepository.GetLocalMangaChapters(c.QueryParam("series"))
	if err != nil {
		return respondWithLocalMangaUploadError(h, c, err)
	}

	return h.RespondWithData(c, chapters)
}

// HandleDownloadLocalMangaChapter
//
//	@summary downloads one chapter of a local series as a CBZ file.
//	@desc The chapter is normalized on the way out — a loose ".zip" or a folder of
//	@desc images is served as a proper CBZ with metadata, and the stored files are
//	@desc left untouched.
//	@route /api/v1/manga/local/chapter-archive [GET]
//	@param series - string - true - "Series directory name"
//	@param chapter - string - true - "Chapter filename within the series"
//	@returns nil
func (h *Handler) HandleDownloadLocalMangaChapter(c echo.Context) error {
	series := c.QueryParam("series")
	chapter := c.QueryParam("chapter")
	if series == "" || chapter == "" {
		return h.RespondWithStatusError(c, http.StatusBadRequest, errors.New("series and chapter are required"))
	}

	// The archive is built into the response as it is sent, so the status code is
	// fixed once the first byte is out. Everything that can be known up front is
	// checked here, while a proper error can still be returned.
	if err := h.App.MangaRepository.CheckLocalMangaChapterDownload(series, chapter); err != nil {
		return respondWithLocalMangaUploadError(h, c, err)
	}

	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("manga: Serving a local chapter without AniList metadata")
	}

	filename := fmt.Sprintf("%s - %s", series, manga.LocalMangaChapterDownloadName(chapter))
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filename))
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.comicbook+zip")
	c.Response().WriteHeader(http.StatusOK)

	if err := h.App.MangaRepository.WriteLocalMangaChapterCBZ(c.Response(), series, chapter, collection); err != nil {
		h.App.Logger.Error().Err(err).
			Str("series", series).
			Str("chapter", chapter).
			Msg("manga: Failed to stream the local chapter archive")
		return err
	}

	return nil
}

// HandleDownloadLocalMangaSeries
//
//	@summary downloads every chapter of a local series as a zip of CBZ files.
//	@route /api/v1/manga/local/series-archive [GET]
//	@param series - string - true - "Series directory name"
//	@returns nil
func (h *Handler) HandleDownloadLocalMangaSeries(c echo.Context) error {
	series := c.QueryParam("series")
	if series == "" {
		return h.RespondWithStatusError(c, http.StatusBadRequest, errors.New("series is required"))
	}

	chapters, err := h.App.MangaRepository.GetLocalMangaChapters(series)
	if err != nil {
		return respondWithLocalMangaUploadError(h, c, err)
	}
	// Committing to a 200 and then streaming an empty zip would look like a
	// successful download of nothing.
	if !slices.ContainsFunc(chapters, func(chapter *manga.LocalMangaChapter) bool { return chapter.Downloadable }) {
		return h.RespondWithStatusError(c, http.StatusNotFound, errors.New("this series has no downloadable chapters"))
	}

	collection, err := h.App.GetMangaCollection(false)
	if err != nil {
		h.App.Logger.Warn().Err(err).Msg("manga: Serving a local series without AniList metadata")
	}

	// A whole series is too large to buffer, so this one streams: the headers go
	// out first and a mid-stream failure can only be logged.
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", series+".zip"))
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().WriteHeader(http.StatusOK)

	if _, err := h.App.MangaRepository.WriteLocalMangaSeriesArchive(c.Response(), series, collection); err != nil {
		h.App.Logger.Error().Err(err).Str("series", series).Msg("manga: Failed to stream the local series archive")
		return err
	}

	return nil
}

// HandleMapLocalMangaSeries
//
//	@summary maps a manga entry to a series directory of the local library.
//	@route /api/v1/manga/local/map [POST]
//	@returns bool
func (h *Handler) HandleMapLocalMangaSeries(c echo.Context) error {
	type body struct {
		MediaId int    `json:"mediaId"`
		Series  string `json:"series"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithStatusError(c, http.StatusBadRequest, err)
	}

	if err := h.App.MangaRepository.MapLocalMangaSeries(b.MediaId, b.Series); err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

// HandleDeleteLocalMangaSeries
//
//	@summary deletes a series directory from the local manga library.
//	@route /api/v1/manga/local/series [DELETE]
//	@returns bool
func (h *Handler) HandleDeleteLocalMangaSeries(c echo.Context) error {
	type body struct {
		Series string `json:"series"`
	}

	var b body
	if err := c.Bind(&b); err != nil {
		return h.RespondWithStatusError(c, http.StatusBadRequest, err)
	}

	if err := h.App.MangaRepository.DeleteLocalMangaSeries(b.Series); err != nil {
		return h.RespondWithError(c, err)
	}

	return h.RespondWithData(c, true)
}

func applyLocalMangaUploadField(opts *manga.LocalMangaUploadOptions, name string, value string) {
	switch name {
	case "series":
		opts.Series = strings.TrimSpace(value)
	case "mediaId":
		if mediaId, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			opts.MediaId = mediaId
		}
	case "overwrite":
		opts.Overwrite = value == "true" || value == "1"
	}
}

func readMultipartValue(part *multipart.Part) (string, error) {
	defer part.Close()

	value, err := io.ReadAll(io.LimitReader(part, maxLocalMangaUploadFormValue))
	if err != nil {
		return "", err
	}

	return string(value), nil
}

// respondWithLocalMangaUploadError maps the upload failures the client can act
// on to the status codes that say so, instead of a blanket 500.
func respondWithLocalMangaUploadError(h *Handler, c echo.Context, err error) error {
	switch {
	case errors.Is(err, manga.ErrLocalMangaChapterExists):
		return h.RespondWithStatusError(c, http.StatusConflict, err)
	case errors.Is(err, manga.ErrLocalMangaSeriesNotFound),
		errors.Is(err, manga.ErrLocalMangaChapterNotFound):
		return h.RespondWithStatusError(c, http.StatusNotFound, err)
	case errors.Is(err, manga.ErrLocalMangaUploadTooLarge):
		return h.RespondWithStatusError(c, http.StatusRequestEntityTooLarge, err)
	case errors.Is(err, manga.ErrLocalMangaUploadNotAnArchive),
		errors.Is(err, manga.ErrLocalMangaUploadNoPages),
		errors.Is(err, manga.ErrLocalMangaUploadUnsafeEntry),
		errors.Is(err, manga.ErrInvalidLocalMangaName),
		errors.Is(err, errNoLocalMangaArchive),
		errors.Is(err, errMultipleLocalMangaArchives):
		return h.RespondWithStatusError(c, http.StatusBadRequest, err)
	default:
		return h.RespondWithError(c, err)
	}
}
