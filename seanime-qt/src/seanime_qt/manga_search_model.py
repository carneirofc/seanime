"""MangaSearchModel — provider search results for the manual mapping dialog.

A ``QAbstractListModel`` populated from a ``/api/v1/manga/search`` payload (a
``[]HibikeManga_SearchResult``). Each result's cover image URL is rewritten to go
through the server's image proxy (``/api/v1/image-proxy``) so provider-specific
request headers (Referer, etc.) are applied server-side — a bare QML ``Image``
can't send them itself. The raw provider manga ``id`` is kept so it can be handed
back to the mapping call. URL building uses only Qt facilities (QUrl / QUrlQuery /
QJsonDocument) to keep the C++ port mechanical.
"""

from __future__ import annotations

from PySide6.QtCore import (
    QAbstractListModel,
    QByteArray,
    QJsonDocument,
    QModelIndex,
    Qt,
    QUrl,
    QUrlQuery,
    Slot,
)


def proxied_image_url(base_url: str, url: str, headers) -> str:
    """Build ``{base}/api/v1/image-proxy?url=<url>&headers=<json>``.

    Returns ``""`` for an empty ``url`` so the UI can show a placeholder.
    """
    if not url:
        return ""
    proxied = QUrl(base_url.rstrip("/") + "/api/v1/image-proxy")
    query = QUrlQuery()
    query.addQueryItem("url", url)
    if headers:
        headers_json = bytes(
            QJsonDocument.fromVariant(dict(headers)).toJson(
                QJsonDocument.JsonFormat.Compact
            )
        ).decode("utf-8")
        query.addQueryItem("headers", headers_json)
    proxied.setQuery(query)
    return proxied.toString()


class MangaSearchModel(QAbstractListModel):
    IdRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    YearRole = Qt.ItemDataRole.UserRole + 3
    ImageUrlRole = Qt.ItemDataRole.UserRole + 4
    UrlRole = Qt.ItemDataRole.UserRole + 5
    SynonymsRole = Qt.ItemDataRole.UserRole + 6

    _ROLES = {
        IdRole: b"mangaId",
        TitleRole: b"title",
        YearRole: b"year",
        ImageUrlRole: b"imageUrl",
        UrlRole: b"url",
        SynonymsRole: b"synonyms",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, results, base_url: str) -> None:
        """Rebuild from a ``[]SearchResult`` payload, proxying each cover URL."""
        results = results if isinstance(results, list) else []
        rows: list[dict] = []
        for res in results:
            res = res or {}
            manga_id = str(res.get("id") or "")
            if not manga_id:
                continue
            rows.append(
                {
                    "mangaId": manga_id,
                    "title": res.get("title") or "",
                    "year": int(res.get("year") or 0),
                    "imageUrl": proxied_image_url(
                        base_url, res.get("image") or "", res.get("imageHeaders")
                    ),
                    "url": res.get("url") or "",
                    "synonyms": ", ".join(res.get("synonyms") or []),
                }
            )
        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    def clear(self) -> None:
        self.beginResetModel()
        self._rows = []
        self.endResetModel()

    @property
    def count(self) -> int:
        return len(self._rows)

    @Slot(int, result=str)
    def mangaIdAt(self, index: int) -> str:
        """Return the provider manga ID at ``index`` (for the mapping call)."""
        if 0 <= index < len(self._rows):
            return self._rows[index]["mangaId"]
        return ""

    # ---- QAbstractListModel API -----------------------------------------

    def rowCount(self, parent=QModelIndex()) -> int:
        return 0 if parent.isValid() else len(self._rows)

    def data(self, index, role=Qt.ItemDataRole.DisplayRole):
        if not index.isValid() or not (0 <= index.row() < len(self._rows)):
            return None
        key = self._ROLES.get(role)
        if key is None:
            return None
        return self._rows[index.row()].get(key.decode())

    def roleNames(self):
        return {role: QByteArray(name) for role, name in self._ROLES.items()}
