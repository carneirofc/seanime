"""CharacterModel — characters for the detail view.

A ``QAbstractListModel`` with named roles, populated from the ``characters``
field of an ``anilist/media-details/:id`` payload. That field has the AniList
connection shape ``{ "edges": [ { "role", "name", "node": baseCharacter } ] }``.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt


class CharacterModel(QAbstractListModel):
    NameRole = Qt.ItemDataRole.UserRole + 1
    ImageRole = Qt.ItemDataRole.UserRole + 2
    RoleRole = Qt.ItemDataRole.UserRole + 3

    _ROLES = {
        NameRole: b"name",
        ImageRole: b"imageUrl",
        RoleRole: b"role",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    def load(self, characters) -> None:
        """Rebuild from the ``characters`` connection object (or its edges)."""
        edges = characters.get("edges") if isinstance(characters, dict) else characters
        rows: list[dict] = []
        for edge in edges or []:
            edge = edge or {}
            node = edge.get("node") or {}
            name = node.get("name") or {}
            name_str = (
                name.get("full")
                or name.get("userPreferred")
                or (edge.get("name") if isinstance(edge.get("name"), str) else "")
                or ""
            )
            image = node.get("image") or {}
            rows.append(
                {
                    "name": name_str,
                    "imageUrl": image.get("large") or image.get("medium") or "",
                    "role": (edge.get("role") or "").title(),
                }
            )

        self.beginResetModel()
        self._rows = rows
        self.endResetModel()

    def clear(self) -> None:
        self.beginResetModel()
        self._rows = []
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
