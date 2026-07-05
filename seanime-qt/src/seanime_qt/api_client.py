"""ApiClient — thin, Qt-only HTTP client for the Seanime REST API.

Deliberately uses ONLY Qt libraries so the C++ port is mechanical:
  - QNetworkAccessManager / QNetworkRequest / QNetworkReply for transport
  - QJsonDocument for parsing (never the Python ``json`` module)

Every public fetch issues an async request and, on completion, emits a typed
signal carrying the already-unwrapped ``data`` payload (the API wraps every
response in ``{"data": ...}``). Failures funnel through ``errorOccurred``.
"""

from __future__ import annotations

from PySide6.QtCore import QObject, QUrl, Signal, Slot
from PySide6.QtNetwork import (
    QNetworkAccessManager,
    QNetworkReply,
    QNetworkRequest,
)
from PySide6.QtCore import QJsonDocument


# Header used by password-protected Seanime servers.
_TOKEN_HEADER = b"X-Seanime-Token"


class ApiClient(QObject):
    """Issues requests against a configurable Seanime server base URL."""

    # Each signal carries the unwrapped ``data`` object from the response.
    statusReceived = Signal("QVariant")
    libraryReceived = Signal("QVariant")
    animeEntryReceived = Signal("QVariant")
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

    # ---- internals -------------------------------------------------------

    def _get(self, path: str, on_success: Signal) -> None:
        request = QNetworkRequest(QUrl(self._base_url + path))
        request.setRawHeader(b"Accept", b"application/json")
        if self._token:
            request.setRawHeader(_TOKEN_HEADER, self._token.encode("utf-8"))

        reply = self._manager.get(request)
        # Keep the reply referenced until finished; the lambda captures it.
        reply.finished.connect(lambda: self._handle_reply(reply, on_success))

    def _handle_reply(self, reply: QNetworkReply, on_success: Signal) -> None:
        try:
            if reply.error() != QNetworkReply.NetworkError.NoError:
                self.errorOccurred.emit(reply.errorString())
                return

            raw = bytes(reply.readAll().data())
            doc = QJsonDocument.fromJson(raw)
            if doc.isNull():
                self.errorOccurred.emit("Invalid JSON in server response")
                return

            payload = doc.toVariant()
            if isinstance(payload, dict) and payload.get("error"):
                self.errorOccurred.emit(str(payload.get("error")))
                return

            data = payload.get("data") if isinstance(payload, dict) else None
            on_success.emit(data)
        finally:
            reply.deleteLater()
