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

    def _post_json(self, path: str, body: dict, on_success: Signal) -> None:
        request = self._build_request(path)
        request.setHeader(
            QNetworkRequest.KnownHeaders.ContentTypeHeader, "application/json"
        )
        payload = QJsonDocument.fromVariant(body).toJson(QJsonDocument.JsonFormat.Compact)
        reply = self._manager.post(request, payload)
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
