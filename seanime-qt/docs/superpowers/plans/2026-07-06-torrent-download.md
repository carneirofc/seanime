# Torrent Download for Library Items — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user browse torrents for a library anime (from the detail page and per-episode) and send a chosen torrent to their configured torrent client, downloading into the library.

**Architecture:** seanime-qt is a thin client; the Seanime server already implements torrent search (`POST /api/v1/torrent/search`) and client download (`POST /api/v1/torrent-client/download`). This feature adds UI + API plumbing only: two `ApiClient` methods, a `TorrentModel` list model, pure destination helpers, `AppController` state/slots, and three new QML files, plus two entry-point buttons.

**Tech Stack:** Python 3.9+, PySide6 (Qt 6 / QML), QNetworkAccessManager for HTTP. No new dependencies. Tests use the stdlib `unittest` (the project has no pytest).

**Spec:** `seanime-qt/docs/superpowers/specs/2026-07-06-torrent-download-design.md`

**Conventions:** All commands run from `seanime-qt/`. Commit messages follow Conventional Commits with a `seanime-qt` scope. Do NOT add a `Co-Authored-By` trailer.

---

## File Structure

New files:
- `src/seanime_qt/destination.py` — pure helpers: `sanitize_directory_name`, `default_destination`.
- `src/seanime_qt/torrent_model.py` — `TorrentModel` (QAbstractListModel) + pure `human_size`, `format_torrent_date`.
- `tests/test_destination.py` — unittest for the pure helpers above.
- `qml/TorrentSearchView.qml` — the torrent browser page (search controls + results list).
- `qml/TorrentDelegate.qml` — one result row.
- `qml/DownloadConfirmDialog.qml` — destination confirm + download actions.

Modified files:
- `src/seanime_qt/api_client.py` — two signals + two methods.
- `src/seanime_qt/app_controller.py` — entry-payload capture, torrent state, properties, slots, signals, error reset.
- `qml/Icons.qml` — add a `download` glyph.
- `qml/DetailHeader.qml` — "Download" button.
- `qml/EpisodeDelegate.qml` — per-episode download button.
- `qml/Main.qml` — push the search page, host the confirm dialog.
- `CHANGELOG.md` — Added entry.

---

## Task 1: Pure destination helpers (`destination.py`)

**Files:**
- Create: `src/seanime_qt/destination.py`
- Test: `tests/test_destination.py`

- [ ] **Step 1: Write the failing test**

Create `tests/test_destination.py`:

```python
"""Unit tests for the pure torrent-destination helpers (no Qt required)."""

import os
import sys
import unittest

# Make ``seanime_qt`` importable when running from the repo root.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.destination import default_destination, sanitize_directory_name


class SanitizeDirectoryNameTests(unittest.TestCase):
    def test_replaces_disallowed_characters_with_spaces(self):
        self.assertEqual(sanitize_directory_name("Re:Zero / Season 2?"), "Re Zero Season 2")

    def test_collapses_and_trims_whitespace(self):
        self.assertEqual(sanitize_directory_name("  A   B  "), "A B")

    def test_strips_leading_and_trailing_dots(self):
        self.assertEqual(sanitize_directory_name("...Naruto..."), "Naruto")

    def test_empty_falls_back_to_untitled(self):
        self.assertEqual(sanitize_directory_name(""), "Untitled")
        self.assertEqual(sanitize_directory_name("///"), "Untitled")


class DefaultDestinationTests(unittest.TestCase):
    def test_uses_dirname_of_last_local_file(self):
        files = [
            {"path": "/library/Naruto/ep1.mkv"},
            {"path": "/library/Naruto/ep2.mkv"},
        ]
        self.assertEqual(default_destination(files, "/library", "Naruto"), "/library/Naruto")

    def test_normalizes_windows_separators(self):
        files = [{"path": r"D:\Anime\Bleach\ep1.mkv"}]
        self.assertEqual(default_destination(files, "D:/Anime", "Bleach"), "D:/Anime/Bleach")

    def test_joins_library_path_and_sanitized_title_when_no_files(self):
        self.assertEqual(default_destination([], "/library", "Re:Zero"), "/library/Re Zero")

    def test_empty_when_no_files_and_no_library_path(self):
        self.assertEqual(default_destination([], "", "Whatever"), "")


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `python -m unittest tests.test_destination -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'seanime_qt.destination'`.

- [ ] **Step 3: Write the implementation**

Create `src/seanime_qt/destination.py`:

```python
"""Pure helpers for computing a torrent download destination.

No Qt imports, so these are unit-testable in isolation. Ported from the web
client's ``torrent-download-file-selection.tsx`` (getDefaultDestination /
sanitizeDirectoryName).
"""

from __future__ import annotations

import posixpath
import re

# Characters not allowed in a directory name on common filesystems, plus C0
# control characters.
_DISALLOWED = re.compile(r'[<>:"/\\|?*\x00-\x1f]')


def sanitize_directory_name(name: str) -> str:
    """Make ``name`` safe to use as a single directory name.

    Replaces disallowed characters with spaces, collapses runs of whitespace,
    strips leading/trailing dots and spaces, and falls back to ``"Untitled"``.
    """
    sanitized = _DISALLOWED.sub(" ", name or "")
    sanitized = re.sub(r"\s+", " ", sanitized).strip()
    sanitized = sanitized.strip(".").strip()
    return sanitized or "Untitled"


def default_destination(local_files, library_path: str, romaji_title: str) -> str:
    """Compute the default download destination for an anime entry.

    Mirrors the web client: if the entry already has local files, download next
    to the last one (its parent directory); otherwise, if a library path is
    configured, use ``<library_path>/<sanitized title>``; else ``""``. Paths are
    normalised to forward slashes so results are deterministic across platforms
    (the Go server accepts forward slashes on Windows).
    """
    files = [f for f in (local_files or []) if f]
    if files:
        last_path = (files[-1] or {}).get("path") or ""
        if last_path:
            return posixpath.dirname(last_path.replace("\\", "/"))
    if library_path:
        return posixpath.join(library_path.replace("\\", "/"), sanitize_directory_name(romaji_title))
    return ""
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `python -m unittest tests.test_destination -v`
Expected: PASS (8 tests OK).

- [ ] **Step 5: Commit**

```bash
git add src/seanime_qt/destination.py tests/test_destination.py
git commit -m "feat(seanime-qt): add pure torrent destination helpers"
```

---

## Task 2: TorrentModel + size/date helpers (`torrent_model.py`)

**Files:**
- Create: `src/seanime_qt/torrent_model.py`
- Test: `tests/test_torrent_model.py`

- [ ] **Step 1: Write the failing test** (covers the pure helpers only; the model itself is verified via the running app in Task 8)

Create `tests/test_torrent_model.py`:

```python
"""Unit tests for the pure helpers in torrent_model (no running Qt app needed)."""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.torrent_model import format_torrent_date, human_size


class HumanSizeTests(unittest.TestCase):
    def test_bytes(self):
        self.assertEqual(human_size(512), "512 B")

    def test_kilobytes(self):
        self.assertEqual(human_size(2048), "2.0 KB")

    def test_gigabytes(self):
        self.assertEqual(human_size(1610612736), "1.5 GB")

    def test_zero_and_invalid(self):
        self.assertEqual(human_size(0), "")
        self.assertEqual(human_size(None), "")
        self.assertEqual(human_size("nope"), "")


class FormatTorrentDateTests(unittest.TestCase):
    def test_reduces_rfc3339_to_date(self):
        self.assertEqual(format_torrent_date("2023-01-02T15:04:05Z"), "2023-01-02")

    def test_passes_through_non_timestamp(self):
        self.assertEqual(format_torrent_date("yesterday"), "yesterday")

    def test_empty(self):
        self.assertEqual(format_torrent_date(None), "")


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `python -m unittest tests.test_torrent_model -v`
Expected: FAIL — `ModuleNotFoundError: No module named 'seanime_qt.torrent_model'`.

- [ ] **Step 3: Write the implementation**

Create `src/seanime_qt/torrent_model.py`:

```python
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `python -m unittest tests.test_torrent_model -v`
Expected: PASS (6 tests OK).

- [ ] **Step 5: Commit**

```bash
git add src/seanime_qt/torrent_model.py tests/test_torrent_model.py
git commit -m "feat(seanime-qt): add TorrentModel for search results"
```

---

## Task 3: ApiClient torrent endpoints

**Files:**
- Modify: `src/seanime_qt/api_client.py`

- [ ] **Step 1: Add the two signals**

In `src/seanime_qt/api_client.py`, in the signal block, right after the line
`progressUpdated = Signal("QVariant")   # episode progress update succeeded` (currently line 47), add:

```python
    torrentSearchReceived = Signal("QVariant")     # torrent search results
    torrentDownloadSucceeded = Signal("QVariant")  # torrents handed to the client
```

- [ ] **Step 2: Add the two methods**

In the same file, immediately after the `update_progress(...)` method (it ends at the line `self.progressUpdated,\n        )` around line 193), add:

```python
    def search_torrent(self, body: dict) -> None:
        """POST a torrent search; the result routes to ``torrentSearchReceived``.

        ``body`` carries ``{type, provider, query, episodeNumber, batch,
        resolution, media}``. An empty ``provider`` makes the server use the
        user's default torrent provider.
        """
        self._post_json("/api/v1/torrent/search", body, self.torrentSearchReceived)

    def torrent_client_download(self, body: dict) -> None:
        """POST selected torrents to the configured torrent client.

        ``body`` carries ``{torrents, destination, smartSelect, media}``.
        """
        self._post_json(
            "/api/v1/torrent-client/download", body, self.torrentDownloadSucceeded
        )
```

- [ ] **Step 3: Verify the module imports**

Run: `python -c "import sys; sys.path.insert(0, 'src'); from seanime_qt.api_client import ApiClient; print('ok')"`
Expected: prints `ok` (no syntax/import errors).

- [ ] **Step 4: Commit**

```bash
git add src/seanime_qt/api_client.py
git commit -m "feat(seanime-qt): add torrent search + download API methods"
```

---

## Task 4: AppController — state, properties, slots, wiring

**Files:**
- Modify: `src/seanime_qt/app_controller.py`

- [ ] **Step 1: Add imports**

Near the other model imports (after `from .search_model import SearchModel`, line ~42) add:

```python
from .torrent_model import TorrentModel
```

And after `from .adult_filter import AdultFilterProxy` (line ~33) add:

```python
from .destination import default_destination
```

- [ ] **Step 2: Declare the new signals**

In the class-level signal block (after `detailTagsChanged = Signal()`, line ~106) add:

```python
    # Torrent download.
    torrentSearchOpened = Signal()     # QML pushes the torrent search page
    torrentStateChanged = Signal()     # torrent loading / results / selection changed
    torrentDownloadReady = Signal()    # QML opens the download confirm dialog
    torrentDownloadStarted = Signal()  # a download was accepted; QML closes + returns
```

- [ ] **Step 3: Create the model and state in `__init__`**

After `self._character_model = CharacterModel(self)` (line ~120) add:

```python
        self._torrent_model = TorrentModel(self)
```

After the detail state block, near `self._detail_tags: list = []` (line ~187) add:

```python
        # Torrent download state (for the open anime).
        self._entry_media: dict = {}
        self._entry_local_files: list = []
        self._entry_download_info: dict = {}
        self._torrent_search_loading = False
        self._torrent_search_episode = -1
        self._torrent_search_batch = False
        self._torrent_selected: dict | None = None
        self._torrent_selected_name = ""
        self._torrent_can_smart_select = False
        self._torrent_default_destination = ""
        self._torrent_downloading = False
```

- [ ] **Step 4: Wire the client signals**

After `self._client.progressUpdated.connect(self._on_progress_updated)` (line ~268) add:

```python
        self._client.torrentSearchReceived.connect(self._on_torrent_search)
        self._client.torrentDownloadSucceeded.connect(self._on_torrent_download_ok)
```

- [ ] **Step 5: Add the model getter + Property**

After `def _get_character_model(...)` / near the model getters (line ~314) add a getter:

```python
    def _get_torrent_model(self) -> QObject:
        return self._torrent_model
```

And in the Property block near `characterModel = Property(...)` (line ~360) add:

```python
    torrentModel = Property(QObject, _get_torrent_model, constant=True)
```

- [ ] **Step 6: Add the scalar getters + Properties**

After the `detailListProgress = Property(...)` line (line ~460) add:

```python
    def _get_torrent_search_loading(self) -> bool:
        return self._torrent_search_loading

    def _get_torrent_search_episode(self) -> int:
        return self._torrent_search_episode

    def _get_torrent_search_batch(self) -> bool:
        return self._torrent_search_batch

    def _get_torrent_selected_name(self) -> str:
        return self._torrent_selected_name

    def _get_torrent_can_smart_select(self) -> bool:
        return self._torrent_can_smart_select

    def _get_torrent_default_destination(self) -> str:
        return self._torrent_default_destination

    def _get_torrent_downloading(self) -> bool:
        return self._torrent_downloading

    torrentSearchLoading = Property(bool, _get_torrent_search_loading, notify=torrentStateChanged)
    torrentSearchEpisode = Property(int, _get_torrent_search_episode, notify=torrentStateChanged)
    torrentSearchBatch = Property(bool, _get_torrent_search_batch, notify=torrentStateChanged)
    torrentSelectedName = Property(str, _get_torrent_selected_name, notify=torrentStateChanged)
    torrentCanSmartSelect = Property(bool, _get_torrent_can_smart_select, notify=torrentStateChanged)
    torrentDefaultDestination = Property(str, _get_torrent_default_destination, notify=torrentStateChanged)
    torrentDownloading = Property(bool, _get_torrent_downloading, notify=torrentStateChanged)
```

- [ ] **Step 7: Add the slots**

After the `setEpisodeProgress(...)` slot (it ends around line 1052, before `# ---- manga slots ----`) add:

```python
    # ---- torrent download slots -----------------------------------------

    @Slot(int, bool)
    def openTorrentSearch(self, episode_number: int, batch: bool) -> None:
        """Open the torrent browser for the open anime and run an initial search.

        ``episode_number`` is the episode to pre-fill (<= 0 for none); ``batch``
        pre-checks the batch toggle.
        """
        if not self._detail_media_id or not self._entry_media:
            return
        self._torrent_model.clear()
        self._torrent_selected = None
        self._torrent_selected_name = ""
        self._torrent_can_smart_select = False
        self._torrent_search_episode = int(episode_number)
        self._torrent_search_batch = bool(batch)
        self._set_error("")
        self.torrentStateChanged.emit()
        self.torrentSearchOpened.emit()
        self.runTorrentSearch("", int(episode_number), bool(batch), "")

    @Slot(str, int, bool, str)
    def runTorrentSearch(self, query: str, episode_number: int, batch: bool, resolution: str) -> None:
        """Smart-search the server for torrents matching the open anime."""
        if not self._detail_media_id or not self._entry_media:
            return
        self._torrent_search_episode = int(episode_number)
        self._torrent_search_batch = bool(batch)
        self._torrent_search_loading = True
        self._set_error("")
        self.torrentStateChanged.emit()
        body = {
            "type": "smart",
            "provider": "",  # empty -> server uses the default torrent provider
            "query": query or "",
            "episodeNumber": int(episode_number) if episode_number and episode_number > 0 else 0,
            "batch": bool(batch),
            "resolution": resolution or "",
            "media": self._entry_media,
        }
        self._client.search_torrent(body)

    @Slot(int)
    def selectTorrent(self, index: int) -> None:
        """Pick a result and prepare the download confirm dialog."""
        torrent = self._torrent_model.torrentAt(index)
        if not torrent:
            return
        self._torrent_selected = torrent
        self._torrent_selected_name = torrent.get("name") or ""
        library = self._settings.get("library") if isinstance(self._settings, dict) else None
        library_path = (library or {}).get("libraryPath") or ""
        romaji = (self._entry_media.get("title") or {}).get("romaji") or self._detail_title or ""
        self._torrent_default_destination = default_destination(
            self._entry_local_files, library_path, romaji
        )
        self._torrent_can_smart_select = self._compute_can_smart_select(torrent)
        self.torrentStateChanged.emit()
        self.torrentDownloadReady.emit()

    @Slot(str, bool)
    def startTorrentDownload(self, destination: str, smart_select: bool) -> None:
        """Send the selected torrent to the configured client at ``destination``."""
        if not self._torrent_selected:
            return
        dest = (destination or "").strip()
        if not dest or not os.path.isabs(dest):
            self._set_error("Enter an absolute destination path.")
            return
        missing: list[int] = []
        if smart_select:
            to_download = (self._entry_download_info or {}).get("episodesToDownload") or []
            missing = [
                int(e.get("episodeNumber"))
                for e in to_download
                if e and e.get("episodeNumber") is not None
            ]
        body = {
            "torrents": [self._torrent_selected],
            "destination": dest,
            "smartSelect": {"enabled": bool(smart_select), "missingEpisodeNumbers": missing},
            "media": self._entry_media,
        }
        self._torrent_downloading = True
        self._set_error("")
        self.torrentStateChanged.emit()
        self._client.torrent_client_download(body)

    def _compute_can_smart_select(self, torrent: dict) -> bool:
        """Mirror the web client's ``canSmartSelect``: a batch of a finished
        multi-episode series that still has some (but not all) episodes missing."""
        if not torrent.get("isBatch"):
            return False
        media = self._entry_media or {}
        if (media.get("format") or "") == "MOVIE":
            return False
        if (media.get("status") or "") != "FINISHED":
            return False
        episodes = int(media.get("episodes") or 0)
        if episodes <= 1:
            return False
        to_download = (self._entry_download_info or {}).get("episodesToDownload") or []
        if not to_download:
            return False
        return len(to_download) != episodes
```

- [ ] **Step 8: Add the response handlers**

After `_on_progress_updated(...)` (line ~1270) add:

```python
    def _on_torrent_search(self, data) -> None:
        self._torrent_model.load(data)
        self._torrent_search_loading = False
        self.torrentStateChanged.emit()

    def _on_torrent_download_ok(self, _data) -> None:
        self._torrent_downloading = False
        self.torrentStateChanged.emit()
        self.torrentDownloadStarted.emit()
```

- [ ] **Step 9: Capture entry data in `_on_anime_entry`**

In `_on_anime_entry`, right after `media = data.get("media") or {}` (line ~1186) add:

```python
        self._entry_media = media
        self._entry_local_files = data.get("localFiles") or []
        self._entry_download_info = data.get("downloadInfo") or {}
```

- [ ] **Step 10: Reset torrent state in `_reset_detail` and release busy flags on error**

In `_reset_detail`, after `self.detailTagsChanged.emit()` (line ~1374) add:

```python
        self._entry_media = {}
        self._entry_local_files = []
        self._entry_download_info = {}
        self._torrent_selected = None
        self._torrent_selected_name = ""
        self._torrent_can_smart_select = False
        self._torrent_default_destination = ""
        self._torrent_search_loading = False
        self._torrent_downloading = False
        self._torrent_model.clear()
        self.torrentStateChanged.emit()
```

In `_on_error` (line ~1337), before `self._set_error(message)` add:

```python
        if self._torrent_search_loading or self._torrent_downloading:
            self._torrent_search_loading = False
            self._torrent_downloading = False
            self.torrentStateChanged.emit()
```

- [ ] **Step 11: Verify the module imports**

Run: `python -c "import sys; sys.path.insert(0, 'src'); from seanime_qt.app_controller import AppController; print('ok')"`
Expected: prints `ok`.

- [ ] **Step 12: Commit**

```bash
git add src/seanime_qt/app_controller.py
git commit -m "feat(seanime-qt): wire torrent search + download into AppController"
```

---

## Task 5: Add the `download` icon glyph

**Files:**
- Modify: `qml/Icons.qml`

- [ ] **Step 1: Add the glyph**

> IMPORTANT: the `download` glyph value must be written as the literal 6-character
> escape sequence backslash-`u`-`e`-`a`-`9`-`6` (Tabler's `download` codepoint,
> matching how the other entries use `\uXXXX`). Do NOT paste a rendered glyph — the
> code block below may display it as an invisible character.

In `qml/Icons.qml`, in the `glyphs` map, in the "actions / status" section, after the `"photo": "",` line add:

```qml
        "download":                "",
```

- [ ] **Step 2: Commit** (visual verification happens in Task 8)

```bash
git add qml/Icons.qml
git commit -m "feat(seanime-qt): add download icon glyph"
```

> Note: `` is Tabler's `download` codepoint. If it renders as a wrong glyph or blank during Task 8 verification, replace the affected buttons' `iconName: "download"` with text-only buttons and remove this glyph.

---

## Task 6: TorrentDelegate + TorrentSearchView + DownloadConfirmDialog QML

**Files:**
- Create: `qml/TorrentDelegate.qml`
- Create: `qml/TorrentSearchView.qml`
- Create: `qml/DownloadConfirmDialog.qml`

- [ ] **Step 1: Create `qml/TorrentDelegate.qml`**

```qml
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One torrent search result row: name + metadata chips + a Download action.
// Tapping the row or the button asks to download this torrent (emits selected()).
Rectangle {
    id: row
    required property string name
    required property string formattedSize
    required property int seeders
    required property int leechers
    required property string resolution
    required property string releaseGroup
    required property bool isBatch
    required property bool isBestRelease
    required property bool confirmed
    required property string date
    signal selected()

    height: content.implicitHeight + 20
    radius: Theme.radius
    color: hover.hovered ? Theme.surfaceHover : Theme.surface
    border.width: 1
    border.color: hover.hovered ? Theme.border : "transparent"
    Behavior on color { ColorAnimation { duration: Theme.durFast } }

    Accessible.role: Accessible.ListItem
    Accessible.name: row.name

    HoverHandler { id: hover }
    TapHandler { onTapped: row.selected() }

    RowLayout {
        id: content
        anchors.fill: parent
        anchors.margins: 10
        spacing: 12

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 6
            Label {
                Layout.fillWidth: true
                text: row.name
                color: Theme.text
                font.pixelSize: Theme.fontBase
                font.bold: true
                elide: Text.ElideRight
            }
            Flow {
                Layout.fillWidth: true
                spacing: 6
                Chip {
                    visible: row.confirmed
                    text: "Confirmed"
                    icon: "circle-check"
                    textColor: Theme.successText
                    fillColor: Theme.successFill
                }
                Chip {
                    visible: row.isBestRelease
                    text: "Best"
                    textColor: Theme.warnText
                    fillColor: Theme.warnFill
                }
                Chip { visible: row.isBatch; text: "Batch" }
                Chip { visible: row.resolution.length > 0; text: row.resolution }
                Chip { visible: row.formattedSize.length > 0; text: row.formattedSize }
                Chip {
                    text: row.seeders + " S"
                    textColor: Theme.successText
                    fillColor: Theme.successFill
                }
                Chip { text: row.leechers + " L" }
                Chip { visible: row.releaseGroup.length > 0; text: row.releaseGroup }
                Chip { visible: row.date.length > 0; text: row.date }
            }
        }

        AppButton {
            text: "Download"
            iconName: "download"
            Layout.alignment: Qt.AlignVCenter
            onClicked: row.selected()
        }
    }
}
```

- [ ] **Step 2: Create `qml/TorrentSearchView.qml`**

```qml
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Torrent browser for the open library anime. Smart-searches the Seanime server
// (provider left to the server default) and lists results; tapping a result asks
// the AppController to open the download confirm dialog.
Item {
    id: root
    signal back()

    readonly property var resolutionOptions: [
        { label: "Any resolution", value: "" },
        { label: "1080p", value: "1080" },
        { label: "720p", value: "720" },
        { label: "540p", value: "540" },
        { label: "480p", value: "480" }
    ]

    function runSearch() {
        app.runTorrentSearch(
            queryField.text,
            parseInt(episodeField.text) || 0,
            batchSwitch.checked,
            resolutionCombo.currentValue)
    }

    // Seed the controls from the search context the controller set when opening.
    Component.onCompleted: {
        batchSwitch.checked = app.torrentSearchBatch
        if (app.torrentSearchEpisode > 0)
            episodeField.text = app.torrentSearchEpisode
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        // Top bar: back + title.
        RowLayout {
            Layout.fillWidth: true
            spacing: 10
            AppButton {
                objectName: "torrentBackButton"
                iconName: "arrow-left"
                text: "Back"
                onClicked: root.back()
            }
            Label {
                Layout.fillWidth: true
                text: "Download · " + app.detailTitle
                color: Theme.textStrong
                font.pixelSize: Theme.fontXl
                font.bold: true
                elide: Text.ElideRight
            }
        }

        // Search controls.
        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            AppTextField {
                id: queryField
                objectName: "torrentQueryField"
                Layout.fillWidth: true
                placeholderText: "Refine query (optional)…"
                onAccepted: root.runSearch()
            }
            AppComboBox {
                id: resolutionCombo
                objectName: "torrentResolutionCombo"
                width: 150
                textRole: "label"
                valueRole: "value"
                model: root.resolutionOptions
            }
            AppTextField {
                id: episodeField
                objectName: "torrentEpisodeField"
                width: 90
                placeholderText: "Episode"
                inputMethodHints: Qt.ImhDigitsOnly
                validator: IntValidator { bottom: 0; top: 9999 }
                onAccepted: root.runSearch()
            }
            AppSwitch {
                id: batchSwitch
                objectName: "torrentBatchSwitch"
                text: "Batch"
            }
            AppButton {
                objectName: "torrentSearchButton"
                text: "Search"
                onClicked: root.runSearch()
            }
        }

        // Loading + empty states.
        Label {
            Layout.fillWidth: true
            visible: app.torrentSearchLoading
            text: "Searching torrents…"
            color: Theme.textMuted
            font.pixelSize: Theme.fontLg
            horizontalAlignment: Text.AlignHCenter
        }
        Label {
            Layout.fillWidth: true
            visible: !app.torrentSearchLoading && list.count === 0
            text: "No torrents found. Try a different query or filters."
            color: Theme.textMuted
            font.pixelSize: Theme.fontLg
            horizontalAlignment: Text.AlignHCenter
        }

        ListView {
            id: list
            objectName: "torrentList"
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            spacing: 8
            model: app.torrentModel
            ScrollBar.vertical: ScrollBar {}
            delegate: TorrentDelegate {
                width: list.width
                onSelected: app.selectTorrent(index)
            }
        }
    }
}
```

- [ ] **Step 3: Create `qml/DownloadConfirmDialog.qml`**

```qml
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Confirms the download destination for the selected torrent, then sends it to
// the configured torrent client via the AppController. When the picked torrent
// is a batch of a finished series with missing episodes, also offers a
// "Download missing episodes" (smart-select) action.
Dialog {
    id: dialog
    objectName: "downloadConfirmDialog"
    modal: true
    title: "Download to library"
    anchors.centerIn: Overlay.overlay
    width: 460
    closePolicy: Popup.CloseOnEscape

    // Pre-fill the destination from the controller each time it opens.
    onOpened: destinationField.text = app.torrentDefaultDestination

    background: Rectangle { color: Theme.surface; radius: Theme.radius; border.color: Theme.border }

    enter: Transition {
        ParallelAnimation {
            NumberAnimation { property: "opacity"; from: 0; to: 1; duration: Theme.durBase; easing.type: Theme.easeStandard }
            NumberAnimation { property: "scale"; from: 0.96; to: 1; duration: Theme.durBase; easing.type: Theme.easeEmphasis }
        }
    }
    exit: Transition {
        NumberAnimation { property: "opacity"; from: 1; to: 0; duration: Theme.durFast }
    }

    contentItem: ColumnLayout {
        spacing: 12

        Label {
            Layout.fillWidth: true
            text: app.torrentSelectedName
            color: Theme.text
            font.pixelSize: Theme.fontBase
            font.bold: true
            wrapMode: Text.WordWrap
        }

        Label { text: "Destination"; color: Theme.textDim; font.pixelSize: Theme.fontSm }
        AppTextField {
            id: destinationField
            objectName: "torrentDestinationField"
            Layout.fillWidth: true
            placeholderText: "Absolute path in your library…"
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 8
            AppButton {
                objectName: "torrentDownloadConfirmButton"
                Layout.fillWidth: true
                text: "Download"
                iconName: "download"
                enabled: !app.torrentDownloading && destinationField.text.trim().length > 0
                onClicked: app.startTorrentDownload(destinationField.text, false)
            }
            AppButton {
                objectName: "torrentDownloadMissingButton"
                Layout.fillWidth: true
                visible: app.torrentCanSmartSelect
                text: "Download missing episodes"
                enabled: !app.torrentDownloading && destinationField.text.trim().length > 0
                onClicked: app.startTorrentDownload(destinationField.text, true)
            }
            AppButton {
                objectName: "torrentDownloadCancelButton"
                Layout.fillWidth: true
                text: "Cancel"
                onClicked: dialog.close()
            }
        }
    }
}
```

- [ ] **Step 4: Commit** (wired up + verified in Tasks 7–8)

```bash
git add qml/TorrentDelegate.qml qml/TorrentSearchView.qml qml/DownloadConfirmDialog.qml
git commit -m "feat(seanime-qt): add torrent browser + download confirm QML"
```

---

## Task 7: Wire entry points + navigation

**Files:**
- Modify: `qml/DetailHeader.qml`
- Modify: `qml/EpisodeDelegate.qml`
- Modify: `qml/Main.qml`

- [ ] **Step 1: Add the detail-page Download button**

In `qml/DetailHeader.qml`, in the "List-entry status + edit action" `RowLayout`, between the `Item { Layout.fillWidth: true }` spacer and the `editListButton` (line ~141–146), insert:

```qml
                AppButton {
                    objectName: "downloadButton"
                    text: "Download"
                    iconName: "download"
                    onClicked: app.openTorrentSearch(-1, false)
                }
```

(Result order in the row: status label, spacer, Download button, Edit-list button.)

- [ ] **Step 2: Add the per-episode download button**

In `qml/EpisodeDelegate.qml`, immediately before the existing "Mark watched" `AppButton` (line ~92–97), insert:

```qml
        // Search torrents for just this episode.
        AppButton {
            objectName: "episodeDownloadButton_" + progressNumber
            iconName: "download"
            onClicked: app.openTorrentSearch(parseInt(number) || progressNumber, false)
        }
```

- [ ] **Step 3: Wire navigation + host the dialog in `qml/Main.qml`**

In the `Connections { target: app ... }` block, after `function onTagSearchRequested(tag) { window.showPage(searchComponent, "search") }` (line ~60) add:

```qml
        // Torrent download: push the browser, host the confirm dialog, and pop
        // back to the detail page once a download has been handed to the client.
        function onTorrentSearchOpened() { stack.push(torrentSearchComponent) }
        function onTorrentDownloadReady() { downloadConfirmDialog.open() }
        function onTorrentDownloadStarted() { downloadConfirmDialog.close(); stack.pop() }
```

After the `detailComponent` `Component { ... }` block (line ~187) add:

```qml
    Component {
        id: torrentSearchComponent
        TorrentSearchView {
            onBack: stack.pop()
        }
    }
```

As the last child of `ApplicationWindow` (just before the final closing `}` of the window, after the `settingsComponent` Component at line ~242), add:

```qml
    DownloadConfirmDialog {
        id: downloadConfirmDialog
    }
```

- [ ] **Step 4: Commit**

```bash
git add qml/DetailHeader.qml qml/EpisodeDelegate.qml qml/Main.qml
git commit -m "feat(seanime-qt): add download entry points and browser navigation"
```

---

## Task 8: End-to-end verification + CHANGELOG

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Run the full unit-test suite**

Run: `python -m unittest discover -s tests -v`
Expected: all tests from Tasks 1–2 pass (14 tests OK).

- [ ] **Step 2: Launch the app and verify the flow via the MCP agent tools**

Start the app (`mcp__seanime-qt__app_start`), then:
1. `get_logs` immediately after load — expect NO new QML warnings (no "unknown icon name 'download'", no "Cannot read property" / "is not a function" for the new `app.*` members).
2. Open a library anime (click an `AnimeCard`), landing on the detail page.
3. Verify the detail-page `downloadButton` is visible with a rendered download icon (not tofu/blank). If blank, apply the Task 5 fallback (text-only buttons).
4. Click `downloadButton` → the `torrentSearchComponent` page pushes; an initial search runs (`torrentList` populates, or the "No torrents found" state shows). Confirm no errors in `get_logs`.
5. Click a `TorrentDelegate`'s Download button (or the row) → the `downloadConfirmDialog` opens with `torrentDestinationField` pre-filled (a non-empty absolute path when a library path is configured).
6. Go back, open an episode row's `episodeDownloadButton_*` → the browser opens with the episode number pre-filled in `torrentEpisodeField`.
7. (If a torrent client is configured) click Download and confirm the request fires (dialog closes, returns to detail, no error banner). If none is configured, confirm the server error surfaces in the `errorBanner` and the busy state clears (button re-enables).

Capture a `screenshot` of the torrent browser and the confirm dialog for the completion report.

- [ ] **Step 3: Add the CHANGELOG entry**

In `CHANGELOG.md`, under `## [Unreleased]` → `### Added`, add as the first bullet:

```markdown
- **Torrent download**: a "Download" button on the anime detail page and a
  per-episode download button open a torrent browser for the entry. It
  smart-searches the Seanime server (using the default torrent provider),
  lists results with seeders/size/resolution and batch/best-release/confirmed
  badges, and — after confirming an auto-filled destination (existing local
  folder, else `<library path>/<title>`) — sends the chosen torrent to the
  configured torrent client. Batch torrents of a finished series also offer
  "Download missing episodes" (smart-select).
```

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(seanime-qt): changelog for torrent download feature"
```

---

## Self-Review Notes (addressed)

- **Spec coverage:** entry points (Task 7), search (Tasks 3–4, 6), TorrentModel (Task 2), destination default + editable field (Tasks 1, 4, 6), single-torrent + batch smart-select (Task 4 `_compute_can_smart_select`, `startTorrentDownload`; Task 6 dialog), error handling (Task 4 `_on_error`; Task 6 disabled buttons), testing (Tasks 1–2, 8). All covered.
- **Type consistency:** signal name `torrentStateChanged` and slot/property names (`openTorrentSearch`, `runTorrentSearch`, `selectTorrent`, `startTorrentDownload`, `torrentModel`, `torrentDefaultDestination`, `torrentCanSmartSelect`, `torrentDownloading`, `torrentSelectedName`, `torrentSearchEpisode`, `torrentSearchBatch`, `torrentSearchLoading`) are used identically in the Python (Task 4) and QML (Tasks 6–7).
- **Model access from QML:** the empty-state binds to `list.count` (ListView), not a model method, since `QAbstractListModel.rowCount` is not a QML property.
- **Icon risk:** the `download` glyph is isolated in Task 5 with a documented text-only fallback verified in Task 8.
