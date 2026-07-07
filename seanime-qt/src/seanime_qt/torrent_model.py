"""TorrentModel — torrent search results for the download browser.

A ``QAbstractListModel`` with named roles, populated from the ``torrents`` array
of a ``/api/v1/torrent/search`` (SearchData) payload. It keeps the raw
``AnimeTorrent`` dicts internally so the full object can be handed back to the
download call via ``torrentAt``.
"""

from __future__ import annotations

from PySide6.QtCore import QAbstractListModel, QByteArray, QModelIndex, Qt, Slot


def human_size(num_bytes) -> str:
    """Format a byte count as a human-readable size (e.g. ``"1.5 GB"``).

    Returns ``""`` for non-positive or unparseable input so the UI can hide it.
    """
    try:
        size = float(num_bytes or 0)
    except (TypeError, ValueError):
        return ""
    if size <= 0:
        return ""
    units = ["B", "KB", "MB", "GB", "TB"]
    idx = 0
    while size >= 1024 and idx < len(units) - 1:
        size /= 1024
        idx += 1
    if idx == 0:
        return f"{int(size)} {units[idx]}"
    return f"{size:.1f} {units[idx]}"


def format_torrent_date(raw) -> str:
    """Reduce an RFC3339 timestamp to its ``YYYY-MM-DD`` date part.

    Anything that doesn't look like an ISO date is passed through unchanged.
    """
    s = str(raw or "")
    if len(s) >= 10 and s[4] == "-" and s[7] == "-":
        return s[:10]
    return s


class TorrentModel(QAbstractListModel):
    NameRole = Qt.ItemDataRole.UserRole + 1
    SizeRole = Qt.ItemDataRole.UserRole + 2
    SeedersRole = Qt.ItemDataRole.UserRole + 3
    LeechersRole = Qt.ItemDataRole.UserRole + 4
    ResolutionRole = Qt.ItemDataRole.UserRole + 5
    ReleaseGroupRole = Qt.ItemDataRole.UserRole + 6
    IsBatchRole = Qt.ItemDataRole.UserRole + 7
    IsBestReleaseRole = Qt.ItemDataRole.UserRole + 8
    ConfirmedRole = Qt.ItemDataRole.UserRole + 9
    DateRole = Qt.ItemDataRole.UserRole + 10
    LinkRole = Qt.ItemDataRole.UserRole + 11

    _ROLES = {
        NameRole: b"name",
        SizeRole: b"formattedSize",
        SeedersRole: b"seeders",
        LeechersRole: b"leechers",
        ResolutionRole: b"resolution",
        ReleaseGroupRole: b"releaseGroup",
        IsBatchRole: b"isBatch",
        IsBestReleaseRole: b"isBestRelease",
        ConfirmedRole: b"confirmed",
        DateRole: b"date",
        LinkRole: b"link",
    }

    def __init__(self, parent=None) -> None:
        super().__init__(parent)
        self._rows: list[dict] = []   # display dicts, keyed by role name
        self._raw: list[dict] = []    # raw AnimeTorrent dicts, for the download call

    def load(self, search_data) -> None:
        """Rebuild from a SearchData payload (reads its ``torrents`` array)."""
        data = search_data if isinstance(search_data, dict) else {}
        torrents = data.get("torrents") or []
        rows: list[dict] = []
        raw: list[dict] = []
        for t in torrents:
            t = t or {}
            raw.append(t)
            rows.append(
                {
                    "name": t.get("name") or "",
                    "formattedSize": t.get("formattedSize") or human_size(t.get("size")),
                    "seeders": int(t.get("seeders") or 0),
                    "leechers": int(t.get("leechers") or 0),
                    "resolution": t.get("resolution") or "",
                    "releaseGroup": t.get("releaseGroup") or "",
                    "isBatch": bool(t.get("isBatch")),
                    "isBestRelease": bool(t.get("isBestRelease")),
                    "confirmed": bool(t.get("confirmed")),
                    "date": format_torrent_date(t.get("date")),
                    "link": t.get("link") or "",
                }
            )
        self.beginResetModel()
        self._rows = rows
        self._raw = raw
        self.endResetModel()

    def clear(self) -> None:
        self.beginResetModel()
        self._rows = []
        self._raw = []
        self.endResetModel()

    @Slot(int, result="QVariant")
    def torrentAt(self, index: int):
        """Return the raw ``AnimeTorrent`` dict at ``index`` (for the download call)."""
        if 0 <= index < len(self._raw):
            return self._raw[index]
        return None

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
