"""LibraryModel — flattens the library collection into rows for the grid view.

A ``QAbstractListModel`` with named roles, exactly as it would be written in
C++ Qt. Each row is one anime entry, tagged with the watch-status of the list
it came from so QML can section the grid.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt


def _title_of(media: dict) -> str:
    title = media.get("title") or {}
    return (
        title.get("userPreferred")
        or title.get("english")
        or title.get("romaji")
        or title.get("native")
        or f"#{media.get('id', '?')}"
    )


def _poster_of(media: dict) -> str:
    cover = media.get("coverImage") or {}
    return cover.get("large") or cover.get("extraLarge") or cover.get("medium") or ""


class LibraryModel(QAbstractListModel):
    MediaIdRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    PosterRole = Qt.ItemDataRole.UserRole + 3
    StatusRole = Qt.ItemDataRole.UserRole + 4
    ProgressRole = Qt.ItemDataRole.UserRole + 5
    EpisodeCountRole = Qt.ItemDataRole.UserRole + 6
    ScoreRole = Qt.ItemDataRole.UserRole + 7
    IsAdultRole = Qt.ItemDataRole.UserRole + 8

    _ROLES = {
        MediaIdRole: b"mediaId",
        TitleRole: b"title",
        PosterRole: b"posterUrl",
        StatusRole: b"status",
        ProgressRole: b"progress",
        EpisodeCountRole: b"episodeCount",
        ScoreRole: b"score",
        IsAdultRole: b"isAdult",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, data) -> None:
        """Rebuild the model from the ``library/collection`` payload."""
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
                        "episodeCount": media.get("episodes") or 0,
                        "score": media.get("meanScore") or media.get("averageScore") or 0,
                        "isAdult": bool(media.get("isAdult")),
                    }
                )

        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

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
