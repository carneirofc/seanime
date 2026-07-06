"""TokenCache — persistent per-server AniList access-token storage (QSettings).

Extracted from AppController: this is self-contained storage with no QML surface.
Tokens are scoped to the server they authenticate against, so switching servers
never reuses the wrong credentials.
"""

from __future__ import annotations

from PySide6.QtCore import QSettings


class TokenCache:
    def _settings(self) -> QSettings:
        return QSettings(
            QSettings.Format.IniFormat, QSettings.Scope.UserScope, "Seanime", "Seanime-Qt"
        )

    @staticmethod
    def _key(base_url: str) -> str:
        safe = "".join(ch if ch.isalnum() else "_" for ch in base_url)
        return f"anilistTokens/{safe}"

    def save(self, base_url: str, token: str) -> None:
        settings = self._settings()
        settings.setValue(self._key(base_url), token)
        settings.sync()

    def load(self, base_url: str) -> str:
        value = self._settings().value(self._key(base_url), "")
        return value if isinstance(value, str) else ""

    def clear(self, base_url: str) -> None:
        settings = self._settings()
        settings.remove(self._key(base_url))
        settings.sync()
