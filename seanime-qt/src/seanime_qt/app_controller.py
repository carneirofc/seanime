"""AppController — orchestration object exposed to QML as ``app``.

Owns the ApiClient and the two list models, wires the client's signals into
model updates and observable state, and exposes slots the QML calls. Every
member here has a direct C++ Qt analog (Q_PROPERTY / Q_INVOKABLE / signals).
"""

from __future__ import annotations

from PySide6.QtCore import Property, QObject, Signal, Slot

from .api_client import ApiClient
from .episode_model import EpisodeModel
from .library_model import LibraryModel


class AppController(QObject):
    connectionStatusChanged = Signal()
    errorMessageChanged = Signal()
    detailChanged = Signal()
    animeOpened = Signal()  # QML pushes the detail page on this

    def __init__(self, parent: QObject | None = None) -> None:
        super().__init__(parent)

        self._client = ApiClient(self)
        self._library_model = LibraryModel(self)
        self._episode_model = EpisodeModel(self)

        self._connection_status = "disconnected"
        self._error_message = ""
        self._detail_title = ""
        self._detail_synopsis = ""
        self._detail_poster = ""
        self._detail_banner = ""

        self._client.statusReceived.connect(self._on_status)
        self._client.libraryReceived.connect(self._on_library)
        self._client.animeEntryReceived.connect(self._on_anime_entry)
        self._client.errorOccurred.connect(self._on_error)

    # ---- properties exposed to QML --------------------------------------

    def _get_library_model(self) -> LibraryModel:
        return self._library_model

    def _get_episode_model(self) -> EpisodeModel:
        return self._episode_model

    libraryModel = Property(QObject, _get_library_model, constant=True)
    episodeModel = Property(QObject, _get_episode_model, constant=True)

    def _get_connection_status(self) -> str:
        return self._connection_status

    connectionStatus = Property(
        str, _get_connection_status, notify=connectionStatusChanged
    )

    def _get_error_message(self) -> str:
        return self._error_message

    errorMessage = Property(str, _get_error_message, notify=errorMessageChanged)

    def _get_detail_title(self) -> str:
        return self._detail_title

    def _get_detail_synopsis(self) -> str:
        return self._detail_synopsis

    def _get_detail_poster(self) -> str:
        return self._detail_poster

    def _get_detail_banner(self) -> str:
        return self._detail_banner

    detailTitle = Property(str, _get_detail_title, notify=detailChanged)
    detailSynopsis = Property(str, _get_detail_synopsis, notify=detailChanged)
    detailPoster = Property(str, _get_detail_poster, notify=detailChanged)
    detailBanner = Property(str, _get_detail_banner, notify=detailChanged)

    # ---- slots invoked from QML -----------------------------------------

    @Slot(str, str, str)
    def connectToServer(self, host: str, port: str, token: str) -> None:
        host = (host or "127.0.0.1").strip()
        port = (port or "43211").strip()
        scheme = "http://" if "://" not in host else ""
        self._client.set_base_url(f"{scheme}{host}:{port}")
        self._client.set_token(token.strip())
        self._set_error("")
        self._set_connection_status("connecting")
        self._client.fetch_status()

    @Slot()
    def refresh(self) -> None:
        self._client.fetch_library()

    @Slot(int)
    def openAnime(self, media_id: int) -> None:
        self._episode_model.clear()
        self._set_detail("", "", "", "")
        self._client.fetch_anime_entry(media_id)
        self.animeOpened.emit()

    # ---- client signal handlers -----------------------------------------

    def _on_status(self, _data) -> None:
        self._set_connection_status("connected")
        self._set_error("")
        self._client.fetch_library()

    def _on_library(self, data) -> None:
        self._library_model.load(data)

    def _on_anime_entry(self, data) -> None:
        data = data or {}
        media = data.get("media") or {}
        title = media.get("title") or {}
        cover = media.get("coverImage") or {}
        self._set_detail(
            title.get("userPreferred")
            or title.get("english")
            or title.get("romaji")
            or "",
            _strip_html(media.get("description") or ""),
            cover.get("large") or cover.get("extraLarge") or "",
            media.get("bannerImage") or "",
        )
        self._episode_model.load(data.get("episodes"))

    def _on_error(self, message: str) -> None:
        if self._connection_status == "connecting":
            self._set_connection_status("disconnected")
        self._set_error(message)

    # ---- setters that emit change notifications -------------------------

    def _set_connection_status(self, value: str) -> None:
        if value != self._connection_status:
            self._connection_status = value
            self.connectionStatusChanged.emit()

    def _set_error(self, value: str) -> None:
        if value != self._error_message:
            self._error_message = value
            self.errorMessageChanged.emit()

    def _set_detail(self, title: str, synopsis: str, poster: str, banner: str) -> None:
        self._detail_title = title
        self._detail_synopsis = synopsis
        self._detail_poster = poster
        self._detail_banner = banner
        self.detailChanged.emit()


def _strip_html(text: str) -> str:
    """AniList descriptions contain light HTML; strip tags for plain display."""
    out = []
    depth = 0
    for ch in text:
        if ch == "<":
            depth += 1
        elif ch == ">":
            if depth > 0:
                depth -= 1
        elif depth == 0:
            out.append(ch)
    return "".join(out).replace("&mdash;", "—").replace("&amp;", "&").strip()
