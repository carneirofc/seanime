package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"seanime/internal/security"
	"seanime/internal/util"
	"time"

	"github.com/imroc/req/v3"
	"github.com/labstack/echo/v4"
)

// imageRequestTimeout bounds how long a single image fetch may take so a stalled
// connection can't hang a download (or the proxy endpoint) indefinitely.
const imageRequestTimeout = 60 * time.Second

type ImageProxy struct{}

func (ip *ImageProxy) GetImage(url string, headers map[string]string) ([]byte, string, error) {
	// The URL was pre-checked by the caller, but the address it resolves to at
	// connect time — and after any redirect — is only enforceable here.
	request := req.C().
		SetTimeout(imageRequestTimeout).
		SetDial(security.HardenedDialContext(30*time.Second, 30*time.Second)).
		DisableAutoReadResponse().
		NewRequest()

	for key, value := range headers {
		request.SetHeader(key, value)
	}

	resp, err := request.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	// Reject error responses up-front. Without this, an HTML error page (403/404/etc.)
	// would be returned as "image" bytes and fail to decode downstream.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("image proxy: unexpected status %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return body, resp.Header.Get(echo.HeaderContentType), nil
}

func (ip *ImageProxy) setHeaders(c echo.Context, contentType string) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	headers := c.Response().Header()
	headers.Set(echo.HeaderContentType, contentType)
	headers.Set(echo.HeaderCacheControl, "public, max-age=31536000")
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	headers.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	headers.Set("Cross-Origin-Resource-Policy", "cross-origin")
}

func (ip *ImageProxy) ProxyImage(c echo.Context) (err error) {
	defer util.HandlePanicInModuleWithError("util/ImageProxy", &err)

	url := c.QueryParam("url")
	headersJSON := c.QueryParam("headers")

	if url == "" {
		return c.String(echo.ErrBadRequest.Code, "No URL provided")
	}

	if err := security.ValidateOutboundUrl(url); err != nil {
		return c.String(http.StatusForbidden, err.Error())
	}

	headers := make(map[string]string)
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			return c.String(echo.ErrBadRequest.Code, "Error parsing headers JSON")
		}
	}

	imageBuffer, contentType, err := ip.GetImage(url, headers)
	if err != nil {
		return c.String(echo.ErrInternalServerError.Code, "Error fetching image")
	}
	ip.setHeaders(c, contentType)

	return c.Blob(http.StatusOK, c.Response().Header().Get(echo.HeaderContentType), imageBuffer)
}
