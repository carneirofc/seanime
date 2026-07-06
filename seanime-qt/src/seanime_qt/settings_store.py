"""SettingsStore — persistent app-local (client) preferences via QSettings.

Sibling of :mod:`token_cache`: self-contained storage with no QML surface. Holds the
handful of client-side settings the app needs to remember across launches — the server
connection (host/port/optional password token) and the AniList OAuth client
credentials — so the user doesn't retype them or rely on environment variables.

Server-side settings (library, media player, torrent, …) are NOT stored here; those
live on the Seanime server and are edited via the REST API.
"""

from __future__ import annotations

from PySide6.QtCore import QSettings

# Same organisation/application as TokenCache so both share one INI file.
_ORG = "Seanime"
_APP = "Seanime-Qt"

# Keys under a ``client/`` group, kept together and away from ``anilistTokens/``.
_HOST = "client/serverHost"
_PORT = "client/serverPort"
_TOKEN = "client/serverToken"
_CLIENT_ID = "client/anilistClientId"
_CLIENT_SECRET = "client/anilistClientSecret"


class SettingsStore:
    def _settings(self) -> QSettings:
        return QSettings(
            QSettings.Format.IniFormat, QSettings.Scope.UserScope, _ORG, _APP
        )

    @staticmethod
    def _str(value, default: str) -> str:
        return value if isinstance(value, str) and value != "" else default

    # ---- reads (each falls back to the caller-supplied default) ----------

    def server_host(self, default: str) -> str:
        return self._str(self._settings().value(_HOST), default)

    def server_port(self, default: str) -> str:
        return self._str(self._settings().value(_PORT), default)

    def server_token(self, default: str = "") -> str:
        return self._str(self._settings().value(_TOKEN), default)

    def anilist_client_id(self, default: str) -> str:
        return self._str(self._settings().value(_CLIENT_ID), default)

    def anilist_client_secret(self, default: str = "") -> str:
        return self._str(self._settings().value(_CLIENT_SECRET), default)

    # ---- writes ----------------------------------------------------------

    def save_connection(self, host: str, port: str, token: str) -> None:
        settings = self._settings()
        settings.setValue(_HOST, host)
        settings.setValue(_PORT, port)
        settings.setValue(_TOKEN, token)
        settings.sync()

    def save_anilist_credentials(self, client_id: str, client_secret: str) -> None:
        settings = self._settings()
        settings.setValue(_CLIENT_ID, client_id)
        settings.setValue(_CLIENT_SECRET, client_secret)
        settings.sync()
