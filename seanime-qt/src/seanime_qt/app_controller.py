"""AppController — orchestration object exposed to QML as ``app``.

Owns the ApiClient and the two list models, wires the client's signals into
model updates and observable state, and exposes slots the QML calls. Every
member here has a direct C++ Qt analog (Q_PROPERTY / Q_INVOKABLE / signals).
"""

from __future__ import annotations

import os

from PySide6.QtCore import (
    Property,
    QObject,
    QSortFilterProxyModel,
    Qt,
    QUrl,
    QUrlQuery,
    Signal,
    Slot,
)
from PySide6.QtQml import QJSValue

from .anilist_queries import (
    discover_bodies,
    humanize,
    media_list_of,
    next_airing_text,
    search_body,
    strip_html,
)
from .adult_filter import AdultFilterProxy
from .api_client import ApiClient
from .chapter_model import ChapterModel
from .character_model import CharacterModel
from .episode_model import EpisodeModel
from .library_model import LibraryModel
from .manga_library_model import MangaLibraryModel
from .media_tags import MEDIA_TAGS
from .page_model import PageModel
from .search_model import SearchModel
from .settings_store import SettingsStore
from .token_cache import TokenCache

# AniList's OAuth authorize endpoint. AniList has disabled the implicit grant, so we
# use the authorization-code grant (response_type=code): the redirect delivers a
# short-lived ``code`` in the query, which is exchanged (with the client secret) for
# an access token at the token endpoint.
_ANILIST_AUTHORIZE_URL = "https://anilist.co/api/v2/oauth/authorize"
# The registered AniList API client. Overridable via env / QML.
_DEFAULT_ANILIST_CLIENT_ID = os.environ.get("SEANIME_QT_ANILIST_CLIENT_ID", "45180")
# The client secret is required by the authorization-code grant. A desktop app cannot
# truly keep a secret, so for this PoC it is read from the environment rather than
# baked into source; set SEANIME_QT_ANILIST_CLIENT_SECRET before launching.
_DEFAULT_ANILIST_CLIENT_SECRET = os.environ.get("SEANIME_QT_ANILIST_CLIENT_SECRET", "")


class AppController(QObject):
    connectionStatusChanged = Signal()
    errorMessageChanged = Signal()
    detailChanged = Signal()
    animeOpened = Signal()  # QML pushes the detail page on this
    authStateChanged = Signal()
    loginFinished = Signal()  # emitted on successful login; QML closes the page
    # Settings.
    settingsChanged = Signal()      # server settings dict refreshed (from status/PATCH)
    settingsSaved = Signal()        # a server-settings PATCH succeeded (QML confirms)
    clientPrefsChanged = Signal()   # persisted client prefs (host/port/token) changed
    # Manga.
    mangaDetailChanged = Signal()
    mangaProvidersChanged = Signal()
    readerChanged = Signal()
    mangaOpened = Signal()    # QML pushes the manga detail page on this
    chapterOpened = Signal()  # QML pushes the reader page on this

    def __init__(self, parent: QObject | None = None) -> None:
        super().__init__(parent)

        self._client = ApiClient(self)
        self._token_cache = TokenCache()
        self._store = SettingsStore()
        self._library_model = LibraryModel(self)
        self._episode_model = EpisodeModel(self)
        self._search_model = SearchModel(self)
        # Detail-page related media (from anilist/media-details).
        self._relations_model = SearchModel(self)
        self._recommendations_model = SearchModel(self)
        self._character_model = CharacterModel(self)
        # Manga models.
        self._manga_library_model = MangaLibraryModel(self)
        self._chapter_model = ChapterModel(self)
        self._page_model = PageModel(self)
        # Discover feed: one model per carousel.
        self._discover_model = SearchModel(self)   # trending
        self._season_model = SearchModel(self)
        self._prev_season_model = SearchModel(self)
        self._upcoming_model = SearchModel(self)
        self._movies_model = SearchModel(self)
        self._missed_sequels_model = SearchModel(self)

        # Client-side "find in library": a proxy that filters the library grid by
        # title as the user types. QML binds to the proxy, not the raw model.
        self._library_filter = QSortFilterProxyModel(self)
        self._library_filter.setSourceModel(self._library_model)
        self._library_filter.setFilterRole(LibraryModel.TitleRole)
        self._library_filter.setFilterCaseSensitivity(Qt.CaseSensitivity.CaseInsensitive)

        # "Split adult content": paired proxies that partition a source model into
        # its safe and adult halves. QML shows the split grids only when the server
        # enables ``anilist.splitAdultContent``; otherwise it binds the raw model.
        # The library proxies sit on top of the find-in-library filter so both the
        # text filter and the split compose.
        self._library_sfw = AdultFilterProxy(False, self)
        self._library_sfw.setSourceModel(self._library_filter)
        self._library_adult = AdultFilterProxy(True, self)
        self._library_adult.setSourceModel(self._library_filter)
        self._search_sfw = AdultFilterProxy(False, self)
        self._search_sfw.setSourceModel(self._search_model)
        self._search_adult = AdultFilterProxy(True, self)
        self._search_adult.setSourceModel(self._search_model)

        self._connection_status = "disconnected"
        self._error_message = ""
        self._detail_title = ""
        self._detail_synopsis = ""
        self._detail_poster = ""
        self._detail_banner = ""
        # Extended detail metadata (surfaced from the anime-entry payload).
        self._detail_media_id = 0
        self._detail_score = 0
        self._detail_status = ""
        self._detail_format = ""
        self._detail_season = ""
        self._detail_episode_count = 0
        self._detail_duration = 0
        self._detail_genres: list[str] = []
        self._detail_next_airing = ""
        # The user's AniList list state for this entry (for the editor + watched marks).
        self._detail_list_status = ""
        self._detail_list_score = 0
        self._detail_list_progress = 0
        # Advanced-search state: remember the active query to paginate it.
        self._search_filters: dict = {}
        self._search_page = 1

        # ---- manga state ----
        self._manga_title = ""
        self._manga_synopsis = ""
        self._manga_poster = ""
        self._manga_banner = ""
        self._manga_media_id = 0
        self._manga_score = 0
        self._manga_status = ""
        self._manga_format = ""
        self._manga_chapter_count = 0
        self._manga_genres: list[str] = []
        self._manga_list_status = ""
        self._manga_list_score = 0
        self._manga_list_progress = 0
        # Chapter source: the list of installed providers and the active one.
        self._manga_providers: list[dict] = []
        self._current_manga_provider = ""
        # Reader state.
        self._reader_chapter_title = ""
        self._reader_chapter_number = 0
        self._reader_loading = False

        # Client connection prefs, restored from disk (fall back to defaults / env).
        self._server_host = self._store.server_host("127.0.0.1")
        self._server_port = self._store.server_port("43211")
        self._server_token = self._store.server_token("")
        self._base_url = f"http://{self._server_host}:{self._server_port}"
        self._anilist_client_id = self._store.anilist_client_id(
            _DEFAULT_ANILIST_CLIENT_ID
        )
        self._anilist_client_secret = self._store.anilist_client_secret(
            _DEFAULT_ANILIST_CLIENT_SECRET
        )
        # The server's settings object, mirrored from the status/PATCH payloads.
        self._settings: dict = {}
        self._username = ""
        self._avatar_url = ""
        self._banner_url = ""
        self._library_count = 0
        self._capturing_token = False
        # Token caching / silent re-login.
        self._pending_token = ""       # token awaiting a login result, to cache on success
        self._silent_login = False     # current login attempt is a silent (cached) one
        self._silent_tried = False     # guard: at most one silent attempt per connect

        self._client.statusReceived.connect(self._on_status)
        self._client.libraryReceived.connect(self._on_library)
        self._client.animeEntryReceived.connect(self._on_anime_entry)
        self._client.mediaDetailsReceived.connect(self._on_media_details)
        self._client.searchReceived.connect(self._on_search)
        self._client.searchMoreReceived.connect(self._on_search_more)
        self._client.discoverReceived.connect(self._discover_model.load)
        self._client.seasonReceived.connect(self._season_model.load)
        self._client.prevSeasonReceived.connect(self._prev_season_model.load)
        self._client.upcomingReceived.connect(self._upcoming_model.load)
        self._client.moviesReceived.connect(self._movies_model.load)
        self._client.missedSequelsReceived.connect(self._on_missed_sequels)
        self._client.listEntryUpdated.connect(self._on_list_updated)
        self._client.progressUpdated.connect(self._on_progress_updated)
        self._client.mangaCollectionReceived.connect(self._on_manga_collection)
        self._client.mangaEntryReceived.connect(self._on_manga_entry)
        self._client.mangaProvidersReceived.connect(self._on_manga_providers)
        self._client.mangaChaptersReceived.connect(self._on_manga_chapters)
        self._client.mangaPagesReceived.connect(self._on_manga_pages)
        self._client.mangaProgressUpdated.connect(self._on_manga_progress_updated)
        self._client.settingsSaved.connect(self._on_settings_saved)
        self._client.anilistTokenObtained.connect(self._on_anilist_token)
        self._client.loginSucceeded.connect(self._on_login)
        self._client.loginFailed.connect(self._on_login_failed)
        self._client.errorOccurred.connect(self._on_error)

    # ---- properties exposed to QML --------------------------------------

    def _get_library_model(self) -> QObject:
        return self._library_filter

    def _get_episode_model(self) -> EpisodeModel:
        return self._episode_model

    def _get_search_model(self) -> SearchModel:
        return self._search_model

    def _get_search_sfw_model(self) -> QObject:
        return self._search_sfw

    def _get_search_adult_model(self) -> QObject:
        return self._search_adult

    def _get_library_sfw_model(self) -> QObject:
        return self._library_sfw

    def _get_library_adult_model(self) -> QObject:
        return self._library_adult

    def _get_discover_model(self) -> SearchModel:
        return self._discover_model

    def _get_relations_model(self) -> SearchModel:
        return self._relations_model

    def _get_recommendations_model(self) -> SearchModel:
        return self._recommendations_model

    def _get_character_model(self) -> CharacterModel:
        return self._character_model

    def _get_manga_library_model(self) -> MangaLibraryModel:
        return self._manga_library_model

    def _get_chapter_model(self) -> ChapterModel:
        return self._chapter_model

    def _get_page_model(self) -> PageModel:
        return self._page_model

    def _get_season_model(self) -> SearchModel:
        return self._season_model

    def _get_prev_season_model(self) -> SearchModel:
        return self._prev_season_model

    def _get_upcoming_model(self) -> SearchModel:
        return self._upcoming_model

    def _get_movies_model(self) -> SearchModel:
        return self._movies_model

    def _get_missed_sequels_model(self) -> SearchModel:
        return self._missed_sequels_model

    # QML binds the grid to the filter proxy so "find in library" is live.
    libraryModel = Property(QObject, _get_library_model, constant=True)
    episodeModel = Property(QObject, _get_episode_model, constant=True)
    searchModel = Property(QObject, _get_search_model, constant=True)
    searchSfwModel = Property(QObject, _get_search_sfw_model, constant=True)
    searchAdultModel = Property(QObject, _get_search_adult_model, constant=True)
    librarySfwModel = Property(QObject, _get_library_sfw_model, constant=True)
    libraryAdultModel = Property(QObject, _get_library_adult_model, constant=True)
    discoverModel = Property(QObject, _get_discover_model, constant=True)
    relationsModel = Property(QObject, _get_relations_model, constant=True)
    recommendationsModel = Property(QObject, _get_recommendations_model, constant=True)
    characterModel = Property(QObject, _get_character_model, constant=True)
    mangaLibraryModel = Property(QObject, _get_manga_library_model, constant=True)
    chapterModel = Property(QObject, _get_chapter_model, constant=True)
    pageModel = Property(QObject, _get_page_model, constant=True)
    seasonModel = Property(QObject, _get_season_model, constant=True)
    prevSeasonModel = Property(QObject, _get_prev_season_model, constant=True)
    upcomingModel = Property(QObject, _get_upcoming_model, constant=True)
    moviesModel = Property(QObject, _get_movies_model, constant=True)
    missedSequelsModel = Property(QObject, _get_missed_sequels_model, constant=True)

    def _get_connection_status(self) -> str:
        return self._connection_status

    connectionStatus = Property(
        str, _get_connection_status, notify=connectionStatusChanged
    )

    def _get_error_message(self) -> str:
        return self._error_message

    errorMessage = Property(str, _get_error_message, notify=errorMessageChanged)

    def _get_detail_title(self) -> str:
        return self._detail_title

    def _get_detail_synopsis(self) -> str:
        return self._detail_synopsis

    def _get_detail_poster(self) -> str:
        return self._detail_poster

    def _get_detail_banner(self) -> str:
        return self._detail_banner

    detailTitle = Property(str, _get_detail_title, notify=detailChanged)
    detailSynopsis = Property(str, _get_detail_synopsis, notify=detailChanged)
    detailPoster = Property(str, _get_detail_poster, notify=detailChanged)
    detailBanner = Property(str, _get_detail_banner, notify=detailChanged)

    def _get_detail_media_id(self) -> int:
        return self._detail_media_id

    def _get_detail_score(self) -> int:
        return self._detail_score

    def _get_detail_status(self) -> str:
        return self._detail_status

    def _get_detail_format(self) -> str:
        return self._detail_format

    def _get_detail_season(self) -> str:
        return self._detail_season

    def _get_detail_episode_count(self) -> int:
        return self._detail_episode_count

    def _get_detail_duration(self) -> int:
        return self._detail_duration

    def _get_detail_genres(self) -> list:
        return self._detail_genres

    def _get_detail_next_airing(self) -> str:
        return self._detail_next_airing

    def _get_detail_list_status(self) -> str:
        return self._detail_list_status

    def _get_detail_list_score(self) -> int:
        return self._detail_list_score

    def _get_detail_list_progress(self) -> int:
        return self._detail_list_progress

    detailMediaId = Property(int, _get_detail_media_id, notify=detailChanged)
    detailScore = Property(int, _get_detail_score, notify=detailChanged)
    detailStatus = Property(str, _get_detail_status, notify=detailChanged)
    detailFormat = Property(str, _get_detail_format, notify=detailChanged)
    detailSeason = Property(str, _get_detail_season, notify=detailChanged)
    detailEpisodeCount = Property(int, _get_detail_episode_count, notify=detailChanged)
    detailDuration = Property(int, _get_detail_duration, notify=detailChanged)
    detailGenres = Property("QVariantList", _get_detail_genres, notify=detailChanged)
    detailNextAiring = Property(str, _get_detail_next_airing, notify=detailChanged)
    detailListStatus = Property(str, _get_detail_list_status, notify=detailChanged)
    detailListScore = Property(int, _get_detail_list_score, notify=detailChanged)
    detailListProgress = Property(int, _get_detail_list_progress, notify=detailChanged)

    # ---- manga detail properties ----------------------------------------

    def _get_manga_title(self) -> str:
        return self._manga_title

    def _get_manga_synopsis(self) -> str:
        return self._manga_synopsis

    def _get_manga_poster(self) -> str:
        return self._manga_poster

    def _get_manga_banner(self) -> str:
        return self._manga_banner

    def _get_manga_media_id(self) -> int:
        return self._manga_media_id

    def _get_manga_score(self) -> int:
        return self._manga_score

    def _get_manga_status(self) -> str:
        return self._manga_status

    def _get_manga_format(self) -> str:
        return self._manga_format

    def _get_manga_chapter_count(self) -> int:
        return self._manga_chapter_count

    def _get_manga_genres(self) -> list:
        return self._manga_genres

    def _get_manga_list_status(self) -> str:
        return self._manga_list_status

    def _get_manga_list_score(self) -> int:
        return self._manga_list_score

    def _get_manga_list_progress(self) -> int:
        return self._manga_list_progress

    mangaTitle = Property(str, _get_manga_title, notify=mangaDetailChanged)
    mangaSynopsis = Property(str, _get_manga_synopsis, notify=mangaDetailChanged)
    mangaPoster = Property(str, _get_manga_poster, notify=mangaDetailChanged)
    mangaBanner = Property(str, _get_manga_banner, notify=mangaDetailChanged)
    mangaMediaId = Property(int, _get_manga_media_id, notify=mangaDetailChanged)
    mangaScore = Property(int, _get_manga_score, notify=mangaDetailChanged)
    mangaStatus = Property(str, _get_manga_status, notify=mangaDetailChanged)
    mangaFormat = Property(str, _get_manga_format, notify=mangaDetailChanged)
    mangaChapterCount = Property(int, _get_manga_chapter_count, notify=mangaDetailChanged)
    mangaGenres = Property("QVariantList", _get_manga_genres, notify=mangaDetailChanged)
    mangaListStatus = Property(str, _get_manga_list_status, notify=mangaDetailChanged)
    mangaListScore = Property(int, _get_manga_list_score, notify=mangaDetailChanged)
    mangaListProgress = Property(int, _get_manga_list_progress, notify=mangaDetailChanged)

    # ---- manga provider + reader properties -----------------------------

    def _get_manga_providers(self) -> list:
        return self._manga_providers

    def _get_current_manga_provider(self) -> str:
        return self._current_manga_provider

    def _get_reader_chapter_title(self) -> str:
        return self._reader_chapter_title

    def _get_reader_loading(self) -> bool:
        return self._reader_loading

    mangaProviders = Property(
        "QVariantList", _get_manga_providers, notify=mangaProvidersChanged
    )
    currentMangaProvider = Property(
        str, _get_current_manga_provider, notify=mangaProvidersChanged
    )
    readerChapterTitle = Property(
        str, _get_reader_chapter_title, notify=readerChanged
    )
    readerLoading = Property(bool, _get_reader_loading, notify=readerChanged)

    def _get_username(self) -> str:
        return self._username

    def _get_avatar_url(self) -> str:
        return self._avatar_url

    def _get_banner_url(self) -> str:
        return self._banner_url

    def _get_library_count(self) -> int:
        return self._library_count

    username = Property(str, _get_username, notify=authStateChanged)
    avatarUrl = Property(str, _get_avatar_url, notify=authStateChanged)
    bannerUrl = Property(str, _get_banner_url, notify=authStateChanged)
    libraryCount = Property(int, _get_library_count, notify=authStateChanged)

    def _get_anilist_client_id(self) -> str:
        return self._anilist_client_id

    def _set_anilist_client_id(self, value: str) -> None:
        value = (value or "").strip()
        if value and value != self._anilist_client_id:
            self._anilist_client_id = value
            self.authStateChanged.emit()

    anilistClientId = Property(
        str, _get_anilist_client_id, _set_anilist_client_id, notify=authStateChanged
    )

    def _get_anilist_client_secret(self) -> str:
        return self._anilist_client_secret

    def _set_anilist_client_secret(self, value: str) -> None:
        self._anilist_client_secret = (value or "").strip()

    anilistClientSecret = Property(
        str,
        _get_anilist_client_secret,
        _set_anilist_client_secret,
        notify=authStateChanged,
    )

    # ---- settings + client-pref properties ------------------------------

    def _get_settings(self) -> dict:
        return self._settings

    # The whole server-settings object as a plain map; QML reads
    # ``app.settings.<group>.<field>`` and overlays edits before saving.
    settings = Property("QVariant", _get_settings, notify=settingsChanged)

    # ---- adult-content flags (mirrored from settings.anilist) ------------
    # Broken out as first-class bools so QML can bind blur/split/toggle logic
    # without digging through the nested settings map (and so a null settings
    # object during startup reads as False rather than undefined).

    def _anilist_flag(self, key: str) -> bool:
        anilist = (self._settings or {}).get("anilist") or {}
        return bool(anilist.get(key))

    def _get_enable_adult(self) -> bool:
        return self._anilist_flag("enableAdultContent")

    def _get_blur_adult(self) -> bool:
        return self._anilist_flag("blurAdultContent")

    def _get_split_adult(self) -> bool:
        return self._anilist_flag("splitAdultContent")

    enableAdultContent = Property(bool, _get_enable_adult, notify=settingsChanged)
    blurAdultContent = Property(bool, _get_blur_adult, notify=settingsChanged)
    splitAdultContent = Property(bool, _get_split_adult, notify=settingsChanged)

    # AniList media-tag catalog for the advanced-search tag picker. Each entry is
    # ``{name, category, isAdult}``; QML groups by category and hides adult tags
    # unless the server enables adult content.
    def _get_media_tags(self) -> list:
        return [
            {"name": name, "category": category, "isAdult": is_adult}
            for (name, category, is_adult) in MEDIA_TAGS
        ]

    mediaTags = Property("QVariantList", _get_media_tags, constant=True)

    def _get_server_host(self) -> str:
        return self._server_host

    def _get_server_port(self) -> str:
        return self._server_port

    def _get_server_token(self) -> str:
        return self._server_token

    serverHost = Property(str, _get_server_host, notify=clientPrefsChanged)
    serverPort = Property(str, _get_server_port, notify=clientPrefsChanged)
    serverToken = Property(str, _get_server_token, notify=clientPrefsChanged)

    # ---- slots invoked from QML -----------------------------------------

    @Slot(str, str, str)
    def connectToServer(self, host: str, port: str, token: str) -> None:
        host = (host or "127.0.0.1").strip()
        port = (port or "43211").strip()
        token = (token or "").strip()
        # Remember the connection so it's restored on the next launch.
        self._server_host = host
        self._server_port = port
        self._server_token = token
        self._store.save_connection(host, port, token)
        self.clientPrefsChanged.emit()
        scheme = "http://" if "://" not in host else ""
        self._base_url = f"{scheme}{host}:{port}"
        self._client.set_base_url(self._base_url)
        self._client.set_token(token)
        self._set_error("")
        self._silent_tried = False  # allow one silent re-login for this connection
        self._set_connection_status("connecting")
        self._client.fetch_status()

    @Slot(str, str, str, str, str)
    def saveClientPrefs(
        self, host: str, port: str, token: str, client_id: str, client_secret: str
    ) -> None:
        """Persist the app-local prefs (connection + AniList credentials), then
        reconnect so the change takes effect immediately."""
        client_id = (client_id or "").strip() or self._anilist_client_id
        client_secret = (client_secret or "").strip()
        self._anilist_client_id = client_id
        self._anilist_client_secret = client_secret
        self._store.save_anilist_credentials(client_id, client_secret)
        self.authStateChanged.emit()
        # connectToServer persists the connection half and re-fetches status.
        self.connectToServer(host, port, token)

    @Slot(str, result=str)
    def urlToLocalPath(self, url: str) -> str:
        """Convert a file:// URL (from a QML file/folder dialog) to a native path."""
        return QUrl(url).toLocalFile()

    @Slot("QVariant")
    def saveServerSettings(self, payload) -> None:
        """PATCH the server settings from a QML-assembled 8-group object.

        QML must overlay its edits onto the current ``app.settings`` groups so no
        field is dropped — the server persists each group wholesale.
        """
        if isinstance(payload, QJSValue):
            payload = payload.toVariant()
        payload = dict(payload or {})
        if not payload:
            return
        self._set_error("")
        self._client.save_settings(payload)

    # ---- AniList login ---------------------------------------------------

    def _redirect_uri(self) -> str:
        """Callback URI, derived from the connected server so it matches the one
        registered on the AniList client (e.g. .../auth/callback on :43211)."""
        return f"{self._base_url}/auth/callback"

    @Slot(result=str)
    def anilistAuthorizeUrl(self) -> str:
        """Build the AniList authorization-code authorize URL for the login page."""
        self._capturing_token = False  # a fresh login attempt is starting
        url = QUrl(_ANILIST_AUTHORIZE_URL)
        query = QUrlQuery()
        query.addQueryItem("client_id", self._anilist_client_id)
        query.addQueryItem("response_type", "code")
        query.addQueryItem("redirect_uri", self._redirect_uri())
        url.setQuery(query)
        return url.toString()

    @Slot(str, result=bool)
    def handleCallback(self, url: str) -> bool:
        """Inspect a navigated URL for the AniList authorization code or an error.

        Returns True once the callback has been handled so QML can close the login
        page. The code arrives in the redirect query (``?code=...``); it is then
        exchanged for an access token (see _on_anilist_token).
        """
        if self._capturing_token:
            return True

        params = QUrlQuery(QUrl(url).query())
        code = params.queryItemValue("code")
        if code:
            self._capturing_token = True
            if not self._anilist_client_secret:
                self._set_error(
                    "AniList client secret not set — "
                    "set SEANIME_QT_ANILIST_CLIENT_SECRET and relaunch"
                )
                return True
            self._client.exchange_anilist_code(
                code,
                self._anilist_client_id,
                self._anilist_client_secret,
                self._redirect_uri(),
            )
            return True

        error = params.queryItemValue("error_description") or params.queryItemValue(
            "error"
        )
        if error:
            self._capturing_token = True
            self._set_error(f"AniList login failed: {error.replace('+', ' ')}")
            return True

        return False

    def _on_anilist_token(self, access_token: str) -> None:
        """Got the access token from the code exchange; authenticate to Seanime."""
        self._silent_login = False  # this is an interactive login
        self._pending_token = access_token
        self._client.login(access_token)

    def _on_login(self, data) -> None:
        # A login just succeeded, so the token is known-good — cache it for next time.
        if self._pending_token:
            self._token_cache.save(self._base_url, self._pending_token)
            self._pending_token = ""
        self._silent_login = False
        self._apply_user(data)
        self._set_error("")
        self._client.fetch_library()
        self.loginFinished.emit()

    def _on_login_failed(self, message: str) -> None:
        was_silent = self._silent_login
        self._silent_login = False
        self._pending_token = ""
        # The token was rejected (expired/revoked/invalid) — drop it from the cache.
        self._token_cache.clear(self._base_url)
        if not was_silent:
            # Surface only interactive failures; a silent re-login just falls back
            # to the unauthenticated state without nagging the user.
            self._on_error(message)

    # ---- silent re-login via the token cache ----------------------------

    def _maybe_silent_login(self) -> None:
        """Once per connection, try re-authenticating with a cached token."""
        if self._silent_tried:
            return
        self._silent_tried = True
        token = self._token_cache.load(self._base_url)
        if token:
            self._silent_login = True
            self._pending_token = token
            self._client.login(token)

    @Slot()
    def refresh(self) -> None:
        self._client.fetch_library()

    @Slot(str)
    def setLibraryFilter(self, text: str) -> None:
        """Live 'find in library' — filters the grid by title (client-side)."""
        self._library_filter.setFilterFixedString((text or "").strip())

    @Slot(str)
    def searchAnilist(self, term: str) -> None:
        """Simple title search (kept for the plain search box / back-compat)."""
        self.searchAdvanced({"search": term})

    @Slot("QVariant")
    def searchAdvanced(self, filters) -> None:
        """Run an advanced AniList search from a filter object.

        ``filters`` is a JS/dict of: search, sort, genres[], tags[], format,
        season, year, status, minScore, isAdult. Empty fields are omitted from
        the query.
        """
        # QML passes a JS object as a QJSValue; unwrap it to a plain dict.
        if isinstance(filters, QJSValue):
            filters = filters.toVariant()
        filters = dict(filters or {})
        search = (filters.get("search") or "").strip()
        # An empty query with no other filters clears the grid. ``isAdult`` counts
        # as a filter on its own so an adult-only browse (no title) still runs.
        meaningful = search or any(
            filters.get(k)
            for k in (
                "genres", "tags", "format", "season", "year", "status",
                "minScore", "isAdult",
            )
        )
        if not meaningful:
            self._search_filters = {}
            self._search_model.clear()
            return
        self._search_filters = filters
        self._search_page = 1
        self._set_error("")
        self._client.list_anime(search_body(filters, 1), self._client.searchReceived)

    @Slot()
    def searchLoadMore(self) -> None:
        """Fetch and append the next page of the current search."""
        if not self._search_filters:
            return
        self._search_page += 1
        self._client.list_anime(
            search_body(self._search_filters, self._search_page),
            self._client.searchMoreReceived,
        )

    @Slot()
    def loadDiscover(self) -> None:
        """Load every Discover carousel (trending, seasons, upcoming, movies…)."""
        self._set_error("")
        signals = {
            "trending": self._client.discoverReceived,
            "season": self._client.seasonReceived,
            "prev_season": self._client.prevSeasonReceived,
            "upcoming": self._client.upcomingReceived,
            "movies": self._client.moviesReceived,
        }
        for key, body in discover_bodies().items():
            self._client.list_anime(body, signals[key])
        self._client.fetch_missed_sequels()

    @Slot(int)
    def openAnime(self, media_id: int) -> None:
        self._episode_model.clear()
        self._relations_model.clear()
        self._recommendations_model.clear()
        self._character_model.clear()
        self._reset_detail()
        self._detail_media_id = media_id
        self._client.fetch_anime_entry(media_id)
        self._client.fetch_media_details(media_id)
        self.animeOpened.emit()

    @Slot(str, int, int)
    def saveListEntry(self, status: str, score: int, progress: int) -> None:
        """Persist a change to the user's AniList list entry for the open anime."""
        if not self._detail_media_id:
            return
        self._set_error("")
        self._client.edit_list_entry(
            self._detail_media_id, status, int(score), int(progress)
        )

    @Slot(int)
    def setEpisodeProgress(self, episode_number: int) -> None:
        """Mark watched up to ``episode_number`` for the open anime."""
        if not self._detail_media_id:
            return
        self._set_error("")
        self._client.update_progress(
            self._detail_media_id, int(episode_number), self._detail_episode_count
        )

    # ---- manga slots -----------------------------------------------------

    @Slot()
    def loadMangaLibrary(self) -> None:
        """Load the user's manga collection into the grid."""
        self._set_error("")
        self._client.fetch_manga_collection()

    @Slot(int)
    def openManga(self, media_id: int) -> None:
        """Open the manga detail page: fetch the entry and the provider list.

        Chapters are fetched once the provider list arrives (see
        ``_on_manga_providers``), which picks a default provider.
        """
        self._chapter_model.clear()
        self._page_model.clear()
        self._reset_manga_detail()
        self._manga_media_id = media_id
        self._set_error("")
        self._client.fetch_manga_entry(media_id)
        self._client.list_manga_providers()
        self.mangaOpened.emit()

    @Slot(str)
    def setMangaProvider(self, provider: str) -> None:
        """Switch the chapter source and re-fetch chapters for the open manga."""
        provider = (provider or "").strip()
        if not provider or provider == self._current_manga_provider:
            return
        self._current_manga_provider = provider
        self.mangaProvidersChanged.emit()
        self._fetch_chapters()

    @Slot(str, int, str)
    def openChapter(self, chapter_id: str, number: int, title: str) -> None:
        """Open the reader for a chapter: fetch its pages."""
        if not self._manga_media_id or not self._current_manga_provider:
            return
        self._page_model.clear()
        self._reader_chapter_title = title or f"Chapter {number}"
        self._reader_chapter_number = int(number)
        self._reader_loading = True
        self.readerChanged.emit()
        self._set_error("")
        self._client.fetch_manga_pages(
            self._manga_media_id, self._current_manga_provider, chapter_id
        )
        self.chapterOpened.emit()

    @Slot()
    def markCurrentChapterRead(self) -> None:
        """Mark the chapter currently open in the reader as read."""
        if not self._manga_media_id or self._reader_chapter_number <= 0:
            return
        if self._reader_chapter_number <= self._manga_list_progress:
            return  # already at/beyond this chapter
        self._set_error("")
        self._client.update_manga_progress(
            self._manga_media_id,
            self._reader_chapter_number,
            self._manga_chapter_count,
        )

    def _fetch_chapters(self) -> None:
        if self._manga_media_id and self._current_manga_provider:
            self._client.fetch_manga_chapters(
                self._manga_media_id, self._current_manga_provider
            )

    # ---- client signal handlers -----------------------------------------

    def _on_status(self, data) -> None:
        self._set_connection_status("connected")
        self._apply_user(data)
        self._apply_status_settings(data)
        self._set_error("")
        self._client.fetch_library()
        # If we have a cached AniList token, silently re-authenticate so the user
        # doesn't have to repeat the browser login every launch.
        self._maybe_silent_login()

    def _apply_status_settings(self, data) -> None:
        """Mirror the ``settings`` object out of a Status payload for the UI."""
        settings = (data or {}).get("settings")
        if isinstance(settings, dict):
            self._settings = settings
            self.settingsChanged.emit()

    def _on_settings_saved(self, data) -> None:
        """A settings PATCH succeeded; the reply is a fresh Status."""
        self._apply_status_settings(data)
        self._apply_user(data)
        self._set_error("")
        self._client.fetch_library()
        self.settingsSaved.emit()

    def _apply_user(self, data) -> None:
        """Extract the logged-in AniList profile from a Status payload."""
        data = data or {}
        user = data.get("user") or {}
        viewer = user.get("viewer") or {}
        simulated = bool(user.get("isSimulated"))
        name = "" if simulated else (viewer.get("name") or "")
        avatar = "" if simulated else ((viewer.get("avatar") or {}).get("large") or "")
        banner = "" if simulated else (viewer.get("bannerImage") or "")
        if (name, avatar, banner) != (
            self._username,
            self._avatar_url,
            self._banner_url,
        ):
            self._username = name
            self._avatar_url = avatar
            self._banner_url = banner
            self.authStateChanged.emit()

    def _on_library(self, data) -> None:
        self._library_model.load(data)
        self._library_count = self._library_model.rowCount()
        self.authStateChanged.emit()

    def _on_search(self, data) -> None:
        self._search_model.load(data)

    def _on_search_more(self, data) -> None:
        self._search_model.append_media_list(media_list_of(data))

    def _on_missed_sequels(self, data) -> None:
        self._missed_sequels_model.load_media_list(media_list_of(data))

    def _on_anime_entry(self, data) -> None:
        data = data or {}
        media = data.get("media") or {}
        title = media.get("title") or {}
        cover = media.get("coverImage") or {}
        list_data = data.get("listData") or {}

        self._detail_title = (
            title.get("userPreferred")
            or title.get("english")
            or title.get("romaji")
            or ""
        )
        self._detail_synopsis = strip_html(media.get("description") or "")
        self._detail_poster = cover.get("large") or cover.get("extraLarge") or ""
        self._detail_banner = media.get("bannerImage") or ""
        self._detail_media_id = media.get("id") or self._detail_media_id
        self._detail_score = media.get("meanScore") or 0
        self._detail_status = humanize(media.get("status") or "")
        self._detail_format = media.get("format") or ""
        season = media.get("season") or ""
        year = media.get("seasonYear") or ""
        self._detail_season = f"{humanize(season)} {year}".strip()
        self._detail_episode_count = media.get("episodes") or 0
        self._detail_duration = media.get("duration") or 0
        self._detail_genres = list(media.get("genres") or [])
        self._detail_next_airing = next_airing_text(media.get("nextAiringEpisode"))

        self._detail_list_status = list_data.get("status") or ""
        self._detail_list_score = list_data.get("score") or 0
        self._detail_list_progress = list_data.get("progress") or 0

        self.detailChanged.emit()
        self._episode_model.load(data.get("episodes"), self._detail_list_progress)

    def _on_media_details(self, data) -> None:
        data = data or {}
        # relations: { edges: [ { node: baseAnime } ] }
        relations = (data.get("relations") or {}).get("edges") or []
        self._relations_model.load_media_list(
            [(e or {}).get("node") for e in relations]
        )
        # recommendations: { edges: [ { node: { mediaRecommendation: media } } ] }
        recs = (data.get("recommendations") or {}).get("edges") or []
        self._recommendations_model.load_media_list(
            [((e or {}).get("node") or {}).get("mediaRecommendation") for e in recs]
        )
        self._character_model.load(data.get("characters"))

    def _on_list_updated(self, _data) -> None:
        # Re-fetch the entry so the header + episode watched marks reflect the change.
        if self._detail_media_id:
            self._client.fetch_anime_entry(self._detail_media_id)
        self._client.fetch_library()

    def _on_progress_updated(self, _data) -> None:
        if self._detail_media_id:
            self._client.fetch_anime_entry(self._detail_media_id)
        self._client.fetch_library()

    # ---- manga signal handlers ------------------------------------------

    def _on_manga_collection(self, data) -> None:
        self._manga_library_model.load(data)

    def _on_manga_entry(self, data) -> None:
        data = data or {}
        media = data.get("media") or {}
        title = media.get("title") or {}
        cover = media.get("coverImage") or {}
        list_data = data.get("listData") or {}

        self._manga_title = (
            title.get("userPreferred")
            or title.get("english")
            or title.get("romaji")
            or ""
        )
        self._manga_synopsis = strip_html(media.get("description") or "")
        self._manga_poster = cover.get("large") or cover.get("extraLarge") or ""
        self._manga_banner = media.get("bannerImage") or ""
        self._manga_media_id = media.get("id") or self._manga_media_id
        self._manga_score = media.get("meanScore") or 0
        self._manga_status = humanize(media.get("status") or "")
        self._manga_format = media.get("format") or ""
        self._manga_chapter_count = media.get("chapters") or 0
        self._manga_genres = list(media.get("genres") or [])

        self._manga_list_status = list_data.get("status") or ""
        self._manga_list_score = list_data.get("score") or 0
        self._manga_list_progress = list_data.get("progress") or 0

        self.mangaDetailChanged.emit()
        # Re-mark already-loaded chapters against the (possibly updated) progress.
        self._chapter_model.set_read_through(self._manga_list_progress)

    def _on_manga_providers(self, data) -> None:
        providers = [
            {"id": p.get("id") or "", "name": p.get("name") or p.get("id") or ""}
            for p in (data or [])
            if p and p.get("id")
        ]
        self._manga_providers = providers
        # Keep the current provider if still installed; otherwise default to the first.
        installed_ids = {p["id"] for p in providers}
        if self._current_manga_provider not in installed_ids:
            self._current_manga_provider = providers[0]["id"] if providers else ""
        self.mangaProvidersChanged.emit()
        self._fetch_chapters()

    def _on_manga_chapters(self, data) -> None:
        self._chapter_model.load(data, self._manga_list_progress)

    def _on_manga_pages(self, data) -> None:
        self._page_model.load(data, self._base_url)
        self._reader_loading = False
        self.readerChanged.emit()

    def _on_manga_progress_updated(self, _data) -> None:
        # Refresh the entry (updates list progress) and re-fetch chapters so the
        # read marks reflect the new progress.
        if self._manga_media_id:
            self._client.fetch_manga_entry(self._manga_media_id)
        self._client.fetch_manga_collection()

    def _on_error(self, message: str) -> None:
        if self._connection_status == "connecting":
            self._set_connection_status("disconnected")
        self._set_error(message)

    # ---- setters that emit change notifications -------------------------

    def _set_connection_status(self, value: str) -> None:
        if value != self._connection_status:
            self._connection_status = value
            self.connectionStatusChanged.emit()

    def _set_error(self, value: str) -> None:
        if value != self._error_message:
            self._error_message = value
            self.errorMessageChanged.emit()

    def _reset_detail(self) -> None:
        """Clear all detail state (called when a new anime is opened)."""
        self._detail_title = ""
        self._detail_synopsis = ""
        self._detail_poster = ""
        self._detail_banner = ""
        self._detail_media_id = 0
        self._detail_score = 0
        self._detail_status = ""
        self._detail_format = ""
        self._detail_season = ""
        self._detail_episode_count = 0
        self._detail_duration = 0
        self._detail_genres = []
        self._detail_next_airing = ""
        self._detail_list_status = ""
        self._detail_list_score = 0
        self._detail_list_progress = 0
        self.detailChanged.emit()

    def _reset_manga_detail(self) -> None:
        """Clear all manga detail state (called when a new manga is opened)."""
        self._manga_title = ""
        self._manga_synopsis = ""
        self._manga_poster = ""
        self._manga_banner = ""
        self._manga_media_id = 0
        self._manga_score = 0
        self._manga_status = ""
        self._manga_format = ""
        self._manga_chapter_count = 0
        self._manga_genres = []
        self._manga_list_status = ""
        self._manga_list_score = 0
        self._manga_list_progress = 0
        self.mangaDetailChanged.emit()
