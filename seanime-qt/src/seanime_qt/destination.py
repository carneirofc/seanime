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
