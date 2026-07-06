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

# Client-local UI appearance prefs, under a ``ui/`` group. Applied live by the
# QML Theme singleton (font scale, spacing density, colour mode, brand accent).
_UI_SCALE = "ui/scale"
_UI_DENSITY = "ui/density"
_UI_THEME = "ui/theme"
_UI_ACCENT = "ui/accent"
_UI_POSTER_SCALE = "ui/posterScale"
# Client-local override for the server's "split adult content" setting.
_UI_SPLIT_OVERRIDE = "ui/splitAdultOverride"


class SettingsStore:
    def _settings(self) -> QSettings:
        return QSettings(
            QSettings.Format.IniFormat, QSettings.Scope.UserScope, _ORG, _APP
        )

    @staticmethod
    def _str(value, default: str) -> str:
        return value if isinstance(value, str) and value != "" else default

    @staticmethod
    def _float(value, default: float) -> float:
        # QSettings' INI backend returns values as strings; coerce and fall back
        # to the default if the stored value is missing or malformed.
        try:
            return float(value)
        except (TypeError, ValueError):
            return default

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

    def ui_scale(self, default: float = 1.0) -> float:
        return self._float(self._settings().value(_UI_SCALE), default)

    def ui_density(self, default: float = 1.0) -> float:
        return self._float(self._settings().value(_UI_DENSITY), default)

    def ui_theme(self, default: str = "dark") -> str:
        return self._str(self._settings().value(_UI_THEME), default)

    def ui_accent(self, default: str = "#6152df") -> str:
        return self._str(self._settings().value(_UI_ACCENT), default)

    def ui_poster_scale(self, default: float = 1.0) -> float:
        return self._float(self._settings().value(_UI_POSTER_SCALE), default)

    def split_adult_override(self, default: str = "server") -> str:
        return self._str(self._settings().value(_UI_SPLIT_OVERRIDE), default)

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

    def save_ui_prefs(
        self,
        scale: float,
        density: float,
        theme: str,
        accent: str,
        poster_scale: float,
    ) -> None:
        settings = self._settings()
        settings.setValue(_UI_SCALE, float(scale))
        settings.setValue(_UI_DENSITY, float(density))
        settings.setValue(_UI_THEME, theme)
        settings.setValue(_UI_ACCENT, accent)
        settings.setValue(_UI_POSTER_SCALE, float(poster_scale))
        settings.sync()

    def save_split_adult_override(self, value: str) -> None:
        settings = self._settings()
        settings.setValue(_UI_SPLIT_OVERRIDE, value)
        settings.sync()
