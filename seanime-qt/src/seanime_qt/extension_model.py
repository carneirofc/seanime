"""ExtensionModel — installed and marketplace extensions for the install UI.

A ``QAbstractListModel`` with named roles, populated either from a
``/api/v1/extensions/all`` (AllExtensions) payload — combining the enabled,
disabled and invalid extensions into one flat list — or from a
``/api/v1/extensions/marketplace`` ``[]Extension`` array.

``ExtensionFilterProxy`` sits on top of a marketplace model to filter it by a
search term (name / id / description) and by extension type, mirroring the
web frontend's marketplace filters.
"""

from __future__ import annotations

from PySide6.QtCore import (
    QAbstractListModel,
    QByteArray,
    QModelIndex,
    QSortFilterProxyModel,
    Qt,
    Slot,
)

# Human labels for the backend's extension ``type`` values.
_TYPE_LABELS = {
    "anime-torrent-provider": "Torrent provider",
    "manga-provider": "Manga provider",
    "onlinestream-provider": "Streaming provider",
    "plugin": "Plugin",
    "custom-source": "Custom source",
}


def type_label(t: str) -> str:
    """Human-readable label for an extension ``type`` (falls back to Title Case)."""
    t = t or ""
    return _TYPE_LABELS.get(t) or (t.replace("-", " ").title() if t else "Extension")


def _row_of(
    ext: dict,
    *,
    disabled: bool = False,
    invalid: bool = False,
    invalid_reason: str = "",
    installed: bool = False,
) -> dict:
    """Flatten an Extension dict into the display row this model exposes."""
    ext = ext if isinstance(ext, dict) else {}
    manifest = ext.get("manifestURI") or ""
    return {
        "extId": ext.get("id") or "",
        "name": ext.get("name") or ext.get("id") or "",
        "version": ext.get("version") or "",
        "extType": ext.get("type") or "",
        "typeLabel": type_label(ext.get("type") or ""),
        "language": ext.get("language") or "",
        "lang": ext.get("lang") or "",
        "author": ext.get("author") or "",
        "description": ext.get("description") or "",
        "icon": ext.get("icon") or "",
        "website": ext.get("website") or "",
        "readme": ext.get("readme") or "",
        "manifestUri": manifest,
        "isBuiltin": manifest == "builtin",
        "disabled": bool(disabled),
        "invalid": bool(invalid),
        "invalidReason": invalid_reason or "",
        "installed": bool(installed),
    }


class ExtensionModel(QAbstractListModel):
    IdRole = Qt.ItemDataRole.UserRole + 1
    NameRole = Qt.ItemDataRole.UserRole + 2
    VersionRole = Qt.ItemDataRole.UserRole + 3
    TypeRole = Qt.ItemDataRole.UserRole + 4
    TypeLabelRole = Qt.ItemDataRole.UserRole + 5
    LanguageRole = Qt.ItemDataRole.UserRole + 6
    LangRole = Qt.ItemDataRole.UserRole + 7
    AuthorRole = Qt.ItemDataRole.UserRole + 8
    DescriptionRole = Qt.ItemDataRole.UserRole + 9
    IconRole = Qt.ItemDataRole.UserRole + 10
    WebsiteRole = Qt.ItemDataRole.UserRole + 11
    ReadmeRole = Qt.ItemDataRole.UserRole + 12
    ManifestUriRole = Qt.ItemDataRole.UserRole + 13
    IsBuiltinRole = Qt.ItemDataRole.UserRole + 14
    DisabledRole = Qt.ItemDataRole.UserRole + 15
    InvalidRole = Qt.ItemDataRole.UserRole + 16
    InvalidReasonRole = Qt.ItemDataRole.UserRole + 17
    InstalledRole = Qt.ItemDataRole.UserRole + 18

    _ROLES = {
        IdRole: b"extId",
        NameRole: b"name",
        VersionRole: b"version",
        TypeRole: b"extType",
        TypeLabelRole: b"typeLabel",
        LanguageRole: b"language",
        LangRole: b"lang",
        AuthorRole: b"author",
        DescriptionRole: b"description",
        IconRole: b"icon",
        WebsiteRole: b"website",
        ReadmeRole: b"readme",
        ManifestUriRole: b"manifestUri",
        IsBuiltinRole: b"isBuiltin",
        DisabledRole: b"disabled",
        InvalidRole: b"invalid",
        InvalidReasonRole: b"invalidReason",
        InstalledRole: b"installed",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []

    def load_installed(self, all_data) -> None:
        """Rebuild from an AllExtensions payload (enabled + disabled + invalid)."""
        data = all_data if isinstance(all_data, dict) else {}
        rows: list[dict] = []
        for ext in data.get("extensions") or []:
            rows.append(_row_of(ext))
        for ext in data.get("disabledExtensions") or []:
            rows.append(_row_of(ext, disabled=True))
        for inv in data.get("invalidExtensions") or []:
            inv = inv if isinstance(inv, dict) else {}
            ext = inv.get("extension") or {"id": inv.get("id")}
            reason = inv.get("reason") or inv.get("code") or "Invalid extension"
            rows.append(_row_of(ext, invalid=True, invalid_reason=reason))
        rows.sort(key=lambda r: (r["typeLabel"], r["name"].lower()))
        self._reset(rows)

    def load_marketplace(self, items, installed_ids) -> None:
        """Rebuild from a marketplace ``[]Extension`` array.

        ``installed_ids`` marks which items are already installed so the UI can
        show an "Installed" state instead of an install button.
        """
        installed = set(installed_ids or ())
        rows = [
            _row_of(ext, installed=(ext or {}).get("id") in installed)
            for ext in (items or [])
            if ext and ext.get("id")
        ]
        rows.sort(key=lambda r: r["name"].lower())
        self._reset(rows)

    def clear(self) -> None:
        self._reset([])

    def _reset(self, rows: list[dict]) -> None:
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


class ExtensionFilterProxy(QSortFilterProxyModel):
    """Filters a marketplace ExtensionModel by search text and extension type."""

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._search = ""
        self._type = ""
        self.setDynamicSortFilter(True)

    @Slot(str)
    def setSearchText(self, text: str) -> None:
        text = (text or "").strip().lower()
        if text != self._search:
            self._search = text
            self.invalidateFilter()

    @Slot(str)
    def setTypeFilter(self, ext_type: str) -> None:
        ext_type = ext_type or ""
        if ext_type != self._type:
            self._type = ext_type
            self.invalidateFilter()

    def filterAcceptsRow(self, source_row, source_parent) -> bool:
        model = self.sourceModel()
        if model is None:
            return True
        idx = model.index(source_row, 0, source_parent)
        if self._type:
            if (model.data(idx, ExtensionModel.TypeRole) or "") != self._type:
                return False
        if self._search:
            name = (model.data(idx, ExtensionModel.NameRole) or "").lower()
            ext_id = (model.data(idx, ExtensionModel.IdRole) or "").lower()
            desc = (model.data(idx, ExtensionModel.DescriptionRole) or "").lower()
            if (
                self._search not in name
                and self._search not in ext_id
                and self._search not in desc
            ):
                return False
        return True
