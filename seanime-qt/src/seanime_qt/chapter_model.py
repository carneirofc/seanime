"""ChapterModel — chapters for the manga detail view.

A ``QAbstractListModel`` populated from a ``manga/chapters`` ChapterContainer
payload (``{ "chapters": [ChapterDetails, ...] }``). ``read`` is derived from the
AniList list progress: a chapter counts as read when its chapter number is at or
below the user's recorded progress.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt


def _chapter_number(chapter: dict) -> float:
    """Parse a chapter's numeric value (``"1"``, ``"1.5"``, …); 0 if unknown."""
    raw = str(chapter.get("chapter") or "").strip()
    try:
        return float(raw)
    except ValueError:
        return 0.0


class ChapterModel(QAbstractListModel):
    ChapterIdRole = Qt.ItemDataRole.UserRole + 1
    TitleRole = Qt.ItemDataRole.UserRole + 2
    NumberRole = Qt.ItemDataRole.UserRole + 3
    IndexRole = Qt.ItemDataRole.UserRole + 4
    ScanlatorRole = Qt.ItemDataRole.UserRole + 5
    LanguageRole = Qt.ItemDataRole.UserRole + 6
    ReadRole = Qt.ItemDataRole.UserRole + 7

    _ROLES = {
        ChapterIdRole: b"chapterId",
        TitleRole: b"title",
        NumberRole: b"number",
        IndexRole: b"chapterIndex",
        ScanlatorRole: b"scanlator",
        LanguageRole: b"language",
        ReadRole: b"read",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    # ---- population ------------------------------------------------------

    def load(self, data, read_through: float = 0) -> None:
        """Populate from a ChapterContainer; ``read_through`` is the AniList list
        progress (the highest chapter number counted as read)."""
        chapters = (data or {}).get("chapters") or []
        rows: list[dict] = []
        for ch in chapters:
            number = _chapter_number(ch)
            title = ch.get("title") or (
                f"Chapter {ch.get('chapter')}" if ch.get("chapter") else "Chapter"
            )
            rows.append(
                {
                    "chapterId": ch.get("id") or "",
                    "title": title,
                    "number": number,
                    "chapterIndex": ch.get("index") or 0,
                    "scanlator": ch.get("scanlator") or "",
                    "language": ch.get("language") or "",
                    "read": number > 0 and number <= read_through,
                }
            )

        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    def set_read_through(self, read_through: float) -> None:
        """Recompute the ``read`` flag for every loaded chapter against a new
        progress value, without refetching the chapter list."""
        if not self._rows:
            return
        for row in self._rows:
            number = row["number"]
            row["read"] = number > 0 and number <= read_through
        top = self.index(len(self._rows) - 1)
        self.dataChanged.emit(self.index(0), top, [self.ReadRole])

    def clear(self) -> None:
        self.load({})

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
