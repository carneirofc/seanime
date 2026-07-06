"""SearchModel — holds AniList catalog search results for the grid view.

A ``QAbstractListModel`` mirroring LibraryModel's roles so the same AnimeCard
delegate renders it. Populated from the ``anilist/list-anime`` payload, whose
shape is ``{ "Page": { "media": [BaseAnime, ...] } }``.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt

from .library_model import _poster_of, _title_of


class SearchModel(QAbstractListModel):
    MediaIdRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    PosterRole = Qt.ItemDataRole.UserRole + 3
    StatusRole = Qt.ItemDataRole.UserRole + 4
    ProgressRole = Qt.ItemDataRole.UserRole + 5
    EpisodeCountRole = Qt.ItemDataRole.UserRole + 6

    _ROLES = {
        MediaIdRole: b"mediaId",
        TitleRole: b"title",
        PosterRole: b"posterUrl",
        StatusRole: b"status",
        ProgressRole: b"progress",
        EpisodeCountRole: b"episodeCount",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, data) -> None:
        """Rebuild the model from an ``anilist/list-anime`` payload."""
        page = (data or {}).get("Page") or {}
        self.load_media_list(page.get("media") or [])

    def load_media_list(self, media_list) -> None:
        """Rebuild the model from an already-extracted list of BaseAnime media."""
        self.beginResetModel()
        self._rows = [self._row_of(media) for media in (media_list or []) if media]
        self.endResetModel()

    def append_media_list(self, media_list) -> None:
        """Append more media (pagination / 'load more')."""
        new_rows = [self._row_of(media) for media in (media_list or []) if media]
        if not new_rows:
            return
        start = len(self._rows)
        self.beginInsertRows(QModelIndex(), start, start + len(new_rows) - 1)
        self._rows.extend(new_rows)
        self.endInsertRows()

    @staticmethod
    def _row_of(media: dict) -> dict:
        media = media or {}
        return {
            "mediaId": media.get("id") or 0,
            "title": _title_of(media),
            "posterUrl": _poster_of(media),
            "status": media.get("format") or "",
            "progress": 0,
            "episodeCount": media.get("episodes") or 0,
        }

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
