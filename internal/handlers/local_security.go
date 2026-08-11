package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"runtime"
	"seanime/internal/database/models"
	"seanime/internal/security"
	"strings"

	"github.com/labstack/echo/v4"
)

// Request-boundary helpers.
//
// Two kinds of control live in this file, and the distinction matters:
//
//   - Deny-only controls (isRequestPermitted, the CORS predicates, the cross-site
//     checks). These may REJECT a request — that is how a passwordless install stays
//     off the internet, and how a session cookie is kept from being ridden by a
//     random website. They never grant anything.
//
//   - Capability checks (the guard* functions). These authorize privileged actions:
//     spawning processes, walking the filesystem, loading third-party code, replacing
//     the binary. They consult operator configuration and nothing else.
//
// What used to sit here was a third kind: request provenance as an authorization
// grant — "your source IP is private and your Origin header says localhost, so you
// may execute code." Every input to that is forgeable by anyone who can open a
// socket to the server, and inside a container orchestrator every peer is on a
// private address to begin with. It is gone. No request proves trustworthiness.

var errStrictFilesystemPathDenied = errors.New("this path is outside the configured library and download directories")

var errGuardResponseWritten = errors.New("guard response written")

// errCapabilityDenied explains which capability is missing and where to grant it,
// so an operator hitting a 403 does not have to guess.
func errCapabilityDenied(capability string) error {
	return fmt.Errorf("this action requires the %q capability; add it to server.capabilities in config.toml", capability)
}

// requireCapability denies a privileged action unless the operator granted the
// capability. It takes no request-derived input on purpose.
func (h *Handler) requireCapability(c echo.Context, capability string) error {
	if security.Allows(capability) {
		return nil
	}

	return respondWithAbort(c, http.StatusForbidden, errCapabilityDenied(capability))
}

func respondWithAbort(c echo.Context, code int, err error) error {
	if c == nil {
		return err
	}

	if writeErr := c.JSON(code, NewErrorResponse(err)); writeErr != nil {
		return writeErr
	}

	return errGuardResponseWritten
}

// hasServerAuth reports whether the server has an authentication gate (OIDC
// login or a server password). Most passwordless request-boundary heuristics
// are skipped when an auth gate exists, since the auth middleware enforces it.
func (h *Handler) hasServerAuth() bool {
	if h == nil || h.App == nil || h.App.Config == nil {
		return false
	}
	return h.App.IsOidcMode() || h.App.Config.Server.Password != ""
}

// isStrictModeSensitive reports whether /status must answer with the restricted
// payload. It used to exempt callers that looked local; since that could be forged,
// the exemption is gone and a passwordless strict-mode server now redacts for
// everyone.
func isStrictModeSensitive(hasServerAuth bool) bool {
	return !hasServerAuth && security.IsStrict()
}

func reqHasOriginMetadata(req *http.Request) bool {
	if req == nil {
		return false
	}

	return strings.TrimSpace(req.Header.Get("Origin")) != "" || strings.TrimSpace(req.Header.Get("Referer")) != ""
}

func isCrossSiteBrowserRequest(req *http.Request) bool {
	if req == nil {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(req.Header.Get("Sec-Fetch-Site")), "cross-site")
}

func isPathNeedingTrustedLocalBoundary(path string) bool {
	return path == "/events" || strings.HasPrefix(path, "/api/")
}

func isHardenedTrustedRequestHost(req *http.Request) bool {
	view := createRequestBoundaryView(req)
	if view.hostname == "" {
		return false
	}

	host := view.hostname
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return addr.IsLoopback()
}

func isTrustedHardenedOriginURL(parsed *url.URL) bool {
	if parsed == nil {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return addr.IsLoopback()
}

func isRequestFromTrustedHardenedOrigin(req *http.Request) bool {
	if req == nil {
		return false
	}

	rawOrigin := strings.TrimSpace(req.Header.Get("Origin"))
	if rawOrigin == "" {
		rawOrigin = strings.TrimSpace(req.Header.Get("Referer"))
	}
	parsed, ok := parseTrustedOrigin(rawOrigin)
	if !ok {
		return false
	}

	return isTrustedHardenedOriginURL(parsed)
}

func hasHardenedLocalClientBoundary(req *http.Request) bool {
	if req == nil {
		return false
	}

	view := createRequestBoundaryView(req)
	if !view.clientIP.IsValid() || !view.clientIP.IsLoopback() {
		return false
	}

	if hasForwardedHeaders(req) && !view.trustedProxy {
		return false
	}

	return true
}

func isRequestFromTrustedHardenedLocal(req *http.Request) bool {
	if !hasHardenedLocalClientBoundary(req) {
		return false
	}

	if !isHardenedTrustedRequestHost(req) {
		return false
	}

	return isRequestFromTrustedHardenedOrigin(req)
}

// isTrustedRequestHost checks if the request originates from a trusted host such as localhost, a loopback, or private network address.
func isTrustedRequestHost(req *http.Request) bool {
	view := createRequestBoundaryView(req)
	if view.hostname == "" {
		return false
	}

	host := view.hostname
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return addr.IsLoopback() || addr.IsPrivate()
}

// isRequestPermitted determines if an HTTP request is permitted based on the server auth gate, access allowlist, and request origin metadata.
func isRequestPermitted(req *http.Request, hasServerAuth bool, accessAllowlist []string) bool {
	if hasServerAuth || security.IsLax() {
		return true
	}

	if security.IsHardened() {
		allowlistedHost := isAllowlistedRequestHost(req, accessAllowlist)
		if !isHardenedTrustedRequestHost(req) && !allowlistedHost {
			return false
		}

		if !reqHasOriginMetadata(req) {
			if isCrossSiteBrowserRequest(req) {
				return false
			}
			if allowlistedHost {
				return true
			}
			return hasHardenedLocalClientBoundary(req)
		}

		if isRequestFromAllowlistedOrigin(req, accessAllowlist) {
			return true
		}

		return isRequestFromTrustedHardenedLocal(req)
	}

	if !isTrustedRequestHost(req) && !isAllowlistedRequestHost(req, accessAllowlist) {
		return false
	}

	if !reqHasOriginMetadata(req) {
		if isCrossSiteBrowserRequest(req) {
			return false
		}
		return true
	}

	return isRequestFromTrustedOrigin(req) || isRequestFromAllowlistedOrigin(req, accessAllowlist)
}

// isTrustedCORSOrigin determines if the provided CORS origin is trusted based on server security settings and allowlist rules.
// Only valid for header-token auth: with cookie-credential auth use isTrustedCORSOriginWithCookies instead.
func isTrustedCORSOrigin(rawOrigin string, hasServerAuth bool, accessAllowlist []string) bool {
	if hasServerAuth || security.IsLax() {
		return true
	}

	parsed, ok := parseTrustedOrigin(rawOrigin)
	if !ok {
		return false
	}

	if isAllowlistedOrigin(parsed, accessAllowlist) {
		return true
	}
	if security.IsHardened() {
		return isTrustedHardenedOriginURL(parsed)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	return addr.IsLoopback() || addr.IsPrivate()
}

// isTrustedCORSOriginWithCookies is the CORS policy while cookie-credential auth
// (OIDC login) is active. Because CORS runs with AllowCredentials, reflecting an
// arbitrary origin would let any website ride the session cookie; only the
// canonical external origin, local development origins and explicit allowlist
// entries are accepted.
func isTrustedCORSOriginWithCookies(rawOrigin string, externalURL string, accessAllowlist []string) bool {
	parsed, ok := parseTrustedOrigin(rawOrigin)
	if !ok || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}

	if external, ok := parseTrustedOrigin(externalURL); ok &&
		strings.EqualFold(external.Scheme, parsed.Scheme) &&
		strings.EqualFold(external.Hostname(), parsed.Hostname()) &&
		getEffectivePort(external.Scheme, external.Port()) == getEffectivePort(parsed.Scheme, parsed.Port()) {
		return true
	}

	if isAllowlistedOrigin(parsed, accessAllowlist) {
		return true
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func (h *Handler) trustedLocalRequestMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if h == nil || h.App == nil || h.App.Config == nil {
			return next(c)
		}

		req := c.Request()
		if req == nil || req.URL == nil || !isPathNeedingTrustedLocalBoundary(req.URL.Path) {
			return next(c)
		}

		if isRequestPermitted(req, h.hasServerAuth(), h.App.Config.Server.AccessAllowlist) {
			return next(c)
		}

		return h.RespondWithStatusError(c, http.StatusForbidden, errRequestBoundaryDenied)
	}
}

var errRequestBoundaryDenied = errors.New("this server does not accept requests from this origin")

// guardPrivilegedSettingsMutation denies changes to settings that decide which
// executable gets spawned — player and torrent-client binaries and their argument
// strings. Changing those is choosing what code runs on the host, so it needs the
// exec capability even though it arrives as an ordinary settings save.
func (h *Handler) guardPrivilegedSettingsMutation(c echo.Context, prev *models.Settings, nextMedia *models.MediaPlayerSettings, nextTorrent *models.TorrentSettings) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if !privilegedSettingsChanged(prev, nextMedia, nextTorrent) {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

// guardPrivilegedExtensionManagement denies installing, updating or reloading
// third-party extensions. Extension code runs in-process with whatever the manifest
// declares, so installation is equivalent to code execution.
func (h *Handler) guardPrivilegedExtensionManagement(c echo.Context) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExtensions)
}

// guardPrivilegedMediastreamSettingsMutation denies pointing the transcoder at a
// different ffmpeg/ffprobe binary.
func (h *Handler) guardPrivilegedMediastreamSettingsMutation(c echo.Context, prev *models.MediastreamSettings, next *models.MediastreamSettings) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if !privilegedMediastreamSettingsChanged(prev, next) {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

func canConsumeMedia(req *http.Request, hasServerAuth bool, accessAllowlist []string) bool {
	return isRequestPermitted(req, hasServerAuth, accessAllowlist)
}

func (h *Handler) guardMediaConsumption(c echo.Context) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if canConsumeMedia(c.Request(), h.hasServerAuth(), h.App.Config.Server.AccessAllowlist) {
		return nil
	}

	return respondWithAbort(c, http.StatusForbidden, errRequestBoundaryDenied)
}

// guardPrivilegedMediaPlayer denies launching an external player configured with a
// custom binary or custom argument string. Spawning the stock player with
// server-built arguments stays ungated; choosing the binary or the arguments over
// HTTP is choosing what runs on the host.
func (h *Handler) guardPrivilegedMediaPlayer(c echo.Context, settings *models.Settings) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if !isPrivilegedMediaPlayer(settings) {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

// guardPrivilegedTorrentClient denies launching a torrent client binary that is not
// at its stock location.
func (h *Handler) guardPrivilegedTorrentClient(c echo.Context, settings *models.Settings) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if !isPrivilegedTorrentClient(settings) {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

// guardPrivilegedMediastream denies transcoding through a custom ffmpeg/ffprobe
// binary. Transcoding through the validated system binary is the normal path and is
// not gated — see validateMediaExecutablePath for what "validated" means.
func (h *Handler) guardPrivilegedMediastream(c echo.Context, settings *models.MediastreamSettings) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	if !isPrivilegedMediastream(settings) {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

// guardPrivilegedLocalExecution denies actions whose whole purpose is to spawn a
// process on the host: opening a directory in the desktop file manager, starting a
// torrent client, launching a player.
func (h *Handler) guardPrivilegedLocalExecution(c echo.Context) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	return h.requireCapability(c, security.CapabilityExec)
}

// guardSelfUpdate denies replacing the running binary. It is separate from
// CapabilityExec because an operator who wants remote playback control has no reason
// to also hand over the ability to swap out the server itself — and in a container
// the update is discarded on restart anyway, so the action is all risk and no value.
func (h *Handler) guardSelfUpdate(c echo.Context) error {
	if h == nil || h.App == nil || h.App.Config == nil {
		return nil
	}

	return h.requireCapability(c, security.CapabilitySelfUpdate)
}

// getContextClientId retrieves the client ID from the echo.Context by checking a header or a cookie, returning an empty string if not found.
func getContextClientId(c echo.Context) string {
	if c == nil {
		return ""
	}

	if value := c.Get("Seanime-Client-Id"); value != nil {
		if clientID, ok := value.(string); ok {
			clientID = strings.TrimSpace(clientID)
			if clientID != "" {
				return clientID
			}
		}
	}

	cookie, err := c.Cookie(clientIdCookieName)
	if err == nil {
		if clientID := strings.TrimSpace(cookie.Value); clientID != "" {
			return clientID
		}
	}

	return ""
}

func getClientPlatformFromContext(c echo.Context) string {
	if c == nil {
		return ""
	}

	if value := c.Get(clientPlatformHeader); value != nil {
		if platform, ok := value.(string); ok {
			return normalizeClientPlatform(platform)
		}
	}

	return ""
}

// getRequestClientId retrieves the client ID from the context or falls back to the claimed value after trimming whitespace.
func getRequestClientId(c echo.Context, claimed string) string {
	if contextClientID := getContextClientId(c); contextClientID != "" {
		return contextClientID
	}

	return strings.TrimSpace(claimed)
}

// isSameContextClientId checks if the claimed client ID matches the context's client ID after trimming spaces and ensuring both are non-empty.
func isSameContextClientId(c echo.Context, claimed string) bool {
	contextClientId := getContextClientId(c)
	claimed = strings.TrimSpace(claimed)
	return contextClientId != "" && claimed != "" && contextClientId == claimed
}

type accessAllowlistEntry struct {
	scheme string
	host   string
	port   string
}

func parseAccessAllowlistEntry(raw string) (*accessAllowlistEntry, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	if strings.Contains(raw, "://") {
		parsed, ok := parseTrustedOrigin(raw)
		if !ok {
			return nil, false
		}

		return &accessAllowlistEntry{
			scheme: strings.ToLower(parsed.Scheme),
			host:   strings.ToLower(parsed.Hostname()),
			port:   getEffectivePort(parsed.Scheme, parsed.Port()),
		}, true
	}

	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Hostname() == "" {
		return nil, false
	}

	return &accessAllowlistEntry{
		host: strings.ToLower(parsed.Hostname()),
		port: parsed.Port(),
	}, true
}

func parseTrustedOrigin(rawOrigin string) (*url.URL, bool) {
	rawOrigin = strings.TrimSpace(rawOrigin)
	if rawOrigin == "" {
		return nil, false
	}

	parsed, err := url.Parse(rawOrigin)
	if err != nil {
		return nil, false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, false
	}

	if parsed.Hostname() == "" {
		return nil, false
	}

	return parsed, true
}

func isRequestFromAllowlistedOrigin(req *http.Request, accessAllowlist []string) bool {
	if req == nil {
		return false
	}

	rawOrigin := strings.TrimSpace(req.Header.Get("Origin"))
	if rawOrigin == "" {
		rawOrigin = strings.TrimSpace(req.Header.Get("Referer"))
	}
	parsed, ok := parseTrustedOrigin(rawOrigin)
	if !ok {
		return false
	}

	return isAllowlistedOrigin(parsed, accessAllowlist)
}

func isRequestFromTrustedOrigin(req *http.Request) bool {
	if req == nil {
		return false
	}

	rawOrigin := strings.TrimSpace(req.Header.Get("Origin"))
	if rawOrigin == "" {
		rawOrigin = strings.TrimSpace(req.Header.Get("Referer"))
	}
	parsed, ok := parseTrustedOrigin(rawOrigin)
	if !ok {
		return false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" {
		return true
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}

	if addr.IsLoopback() {
		return true
	}

	if !addr.IsPrivate() {
		return false
	}

	return isReqSameLiteralHost(req, parsed)
}

func hasForwardedHeaders(req *http.Request) bool {
	if req == nil {
		return false
	}

	return strings.TrimSpace(req.Header.Get("Forwarded")) != "" ||
		strings.TrimSpace(req.Header.Get("X-Forwarded-For")) != "" ||
		strings.TrimSpace(req.Header.Get("X-Forwarded-Host")) != "" ||
		strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")) != "" ||
		strings.TrimSpace(req.Header.Get("X-Real-IP")) != ""
}

func isAllowlistedRequestHost(req *http.Request, accessAllowlist []string) bool {
	view := createRequestBoundaryView(req)
	if view.hostname == "" {
		return false
	}

	return isAllowlistedHost(view.hostname, view.port, "", accessAllowlist)
}

func isAllowlistedOrigin(origin *url.URL, accessAllowlist []string) bool {
	if origin == nil {
		return false
	}

	return isAllowlistedHost(strings.ToLower(origin.Hostname()), getEffectivePort(origin.Scheme, origin.Port()), strings.ToLower(origin.Scheme), accessAllowlist)
}

func isAllowlistedHost(host string, port string, scheme string, accessAllowlist []string) bool {
	if host == "" {
		return false
	}

	for _, rawEntry := range accessAllowlist {
		entry, ok := parseAccessAllowlistEntry(rawEntry)
		if !ok {
			continue
		}
		if entry.scheme != "" && scheme != "" && entry.scheme != scheme {
			continue
		}
		if !isAllowlistHostMatch(entry.host, host) {
			continue
		}
		if entry.port != "" && entry.port != port && !(port == "" && (entry.port == "80" || entry.port == "443")) {
			continue
		}

		return true
	}

	return false
}

func isAllowlistHostMatch(pattern string, host string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	host = strings.ToLower(strings.TrimSpace(host))
	if pattern == "" || host == "" {
		return false
	}
	if pattern == host {
		return true
	}
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}

	suffix := strings.TrimPrefix(pattern, "*.")
	return host != suffix && strings.HasSuffix(host, "."+suffix)
}

func getEffectivePort(scheme string, port string) string {
	if port != "" {
		return port
	}

	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

func isReqSameLiteralHost(req *http.Request, origin *url.URL) bool {
	if req == nil || origin == nil {
		return false
	}

	view := createRequestBoundaryView(req)
	if view.hostname == "" {
		return false
	}

	if !strings.EqualFold(origin.Hostname(), view.hostname) {
		return false
	}

	return getEffectivePort(origin.Scheme, origin.Port()) == getEffectivePort(view.scheme, view.port)
}

func privilegedSettingsChanged(prev *models.Settings, nextMedia *models.MediaPlayerSettings, nextTorrent *models.TorrentSettings) bool {
	if nextMedia != nil {
		prevMedia := prev.GetMediaPlayer()
		if prevMedia.Default != nextMedia.Default ||
			prevMedia.VlcPath != nextMedia.VlcPath ||
			prevMedia.MpcPath != nextMedia.MpcPath ||
			prevMedia.MpvPath != nextMedia.MpvPath ||
			prevMedia.MpvArgs != nextMedia.MpvArgs ||
			prevMedia.IinaPath != nextMedia.IinaPath ||
			prevMedia.IinaArgs != nextMedia.IinaArgs ||
			translateEndpointChanged(prevMedia, nextMedia) {
			return true
		}
	}

	if nextTorrent != nil {
		prevTorrent := prev.GetTorrent()
		if prevTorrent.Default != nextTorrent.Default ||
			prevTorrent.QBittorrentPath != nextTorrent.QBittorrentPath ||
			prevTorrent.TransmissionPath != nextTorrent.TransmissionPath {
			return true
		}
	}

	return false
}

func translateEndpointChanged(prevMedia *models.MediaPlayerSettings, nextMedia *models.MediaPlayerSettings) bool {
	if prevMedia == nil || nextMedia == nil {
		return false
	}

	prevCompatible := strings.EqualFold(prevMedia.VcTranslateProvider, "openai-compatible")
	nextCompatible := strings.EqualFold(nextMedia.VcTranslateProvider, "openai-compatible")
	if prevCompatible != nextCompatible {
		return true
	}
	if prevCompatible || nextCompatible {
		return prevMedia.VcTranslate != nextMedia.VcTranslate ||
			prevMedia.VcTranslateBaseUrl != nextMedia.VcTranslateBaseUrl
	}

	return false
}

// privilegedMediastreamSettingsChanged checks if privileged mediastream settings differ between the previous and the next configuration.
func privilegedMediastreamSettingsChanged(prev *models.MediastreamSettings, next *models.MediastreamSettings) bool {
	if next == nil {
		return false
	}

	if prev == nil {
		return isPrivilegedMediastream(next)
	}

	return prev.FfmpegPath != next.FfmpegPath || prev.FfprobePath != next.FfprobePath
}

func isPrivilegedMediaPlayer(settings *models.Settings) bool {
	media := settings.GetMediaPlayer()

	switch media.Default {
	case "vlc":
		return hasCustomExecutablePath(media.VlcPath, defaultVLCPaths()...)
	case "mpc-hc":
		return hasCustomExecutablePath(media.MpcPath, defaultMpcHcPaths()...)
	case "mpv":
		return strings.TrimSpace(media.MpvArgs) != "" || hasCustomExecutablePath(media.MpvPath, "mpv")
	case "iina":
		return strings.TrimSpace(media.IinaArgs) != "" || hasCustomExecutablePath(media.IinaPath, "iina-cli")
	default:
		return false
	}
}

func isPrivilegedTorrentClient(settings *models.Settings) bool {
	torrent := settings.GetTorrent()

	switch torrent.Default {
	case "qbittorrent":
		return hasCustomExecutablePath(torrent.QBittorrentPath, defaultQBittorrentPaths()...)
	case "transmission":
		return hasCustomExecutablePath(torrent.TransmissionPath, defaultTransmissionPaths()...)
	default:
		return false
	}
}

func isPrivilegedMediastream(settings *models.MediastreamSettings) bool {
	if settings == nil {
		return false
	}

	return hasCustomExecutablePath(settings.FfmpegPath, defaultFFmpegPaths()...) || hasCustomExecutablePath(settings.FfprobePath, defaultFFprobePaths()...)
}

// hasCustomExecutablePath checks if the given path differs from the provided default executable paths. Returns true if a custom path is detected, false otherwise.
func hasCustomExecutablePath(path string, defaults ...string) bool {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false
	}

	for _, defaultPath := range defaults {
		if sameExecutablePath(trimmed, defaultPath) {
			return false
		}
	}

	return true
}

func sameExecutablePath(left string, right string) bool {
	leftPath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(left)))
	rightPath := filepath.Clean(filepath.FromSlash(strings.TrimSpace(right)))

	if runtime.GOOS == "windows" {
		return strings.EqualFold(leftPath, rightPath)
	}

	return leftPath == rightPath
}

func defaultVLCPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"C:\\Program Files\\VideoLAN\\VLC\\vlc.exe"}
	case "linux":
		return []string{"/usr/bin/vlc"}
	case "darwin":
		return []string{"/Applications/VLC.app/Contents/MacOS/VLC"}
	default:
		return []string{"vlc"}
	}
}

func defaultMpcHcPaths() []string {
	return []string{"C:\\Program Files\\MPC-HC\\mpc-hc64.exe"}
}

func defaultQBittorrentPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"C:/Program Files/qBittorrent/qbittorrent.exe"}
	case "linux":
		return []string{"/usr/bin/qbittorrent"}
	case "darwin":
		return []string{"/Applications/qbittorrent.app/Contents/MacOS/qbittorrent"}
	default:
		return []string{"qbittorrent"}
	}
}

func defaultTransmissionPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"C:/Program Files/Transmission/transmission-qt.exe"}
	case "linux":
		return []string{"/usr/bin/transmission-qt", "/usr/bin/transmission-gtk"}
	case "darwin":
		return []string{
			"/Applications/Transmission.app/Contents/MacOS/transmission-qt",
			"/Applications/Transmission.app/Contents/MacOS/Transmission",
		}
	default:
		return []string{"transmission-qt"}
	}
}

func defaultFFmpegPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"ffmpeg.exe", "ffmpeg"}
	case "linux":
		return []string{"/usr/bin/ffmpeg", "/usr/local/bin/ffmpeg", "ffmpeg"}
	case "darwin":
		return []string{"/opt/homebrew/bin/ffmpeg", "/usr/local/bin/ffmpeg", "ffmpeg"}
	default:
		return []string{"ffmpeg"}
	}
}

func defaultFFprobePaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"ffprobe.exe", "ffprobe"}
	case "linux":
		return []string{"/usr/bin/ffprobe", "/usr/local/bin/ffprobe", "ffprobe"}
	case "darwin":
		return []string{"/opt/homebrew/bin/ffprobe", "/usr/local/bin/ffprobe", "ffprobe"}
	default:
		return []string{"ffprobe"}
	}
}
