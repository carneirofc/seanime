"""PageModel — pages for the manga reader.

A ``QAbstractListModel`` populated from a ``manga/pages`` PageContainer payload
(``{ "pages": [ChapterPage, ...] }``). Each page's raw image URL is rewritten to
go through the server's image proxy (``/api/v1/image-proxy``) so provider-specific
request headers (Referer, etc.) are applied server-side — a bare QML ``Image``
can't send them itself. URL building uses only Qt facilities (QUrl / QUrlQuery /
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
)


class PageModel(QAbstractListModel):
    IndexRole = Qt.ItemDataRole.UserRole + 1
    ImageUrlRole = Qt.ItemDataRole.UserRole + 2

    _ROLES = {
        IndexRole: b"pageIndex",
        ImageUrlRole: b"imageUrl",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, data, base_url: str) -> None:
        """Rebuild from a PageContainer, proxying each page URL through ``base_url``."""
        pages = (data or {}).get("pages") or []
        pages = sorted(pages, key=lambda p: p.get("index") or 0)
        rows = [
            {
                "pageIndex": page.get("index") or 0,
                "imageUrl": self._proxied_url(
                    base_url, page.get("url") or "", page.get("headers")
                ),
            }
            for page in pages
            if page.get("url")
        ]

        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    @staticmethod
    def _proxied_url(base_url: str, url: str, headers) -> str:
        """Build ``{base}/api/v1/image-proxy?url=<url>&headers=<json>``."""
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

    def clear(self) -> None:
        self.beginResetModel()
        self._rows = []
        self.endResetModel()

    @property
    def count(self) -> int:
        return len(self._rows)

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
