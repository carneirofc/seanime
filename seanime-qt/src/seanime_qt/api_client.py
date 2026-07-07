"""ApiClient — thin, Qt-only HTTP client for the Seanime REST API.

Deliberately uses ONLY Qt libraries so the C++ port is mechanical:
  - QNetworkAccessManager / QNetworkRequest / QNetworkReply for transport
  - QJsonDocument for parsing (never the Python ``json`` module)

Every public fetch issues an async request and, on completion, emits a typed
signal carrying the already-unwrapped ``data`` payload (the API wraps every
response in ``{"data": ...}``). Failures funnel through ``errorOccurred``.
"""

from __future__ import annotations

from PySide6.QtCore import QObject, QUrl, QUrlQuery, Signal, Slot
from PySide6.QtNetwork import (
    QNetworkAccessManager,
    QNetworkReply,
    QNetworkRequest,
)
from PySide6.QtCore import QJsonDocument


# Header used by password-protected Seanime servers.
_TOKEN_HEADER = b"X-Seanime-Token"

# Origin the Seanime server recognises as a trusted local desktop app (the same
# value its official desktop wrapper sends). On a passwordless server, privileged
# actions — notably extension install/uninstall/enable/disable — are denied
# unless the request comes from a trusted local origin; sending this marks the
# Qt client as one. The server special-cases ``app://-`` for exactly this.
_DESKTOP_ORIGIN = b"app://-"

# AniList's OAuth token endpoint (authorization-code exchange).
_ANILIST_TOKEN_URL = "https://anilist.co/api/v2/oauth/token"


class ApiClient(QObject):
    """Issues requests against a configurable Seanime server base URL."""

    # Each signal carries the unwrapped ``data`` object from the response.
    statusReceived = Signal("QVariant")
    libraryReceived = Signal("QVariant")
    animeEntryReceived = Signal("QVariant")
    mediaDetailsReceived = Signal("QVariant")
    searchReceived = Signal("QVariant")
    searchMoreReceived = Signal("QVariant")  # a paginated "load more" page (append)
    discoverReceived = Signal("QVariant")
    seasonReceived = Signal("QVariant")
    prevSeasonReceived = Signal("QVariant")
    upcomingReceived = Signal("QVariant")
    moviesReceived = Signal("QVariant")
    missedSequelsReceived = Signal("QVariant")
    listEntryUpdated = Signal("QVariant")  # AniList list edit succeeded
    progressUpdated = Signal("QVariant")   # episode progress update succeeded
    torrentSearchReceived = Signal("QVariant")     # torrent search results
    torrentDownloadSucceeded = Signal("QVariant")  # torrents handed to the client
    # Extensions / providers.
    allExtensionsReceived = Signal("QVariant")     # installed + disabled + invalid
    marketplaceReceived = Signal("QVariant")       # marketplace [] Extension
    extensionFetched = Signal("QVariant")          # preview data for a manifest URI
    extensionInstalled = Signal("QVariant")        # install/update succeeded
    extensionUninstalled = Signal("QVariant")      # uninstall succeeded
    extensionDisabledSet = Signal("QVariant")      # enable/disable toggled
    extensionsReloaded = Signal("QVariant")        # external extensions reloaded
    # Manga.
    mangaCollectionReceived = Signal("QVariant")
    mangaEntryReceived = Signal("QVariant")
    mangaProvidersReceived = Signal("QVariant")
    mangaChaptersReceived = Signal("QVariant")
    mangaPagesReceived = Signal("QVariant")
    mangaProgressUpdated = Signal("QVariant")  # manga chapter progress update succeeded
    settingsSaved = Signal("QVariant")     # server settings PATCH succeeded (fresh Status)
    loginSucceeded = Signal("QVariant")
    loginFailed = Signal(str)  # login-specific failure, kept off errorOccurred
    anilistTokenObtained = Signal(str)  # access token from the code exchange
    errorOccurred = Signal(str)

    def __init__(self, parent: QObject | None = None) -> None:
        super().__init__(parent)
        self._manager = QNetworkAccessManager(self)
        self._base_url = "http://127.0.0.1:43211"
        self._token = ""

    # ---- configuration ---------------------------------------------------

    def set_base_url(self, base_url: str) -> None:
        self._base_url = base_url.rstrip("/")

    def set_token(self, token: str) -> None:
        self._token = token or ""

    # ---- public fetches --------------------------------------------------

    @Slot()
    def fetch_status(self) -> None:
        self._get("/api/v1/status", self.statusReceived)

    @Slot()
    def fetch_library(self) -> None:
        self._get("/api/v1/library/collection", self.libraryReceived)

    @Slot(int)
    def fetch_anime_entry(self, media_id: int) -> None:
        self._get(f"/api/v1/library/anime-entry/{media_id}", self.animeEntryReceived)

    @Slot(int)
    def fetch_media_details(self, media_id: int) -> None:
        """Fetch rich AniList details (relations, recommendations, characters)."""
        self._get(
            f"/api/v1/anilist/media-details/{media_id}", self.mediaDetailsReceived
        )

    @Slot()
    def fetch_missed_sequels(self) -> None:
        """Sequels to anime in the user's library they haven't added yet."""
        self._get("/api/v1/anilist/list-missed-sequels", self.missedSequelsReceived)

    # ---- manga -----------------------------------------------------------

    @Slot()
    def fetch_manga_collection(self) -> None:
        self._get("/api/v1/manga/collection", self.mangaCollectionReceived)

    @Slot(int)
    def fetch_manga_entry(self, media_id: int) -> None:
        self._get(f"/api/v1/manga/entry/{media_id}", self.mangaEntryReceived)

    @Slot()
    def list_manga_providers(self) -> None:
        """Installed manga-provider extensions (for the chapter-source dropdown)."""
        self._get(
            "/api/v1/extensions/list/manga-provider", self.mangaProvidersReceived
        )

    def fetch_manga_chapters(self, media_id: int, provider: str) -> None:
        """POST for the chapter list of a manga from a given provider."""
        self._post_json(
            "/api/v1/manga/chapters",
            {"mediaId": media_id, "provider": provider},
            self.mangaChaptersReceived,
        )

    def fetch_manga_pages(
        self, media_id: int, provider: str, chapter_id: str
    ) -> None:
        """POST for the pages of a manga chapter from a given provider."""
        self._post_json(
            "/api/v1/manga/pages",
            {
                "mediaId": media_id,
                "provider": provider,
                "chapterId": chapter_id,
                "doublePage": False,
            },
            self.mangaPagesReceived,
        )

    def update_manga_progress(
        self, media_id: int, chapter_number: int, total_chapters: int
    ) -> None:
        """Mark read progress up to ``chapter_number`` for the given manga."""
        self._post_json(
            "/api/v1/manga/update-progress",
            {
                "mediaId": media_id,
                "chapterNumber": chapter_number,
                "totalChapters": total_chapters,
            },
            self.mangaProgressUpdated,
        )

    def list_anime(self, body: dict, on_success: Signal) -> None:
        """POST the advanced-search / discover query and route the result to a signal."""
        request = self._build_request("/api/v1/anilist/list-anime")
        request.setHeader(
            QNetworkRequest.KnownHeaders.ContentTypeHeader, "application/json"
        )
        payload = QJsonDocument.fromVariant(body).toJson(QJsonDocument.JsonFormat.Compact)
        reply = self._manager.post(request, payload)
        reply.finished.connect(lambda: self._handle_reply(reply, on_success))

    def edit_list_entry(
        self, media_id: int, status: str, score: int, progress: int
    ) -> None:
        """Update the user's AniList list entry (status / score / progress)."""
        body = {
            "mediaId": media_id,
            "status": status,
            "score": score,
            "progress": progress,
            "type": "anime",
        }
        self._post_json(
            "/api/v1/anilist/list-entry", body, self.listEntryUpdated
        )

    def update_progress(
        self, media_id: int, episode_number: int, total_episodes: int
    ) -> None:
        """Mark progress up to ``episode_number`` for the given media."""
        body = {
            "mediaId": media_id,
            "episodeNumber": episode_number,
            "totalEpisodes": total_episodes,
        }
        self._post_json(
            "/api/v1/library/anime-entry/update-progress",
            body,
            self.progressUpdated,
        )

    def search_torrent(self, body: dict) -> None:
        """POST a torrent search; the result routes to ``torrentSearchReceived``.

        ``body`` carries ``{type, provider, query, episodeNumber, batch,
        resolution, media}``. An empty ``provider`` makes the server use the
        user's default torrent provider.
        """
        self._post_json("/api/v1/torrent/search", body, self.torrentSearchReceived)

    def torrent_client_download(self, body: dict) -> None:
        """POST selected torrents to the configured torrent client.

        ``body`` carries ``{torrents, destination, smartSelect, media}``.
        """
        self._post_json(
            "/api/v1/torrent-client/download", body, self.torrentDownloadSucceeded
        )

    # ---- extensions / providers -----------------------------------------

    @Slot()
    def fetch_all_extensions(self) -> None:
        """POST for all installed extensions (enabled, disabled and invalid)."""
        self._post_json(
            "/api/v1/extensions/all", {"withUpdates": False}, self.allExtensionsReceived
        )

    @Slot()
    def fetch_marketplace(self) -> None:
        """GET the marketplace extension catalogue (the default repository)."""
        self._get("/api/v1/extensions/marketplace", self.marketplaceReceived)

    def fetch_external_extension(self, manifest_uri: str) -> None:
        """POST a manifest URI to preview the extension it describes (no install)."""
        self._post_json(
            "/api/v1/extensions/external/fetch",
            {"manifestUri": manifest_uri},
            self.extensionFetched,
        )

    def install_external_extension(self, manifest_uri: str) -> None:
        """POST a manifest URI to install (or update) that extension."""
        self._post_json(
            "/api/v1/extensions/external/install",
            {"manifestUri": manifest_uri},
            self.extensionInstalled,
        )

    def uninstall_external_extension(self, extension_id: str) -> None:
        """POST an extension ID to uninstall it."""
        self._post_json(
            "/api/v1/extensions/external/uninstall",
            {"id": extension_id},
            self.extensionUninstalled,
        )

    def set_extension_disabled(self, extension_id: str, disabled: bool) -> None:
        """POST to enable or disable an installed extension."""
        self._post_json(
            "/api/v1/extensions/external/disabled",
            {"id": extension_id, "disabled": bool(disabled)},
            self.extensionDisabledSet,
        )

    def reload_external_extensions(self) -> None:
        """POST to reload all external extensions from disk."""
        self._post_json(
            "/api/v1/extensions/external/reload", {}, self.extensionsReloaded
        )

    def save_settings(self, body: dict) -> None:
        """PATCH the full server settings (8 groups) and emit ``settingsSaved``.

        The server binds each group wholesale, so callers must send every field of
        every group they intend to keep — the ``AppController``/QML side overlays
        edits onto the current settings before calling this.
        """
        self._send_json(b"PATCH", "/api/v1/settings", body, self.settingsSaved)

    def _post_json(self, path: str, body: dict, on_success: Signal) -> None:
        self._send_json(b"POST", path, body, on_success)

    def _send_json(
        self, verb: bytes, path: str, body: dict, on_success: Signal
    ) -> None:
        """Issue a JSON-bodied request with an arbitrary HTTP verb.

        ``POST`` goes through ``QNetworkAccessManager.post``; anything else (e.g.
        ``PATCH``, which has no dedicated method) uses ``sendCustomRequest``.
        """
        request = self._build_request(path)
        request.setHeader(
            QNetworkRequest.KnownHeaders.ContentTypeHeader, "application/json"
        )
        payload = QJsonDocument.fromVariant(body).toJson(QJsonDocument.JsonFormat.Compact)
        if verb == b"POST":
            reply = self._manager.post(request, payload)
        else:
            reply = self._manager.sendCustomRequest(request, verb, payload)
        reply.finished.connect(lambda: self._handle_reply(reply, on_success))

    @Slot(str)
    def login(self, anilist_token: str) -> None:
        """POST the AniList access token to the server to authenticate.

        Distinct from the ``X-Seanime-Token`` header (a server password): this
        is the AniList OAuth token the server stores and uses on the user's
        behalf. The response is a Status payload, forwarded via loginSucceeded.
        """
        request = self._build_request("/api/v1/auth/login")
        request.setHeader(
            QNetworkRequest.KnownHeaders.ContentTypeHeader, "application/json"
        )
        body = QJsonDocument.fromVariant({"token": anilist_token}).toJson(
            QJsonDocument.JsonFormat.Compact
        )
        reply = self._manager.post(request, body)
        reply.finished.connect(
            lambda: self._handle_reply(reply, self.loginSucceeded, self.loginFailed)
        )

    def exchange_anilist_code(
        self, code: str, client_id: str, client_secret: str, redirect_uri: str
    ) -> None:
        """Exchange an AniList authorization code for an access token.

        This talks directly to AniList (not the Seanime server), form-encoded per
        the OAuth spec, and emits anilistTokenObtained with the access token.
        """
        form = QUrlQuery()
        form.addQueryItem("grant_type", "authorization_code")
        form.addQueryItem("client_id", client_id)
        form.addQueryItem("client_secret", client_secret)
        form.addQueryItem("redirect_uri", redirect_uri)
        form.addQueryItem("code", code)
        body = form.toString(QUrl.ComponentFormattingOption.FullyEncoded).encode("utf-8")

        request = QNetworkRequest(QUrl(_ANILIST_TOKEN_URL))
        request.setHeader(
            QNetworkRequest.KnownHeaders.ContentTypeHeader,
            "application/x-www-form-urlencoded",
        )
        request.setRawHeader(b"Accept", b"application/json")
        reply = self._manager.post(request, body)
        reply.finished.connect(lambda: self._handle_anilist_token(reply))

    # ---- internals -------------------------------------------------------

    def _handle_anilist_token(self, reply: QNetworkReply) -> None:
        try:
            raw = bytes(reply.readAll().data())
            payload = QJsonDocument.fromJson(raw).toVariant()
            if isinstance(payload, dict) and payload.get("access_token"):
                self.anilistTokenObtained.emit(str(payload["access_token"]))
                return
            # AniList error bodies carry hint/message/error fields.
            msg = ""
            if isinstance(payload, dict):
                msg = (
                    payload.get("hint")
                    or payload.get("message")
                    or payload.get("error")
                    or ""
                )
            self.errorOccurred.emit(
                str(msg) or reply.errorString() or "AniList token exchange failed"
            )
        finally:
            reply.deleteLater()

    def _build_request(self, path: str) -> QNetworkRequest:
        request = QNetworkRequest(QUrl(self._base_url + path))
        request.setRawHeader(b"Accept", b"application/json")
        # Identify as a trusted local desktop app so passwordless servers permit
        # privileged actions (e.g. extension management). See _DESKTOP_ORIGIN.
        request.setRawHeader(b"Origin", _DESKTOP_ORIGIN)
        if self._token:
            request.setRawHeader(_TOKEN_HEADER, self._token.encode("utf-8"))
        return request

    def _get(self, path: str, on_success: Signal) -> None:
        reply = self._manager.get(self._build_request(path))
        # Keep the reply referenced until finished; the lambda captures it.
        reply.finished.connect(lambda: self._handle_reply(reply, on_success))

    def _handle_reply(
        self, reply: QNetworkReply, on_success: Signal, on_error: Signal | None = None
    ) -> None:
        error_signal = on_error or self.errorOccurred
        try:
            raw = bytes(reply.readAll().data())
            # Even on an HTTP error status the server sends a JSON body like
            # ``{"error": "..."}``; prefer that message over the generic
            # network error string.
            payload = QJsonDocument.fromJson(raw).toVariant()
            body_error = (
                str(payload.get("error"))
                if isinstance(payload, dict) and payload.get("error")
                else ""
            )

            if reply.error() != QNetworkReply.NetworkError.NoError:
                error_signal.emit(body_error or reply.errorString())
                return

            if body_error:
                error_signal.emit(body_error)
                return

            if not isinstance(payload, dict):
                error_signal.emit("Invalid JSON in server response")
                return

            on_success.emit(payload.get("data"))
        finally:
            reply.deleteLater()
