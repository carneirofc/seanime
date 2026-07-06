"""EpisodeModel — episodes for the detail view.

A ``QAbstractListModel`` with named roles. Populated from the ``episodes`` array
of a ``library/anime-entry/:id`` payload.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt


class EpisodeModel(QAbstractListModel):
    NumberRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    ThumbnailRole = Qt.ItemDataRole.UserRole + 3
    DownloadedRole = Qt.ItemDataRole.UserRole + 4
    SummaryRole = Qt.ItemDataRole.UserRole + 5
    ProgressNumberRole = Qt.ItemDataRole.UserRole + 6
    WatchedRole = Qt.ItemDataRole.UserRole + 7

    _ROLES = {
        NumberRole: b"number",
        TitleRole: b"title",
        ThumbnailRole: b"thumbnailUrl",
        DownloadedRole: b"isDownloaded",
        SummaryRole: b"summary",
        ProgressNumberRole: b"progressNumber",
        WatchedRole: b"isWatched",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    def load(self, episodes, watched_through: int = 0) -> None:
        """Populate from the ``episodes`` array; ``watched_through`` is the
        AniList list progress (the highest episode counted as watched)."""
        rows: list[dict] = []
        for ep in episodes or []:
            meta = ep.get("episodeMetadata") or {}
            display = ep.get("displayTitle") or f"Episode {ep.get('episodeNumber', '?')}"
            episode_title = ep.get("episodeTitle") or ""
            progress_number = ep.get("progressNumber") or ep.get("episodeNumber") or 0
            rows.append(
                {
                    "number": ep.get("episodeNumber") or 0,
                    "title": f"{display} — {episode_title}" if episode_title else display,
                    "thumbnailUrl": meta.get("image") or "",
                    "isDownloaded": bool(ep.get("isDownloaded")),
                    "summary": meta.get("summary") or meta.get("overview") or "",
                    "progressNumber": progress_number,
                    "isWatched": progress_number > 0 and progress_number <= watched_through,
                }
            )

        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    def clear(self) -> None:
        self.load([])

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
