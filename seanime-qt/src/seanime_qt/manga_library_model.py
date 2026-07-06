"""MangaLibraryModel — flattens the manga collection into rows for the grid view.

A ``QAbstractListModel`` mirroring LibraryModel: the ``manga/collection`` payload
has the same ``lists → entries → media + listData`` shape as the anime library,
so the same ``AnimeCard`` delegate renders it. The count badge carries the manga's
chapter count (exposed under the ``episodeCount`` role so the delegate is reused
verbatim).
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt

from .library_model import _poster_of, _title_of


class MangaLibraryModel(QAbstractListModel):
    MediaIdRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    PosterRole = Qt.ItemDataRole.UserRole + 3
    StatusRole = Qt.ItemDataRole.UserRole + 4
    ProgressRole = Qt.ItemDataRole.UserRole + 5
    EpisodeCountRole = Qt.ItemDataRole.UserRole + 6
    # Same role number as LibraryModel/SearchModel so AdultFilterProxy and the
    # AnimeCard blur work over manga rows unchanged.
    IsAdultRole = Qt.ItemDataRole.UserRole + 8

    _ROLES = {
        MediaIdRole: b"mediaId",
        TitleRole: b"title",
        PosterRole: b"posterUrl",
        StatusRole: b"status",
        ProgressRole: b"progress",
        EpisodeCountRole: b"episodeCount",  # chapter count for manga
        IsAdultRole: b"isAdult",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, data) -> None:
        """Rebuild the model from the ``manga/collection`` payload."""
        rows: list[dict] = []
        lists = (data or {}).get("lists") or []
        for lst in lists:
            status = lst.get("status") or lst.get("type") or "UNKNOWN"
            for entry in lst.get("entries") or []:
                media = entry.get("media") or {}
                list_data = entry.get("listData") or {}
                rows.append(
                    {
                        "mediaId": entry.get("mediaId") or media.get("id") or 0,
                        "title": _title_of(media),
                        "posterUrl": _poster_of(media),
                        "status": status,
                        "progress": list_data.get("progress") or 0,
                        "episodeCount": media.get("chapters") or 0,
                        "isAdult": bool(media.get("isAdult")),
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
