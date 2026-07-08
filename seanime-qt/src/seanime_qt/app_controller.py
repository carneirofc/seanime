"""AppController — orchestration object exposed to QML as ``app``.

Owns the ApiClient and the two list models, wires the client's signals into
model updates and observable state, and exposes slots the QML calls. Every
member here has a direct C++ Qt analog (Q_PROPERTY / Q_INVOKABLE / signals).
"""

from __future__ import annotations

import logging
import os
import re

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
from .destination import default_destination
from .chapter_model import ChapterModel
from .character_model import CharacterModel
from .episode_model import EpisodeModel
from .extension_model import ExtensionFilterProxy, ExtensionModel, count_by_type
from .library_model import LibraryModel
from .manga_library_model import MangaLibraryModel
from .manga_search_model import MangaSearchModel
from .media_tags import MEDIA_TAGS
from .page_model import PageModel
from .search_model import SearchModel
from .settings_store import SettingsStore
from .token_cache import TokenCache
from .torrent_model import TorrentModel

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

# UI appearance defaults + guards for the client-local prefs applied by the QML
# Theme. Scale/density are clamped to sane ranges; accent must be a #rrggbb hex.
_DEFAULT_UI_SCALE = 1.0
_DEFAULT_UI_DENSITY = 1.0
_DEFAULT_UI_THEME = "dark"
_DEFAULT_UI_ACCENT = "#6152df"
_DEFAULT_UI_POSTER_SCALE = 1.0
_UI_SCALE_RANGE = (0.8, 1.5)
_UI_DENSITY_RANGE = (0.85, 1.2)
_UI_POSTER_SCALE_RANGE = (0.7, 1.4)
# Client-local override for the server's "split adult content" setting:
# "server" follows it, "on"/"off" force the split on or off regardless.
_SPLIT_OVERRIDE_VALUES = ("server", "on", "off")
_DEFAULT_SPLIT_OVERRIDE = "server"
_ACCENT_HEX_RE = re.compile(r"^#[0-9a-fA-F]{6}$")


def _clamp(value: float, low: float, high: float) -> float:
    return max(low, min(high, value))


_log = logging.getLogger("seanime_qt.controller")


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
    uiPrefsChanged = Signal()       # client-local UI appearance prefs changed (font/theme…)
    # Fires when the effective "split adult content" changes — either the server
    # setting refreshed or the client-local override was edited. splitAdultContent
    # and splitAdultOverride both notify on it.
    adultSplitChanged = Signal()
    # Manga.
    mangaDetailChanged = Signal()
    mangaProvidersChanged = Signal()
    readerChanged = Signal()
    mangaOpened = Signal()    # QML pushes the manga detail page on this
    chapterOpened = Signal()  # QML pushes the reader page on this
    # Manual source-matching (mapping) dialog.
    mangaMappingChanged = Signal()  # mapping search results / current / busy flags
    mangaMappingOpened = Signal()   # QML opens the mapping dialog
    mangaMappingSaved = Signal()    # a mapping was saved; QML closes the dialog
    # Deep-link a genre/tag into the advanced-search page (e.g. tapping a chip on
    # the detail header). QML navigates to search, which consumes the pending
    # genre/tag and runs the query.
    genreSearchRequested = Signal(str)
    tagSearchRequested = Signal(str)
    detailTagsChanged = Signal()  # rich tags from media-details arrived
    searchStateChanged = Signal()  # advanced-search in-flight flag toggled
    # Torrent download.
    torrentSearchOpened = Signal()     # QML pushes the torrent search page
    torrentStateChanged = Signal()     # torrent loading / results / selection changed
    torrentDownloadReady = Signal()    # QML opens the download confirm dialog
    torrentDownloadStarted = Signal()  # a download was accepted; QML closes + returns
    # Extensions / providers.
    extensionsChanged = Signal()        # installed list / loading state changed
    marketplaceChanged = Signal()       # marketplace list / loading state changed
    extensionPreviewChanged = Signal()  # add-dialog fetch preview / busy flags changed
    extensionInstalled = Signal()       # an install succeeded; QML closes the dialog

    def __init__(self, parent: QObject | None = None) -> None:
        super().__init__(parent)

        self._client = ApiClient(self)
        self._token_cache = TokenCache()
        self._store = SettingsStore()
        self._library_model = LibraryModel(self)
        self._episode_model = EpisodeModel(self)
        self._search_model = SearchModel(self)
        self._search_busy = False  # True while an advanced search/page is in flight
        # Detail-page related media (from anilist/media-details).
        self._relations_model = SearchModel(self)
        self._recommendations_model = SearchModel(self)
        self._character_model = CharacterModel(self)
        self._torrent_model = TorrentModel(self)
        # Manga models.
        self._manga_library_model = MangaLibraryModel(self)
        self._chapter_model = ChapterModel(self)
        self._page_model = PageModel(self)
        # Manual source-matching (mapping) search results.
        self._manga_search_model = MangaSearchModel(self)
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
        # The manga library has no find-in-library text filter, so its split
        # proxies sit directly on the source collection model.
        self._manga_sfw = AdultFilterProxy(False, self)
        self._manga_sfw.setSourceModel(self._manga_library_model)
        self._manga_adult = AdultFilterProxy(True, self)
        self._manga_adult.setSourceModel(self._manga_library_model)

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
        # A genre/tag awaiting the search page (set by a detail-header chip tap).
        self._pending_search_genre = ""
        self._pending_search_tag = ""
        # Rich AniList tags for the open anime (from media-details).
        self._detail_tags: list = []
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

        # ---- extensions / providers state ----
        # Two ExtensionModels: the installed list and the marketplace catalogue.
        # The marketplace is exposed through a filter proxy (search + type).
        self._installed_ext_model = ExtensionModel(self)
        self._marketplace_model = ExtensionModel(self)
        self._marketplace_filter = ExtensionFilterProxy(self)
        self._marketplace_filter.setSourceModel(self._marketplace_model)
        self._marketplace_raw: list = []          # last raw marketplace payload
        self._installed_ext_ids: set = set()      # ids of installed extensions
        self._extensions_loading = False
        self._marketplace_loading = False
        self._extension_preview: dict = {}        # add-dialog fetch preview
        self._extension_fetching = False
        self._extension_installing = False

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
        # Manual source-matching (mapping) dialog state.
        self._manga_mapping_current = ""      # current mapped provider manga ID
        self._manga_mapping_searching = False  # a manual search is in flight
        self._manga_mapping_busy = False       # a set/remove mapping is in flight
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
        # Client-local UI appearance prefs, restored from disk. The QML Theme reads
        # these (via Main.qml) and reflows the whole UI when they change.
        self._ui_scale = _clamp(
            self._store.ui_scale(_DEFAULT_UI_SCALE), *_UI_SCALE_RANGE
        )
        self._ui_density = _clamp(
            self._store.ui_density(_DEFAULT_UI_DENSITY), *_UI_DENSITY_RANGE
        )
        self._ui_theme_mode = (
            "light" if self._store.ui_theme(_DEFAULT_UI_THEME) == "light" else "dark"
        )
        accent = self._store.ui_accent(_DEFAULT_UI_ACCENT)
        self._ui_accent = accent if _ACCENT_HEX_RE.match(accent) else _DEFAULT_UI_ACCENT
        self._ui_poster_scale = _clamp(
            self._store.ui_poster_scale(_DEFAULT_UI_POSTER_SCALE),
            *_UI_POSTER_SCALE_RANGE,
        )
        override = self._store.split_adult_override(_DEFAULT_SPLIT_OVERRIDE)
        self._split_adult_override = (
            override if override in _SPLIT_OVERRIDE_VALUES else _DEFAULT_SPLIT_OVERRIDE
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
        self._client.torrentSearchReceived.connect(self._on_torrent_search)
        self._client.torrentDownloadSucceeded.connect(self._on_torrent_download_ok)
        self._client.allExtensionsReceived.connect(self._on_all_extensions)
        self._client.marketplaceReceived.connect(self._on_marketplace)
        self._client.extensionFetched.connect(self._on_extension_fetched)
        self._client.extensionInstalled.connect(self._on_extension_installed)
        self._client.extensionUninstalled.connect(self._on_extension_uninstalled)
        self._client.extensionDisabledSet.connect(self._on_extension_disabled_set)
        self._client.extensionsReloaded.connect(self._on_extensions_reloaded)
        self._client.mangaCollectionReceived.connect(self._on_manga_collection)
        self._client.mangaEntryReceived.connect(self._on_manga_entry)
        self._client.mangaProvidersReceived.connect(self._on_manga_providers)
        self._client.mangaChaptersReceived.connect(self._on_manga_chapters)
        self._client.mangaPagesReceived.connect(self._on_manga_pages)
        self._client.mangaProgressUpdated.connect(self._on_manga_progress_updated)
        self._client.mangaSearchReceived.connect(self._on_manga_search)
        self._client.mangaMappingReceived.connect(self._on_manga_mapping)
        self._client.mangaMappingSet.connect(self._on_manga_mapping_set)
        self._client.mangaMappingRemoved.connect(self._on_manga_mapping_removed)
        self._client.settingsSaved.connect(self._on_settings_saved)
        # A server-settings refresh may flip splitAdultContent, so fan it out to
        # the combined signal the split property notifies on.
        self.settingsChanged.connect(self.adultSplitChanged)
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

    def _get_search_busy(self) -> bool:
        return self._search_busy

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

    def _get_torrent_model(self) -> QObject:
        return self._torrent_model

    def _get_relations_model(self) -> SearchModel:
        return self._relations_model

    def _get_recommendations_model(self) -> SearchModel:
        return self._recommendations_model

    def _get_character_model(self) -> CharacterModel:
        return self._character_model

    def _get_manga_library_model(self) -> MangaLibraryModel:
        return self._manga_library_model

    def _get_manga_library_sfw_model(self) -> QObject:
        return self._manga_sfw

    def _get_manga_library_adult_model(self) -> QObject:
        return self._manga_adult

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
    searchBusy = Property(bool, _get_search_busy, notify=searchStateChanged)
    searchSfwModel = Property(QObject, _get_search_sfw_model, constant=True)
    searchAdultModel = Property(QObject, _get_search_adult_model, constant=True)
    librarySfwModel = Property(QObject, _get_library_sfw_model, constant=True)
    libraryAdultModel = Property(QObject, _get_library_adult_model, constant=True)
    discoverModel = Property(QObject, _get_discover_model, constant=True)
    relationsModel = Property(QObject, _get_relations_model, constant=True)
    recommendationsModel = Property(QObject, _get_recommendations_model, constant=True)
    characterModel = Property(QObject, _get_character_model, constant=True)
    torrentModel = Property(QObject, _get_torrent_model, constant=True)
    mangaLibraryModel = Property(QObject, _get_manga_library_model, constant=True)
    mangaLibrarySfwModel = Property(
        QObject, _get_manga_library_sfw_model, constant=True
    )
    mangaLibraryAdultModel = Property(
        QObject, _get_manga_library_adult_model, constant=True
    )
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

    def _get_detail_tags(self) -> list:
        return self._detail_tags

    # Rich tags: ``{name, rank, isAdult, spoiler}``, ranked high→low. Populated
    # from media-details (a different request than the base entry), so it has its
    # own change signal.
    detailTags = Property("QVariantList", _get_detail_tags, notify=detailTagsChanged)
    detailNextAiring = Property(str, _get_detail_next_airing, notify=detailChanged)
    detailListStatus = Property(str, _get_detail_list_status, notify=detailChanged)
    detailListScore = Property(int, _get_detail_list_score, notify=detailChanged)
    detailListProgress = Property(int, _get_detail_list_progress, notify=detailChanged)

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

    # ---- extensions / providers properties ------------------------------

    def _get_installed_extension_model(self) -> QObject:
        return self._installed_ext_model

    def _get_marketplace_extension_model(self) -> QObject:
        return self._marketplace_filter

    def _get_extensions_loading(self) -> bool:
        return self._extensions_loading

    def _get_marketplace_loading(self) -> bool:
        return self._marketplace_loading

    def _get_extension_preview(self) -> dict:
        return self._extension_preview

    def _get_extension_fetching(self) -> bool:
        return self._extension_fetching

    def _get_extension_installing(self) -> bool:
        return self._extension_installing

    installedExtensionModel = Property(
        QObject, _get_installed_extension_model, constant=True
    )
    marketplaceExtensionModel = Property(
        QObject, _get_marketplace_extension_model, constant=True
    )
    extensionsLoading = Property(
        bool, _get_extensions_loading, notify=extensionsChanged
    )
    marketplaceLoading = Property(
        bool, _get_marketplace_loading, notify=marketplaceChanged
    )
    # The previewed extension ``{id,name,version,author,description,...}`` fetched
    # by the add-dialog's "Find" action, or ``{}`` when there is none.
    extensionPreview = Property(
        "QVariant", _get_extension_preview, notify=extensionPreviewChanged
    )
    extensionFetching = Property(
        bool, _get_extension_fetching, notify=extensionPreviewChanged
    )
    extensionInstalling = Property(
        bool, _get_extension_installing, notify=extensionPreviewChanged
    )

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

    # ---- manga mapping (manual source match) properties -----------------

    def _get_manga_search_model(self) -> QObject:
        return self._manga_search_model

    def _get_manga_mapping_current(self) -> str:
        return self._manga_mapping_current

    def _get_manga_mapping_searching(self) -> bool:
        return self._manga_mapping_searching

    def _get_manga_mapping_busy(self) -> bool:
        return self._manga_mapping_busy

    mangaSearchModel = Property(QObject, _get_manga_search_model, constant=True)
    # The current mapped provider manga ID, or "" when there's no manual match.
    mangaMappingCurrent = Property(
        str, _get_manga_mapping_current, notify=mangaMappingChanged
    )
    mangaMappingSearching = Property(
        bool, _get_manga_mapping_searching, notify=mangaMappingChanged
    )
    mangaMappingBusy = Property(
        bool, _get_manga_mapping_busy, notify=mangaMappingChanged
    )

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
        # Client-local override wins over the server setting when set.
        if self._split_adult_override == "on":
            return True
        if self._split_adult_override == "off":
            return False
        return self._anilist_flag("splitAdultContent")

    def _get_split_adult_override(self) -> str:
        return self._split_adult_override

    enableAdultContent = Property(bool, _get_enable_adult, notify=settingsChanged)
    blurAdultContent = Property(bool, _get_blur_adult, notify=settingsChanged)
    # Notifies on adultSplitChanged (fanned out from settingsChanged) so both a
    # server refresh and a client override edit update bindings.
    splitAdultContent = Property(bool, _get_split_adult, notify=adultSplitChanged)
    # "server" | "on" | "off" — the client-local override.
    splitAdultOverride = Property(
        str, _get_split_adult_override, notify=adultSplitChanged
    )

    @Slot(str)
    def setSplitAdultOverride(self, value: str) -> None:
        """Set the client-local split override and persist it (no-op if unchanged)."""
        value = value if value in _SPLIT_OVERRIDE_VALUES else _DEFAULT_SPLIT_OVERRIDE
        if value == self._split_adult_override:
            return
        self._split_adult_override = value
        self._store.save_split_adult_override(value)
        self.adultSplitChanged.emit()

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

    # ---- UI appearance prefs (client-local; applied live by the QML Theme) ----

    def _get_ui_scale(self) -> float:
        return self._ui_scale

    def _get_ui_density(self) -> float:
        return self._ui_density

    def _get_ui_theme_mode(self) -> str:
        return self._ui_theme_mode

    def _get_ui_accent(self) -> str:
        return self._ui_accent

    def _get_ui_poster_scale(self) -> float:
        return self._ui_poster_scale

    uiScale = Property(float, _get_ui_scale, notify=uiPrefsChanged)
    uiDensity = Property(float, _get_ui_density, notify=uiPrefsChanged)
    uiThemeMode = Property(str, _get_ui_theme_mode, notify=uiPrefsChanged)
    uiAccent = Property(str, _get_ui_accent, notify=uiPrefsChanged)
    uiPosterScale = Property(float, _get_ui_poster_scale, notify=uiPrefsChanged)

    # ---- slots invoked from QML -----------------------------------------

    @Slot(float)
    def setUiScale(self, value: float) -> None:
        self._apply_ui_prefs(scale=_clamp(float(value), *_UI_SCALE_RANGE))

    @Slot(float)
    def setUiDensity(self, value: float) -> None:
        self._apply_ui_prefs(density=_clamp(float(value), *_UI_DENSITY_RANGE))

    @Slot(str)
    def setUiThemeMode(self, value: str) -> None:
        self._apply_ui_prefs(theme="light" if value == "light" else "dark")

    @Slot(str)
    def setUiAccent(self, value: str) -> None:
        value = (value or "").strip()
        if _ACCENT_HEX_RE.match(value):
            self._apply_ui_prefs(accent=value.lower())

    @Slot(float)
    def setUiPosterScale(self, value: float) -> None:
        self._apply_ui_prefs(
            poster_scale=_clamp(float(value), *_UI_POSTER_SCALE_RANGE)
        )

    @Slot()
    def resetUiPrefs(self) -> None:
        """Restore the shipped defaults (dark, 100% scale, comfortable, indigo)."""
        self._apply_ui_prefs(
            scale=_DEFAULT_UI_SCALE,
            density=_DEFAULT_UI_DENSITY,
            theme=_DEFAULT_UI_THEME,
            accent=_DEFAULT_UI_ACCENT,
            poster_scale=_DEFAULT_UI_POSTER_SCALE,
        )

    def _apply_ui_prefs(
        self,
        scale: float | None = None,
        density: float | None = None,
        theme: str | None = None,
        accent: str | None = None,
        poster_scale: float | None = None,
    ) -> None:
        """Update whichever prefs were supplied, persist them all, and notify QML.

        A no-op guard keeps the app quiet when nothing actually changed (e.g. a
        combo re-selecting its current value), so the Theme doesn't reflow for
        free."""
        changed = False
        if scale is not None and scale != self._ui_scale:
            self._ui_scale = scale
            changed = True
        if density is not None and density != self._ui_density:
            self._ui_density = density
            changed = True
        if theme is not None and theme != self._ui_theme_mode:
            self._ui_theme_mode = theme
            changed = True
        if accent is not None and accent != self._ui_accent:
            self._ui_accent = accent
            changed = True
        if poster_scale is not None and poster_scale != self._ui_poster_scale:
            self._ui_poster_scale = poster_scale
            changed = True
        if not changed:
            return
        self._store.save_ui_prefs(
            self._ui_scale,
            self._ui_density,
            self._ui_theme_mode,
            self._ui_accent,
            self._ui_poster_scale,
        )
        self.uiPrefsChanged.emit()

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

    @Slot(str)
    def requestGenreSearch(self, genre: str) -> None:
        """Deep-link a genre into the search page (from a detail-header chip)."""
        genre = (genre or "").strip()
        if not genre:
            return
        self._pending_search_genre = genre
        self.genreSearchRequested.emit(genre)

    @Slot(result=str)
    def consumePendingSearchGenre(self) -> str:
        """Return and clear the genre queued by ``requestGenreSearch`` (the search
        page calls this on load to seed and run the query)."""
        genre = self._pending_search_genre
        self._pending_search_genre = ""
        return genre

    @Slot(str)
    def requestTagSearch(self, tag: str) -> None:
        """Deep-link a tag into the search page (from a detail-header chip)."""
        tag = (tag or "").strip()
        if not tag:
            return
        self._pending_search_tag = tag
        self.tagSearchRequested.emit(tag)

    @Slot(result=str)
    def consumePendingSearchTag(self) -> str:
        """Return and clear the tag queued by ``requestTagSearch``."""
        tag = self._pending_search_tag
        self._pending_search_tag = ""
        return tag

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
            self._set_search_busy(False)
            return
        self._search_filters = filters
        self._search_page = 1
        self._set_error("")
        self._set_search_busy(True)
        self._client.list_anime(search_body(filters, 1), self._client.searchReceived)

    @Slot()
    def searchLoadMore(self) -> None:
        """Fetch and append the next page of the current search."""
        if not self._search_filters:
            return
        self._search_page += 1
        self._set_search_busy(True)
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

    # ---- extensions / providers slots -----------------------------------

    @Slot()
    def loadExtensions(self) -> None:
        """Fetch the installed extensions (enabled + disabled + invalid)."""
        _log.info("Loading installed extensions...")
        self._extensions_loading = True
        self._set_error("")
        self.extensionsChanged.emit()
        self._client.fetch_all_extensions()

    @Slot()
    def loadMarketplace(self) -> None:
        """Fetch the marketplace extension catalogue."""
        _log.info("Loading marketplace catalogue...")
        self._marketplace_loading = True
        self._set_error("")
        self.marketplaceChanged.emit()
        self._client.fetch_marketplace()

    @Slot(str)
    def setMarketplaceSearch(self, text: str) -> None:
        self._marketplace_filter.setSearchText(text)

    @Slot(str)
    def setMarketplaceType(self, ext_type: str) -> None:
        self._marketplace_filter.setTypeFilter(ext_type)

    @Slot(str)
    def fetchExtensionPreview(self, manifest_uri: str) -> None:
        """Preview the extension a manifest URI describes (add-dialog "Find")."""
        uri = (manifest_uri or "").strip()
        if not uri:
            return
        self._extension_preview = {}
        self._extension_fetching = True
        self._set_error("")
        self.extensionPreviewChanged.emit()
        self._client.fetch_external_extension(uri)

    @Slot(str)
    def installExtension(self, manifest_uri: str) -> None:
        """Install (or update) the extension at ``manifest_uri``."""
        uri = (manifest_uri or "").strip()
        if not uri:
            return
        _log.info("Installing extension from %s", uri)
        self._extension_installing = True
        self._set_error("")
        self.extensionPreviewChanged.emit()
        self._client.install_external_extension(uri)

    @Slot(str)
    def uninstallExtension(self, extension_id: str) -> None:
        extension_id = (extension_id or "").strip()
        if not extension_id:
            return
        self._set_error("")
        self._client.uninstall_external_extension(extension_id)

    @Slot(str, bool)
    def setExtensionDisabled(self, extension_id: str, disabled: bool) -> None:
        extension_id = (extension_id or "").strip()
        if not extension_id:
            return
        self._set_error("")
        self._client.set_extension_disabled(extension_id, bool(disabled))

    @Slot()
    def reloadExtensions(self) -> None:
        self._set_error("")
        self._client.reload_external_extensions()

    @Slot()
    def clearExtensionPreview(self) -> None:
        """Reset the add-dialog preview (called when the dialog closes)."""
        self._extension_preview = {}
        self._extension_fetching = False
        self._extension_installing = False
        self.extensionPreviewChanged.emit()

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

    @Slot()
    def openMangaMapping(self) -> None:
        """Open the manual source-match dialog and fetch the current mapping.

        Requires an open manga and an active provider (mapping is per-provider).
        """
        if not self._manga_media_id or not self._current_manga_provider:
            return
        self._manga_search_model.clear()
        self._manga_mapping_current = ""
        self._manga_mapping_searching = False
        self._manga_mapping_busy = False
        self._set_error("")
        self.mangaMappingChanged.emit()
        self._client.get_manga_mapping(
            self._current_manga_provider, self._manga_media_id
        )
        self.mangaMappingOpened.emit()

    @Slot(str)
    def runMangaMappingSearch(self, query: str) -> None:
        """Search the active provider for candidates to map this manga to."""
        query = (query or "").strip()
        if not query or not self._manga_media_id or not self._current_manga_provider:
            return
        self._manga_mapping_searching = True
        self._set_error("")
        self.mangaMappingChanged.emit()
        self._client.manga_manual_search(self._current_manga_provider, query)

    @Slot(str)
    def confirmMangaMapping(self, manga_id: str) -> None:
        """Map the open manga to the chosen provider manga ID."""
        manga_id = (manga_id or "").strip()
        if not manga_id or not self._manga_media_id or not self._current_manga_provider:
            return
        self._manga_mapping_busy = True
        self._set_error("")
        self.mangaMappingChanged.emit()
        self._client.set_manga_mapping(
            self._current_manga_provider, self._manga_media_id, manga_id
        )

    @Slot()
    def removeMangaMapping(self) -> None:
        """Remove the manual mapping for the open manga (revert to automatic)."""
        if not self._manga_media_id or not self._current_manga_provider:
            return
        self._manga_mapping_busy = True
        self._set_error("")
        self.mangaMappingChanged.emit()
        self._client.remove_manga_mapping(
            self._current_manga_provider, self._manga_media_id
        )

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
        self._set_search_busy(False)

    def _on_search_more(self, data) -> None:
        self._search_model.append_media_list(media_list_of(data))
        self._set_search_busy(False)

    def _on_missed_sequels(self, data) -> None:
        self._missed_sequels_model.load_media_list(media_list_of(data))

    def _on_anime_entry(self, data) -> None:
        data = data or {}
        media = data.get("media") or {}
        self._entry_media = media
        self._entry_local_files = data.get("localFiles") or []
        self._entry_download_info = data.get("downloadInfo") or {}
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
        self._apply_detail_tags(data.get("tags"))

    def _apply_detail_tags(self, raw_tags) -> None:
        """Build the ranked tag chip list, honouring the spoiler / adult settings."""
        hide_spoilers = self._anilist_flag("hideMediaTagsSpoilers")
        allow_adult = self._anilist_flag("enableAdultContent")
        tags: list[dict] = []
        for tag in raw_tags or []:
            if not tag or not tag.get("name"):
                continue
            is_adult = bool(tag.get("isAdult"))
            if is_adult and not allow_adult:
                continue
            spoiler = bool(tag.get("isMediaSpoiler") or tag.get("isGeneralSpoiler"))
            if spoiler and hide_spoilers:
                continue
            tags.append(
                {
                    "name": tag.get("name") or "",
                    "rank": int(tag.get("rank") or 0),
                    "isAdult": is_adult,
                    "spoiler": spoiler,
                }
            )
        # Highest-ranked first; cap so the header stays readable.
        tags.sort(key=lambda t: t["rank"], reverse=True)
        self._detail_tags = tags[:18]
        self.detailTagsChanged.emit()

    def _on_list_updated(self, _data) -> None:
        # Re-fetch the entry so the header + episode watched marks reflect the change.
        if self._detail_media_id:
            self._client.fetch_anime_entry(self._detail_media_id)
        self._client.fetch_library()

    def _on_progress_updated(self, _data) -> None:
        if self._detail_media_id:
            self._client.fetch_anime_entry(self._detail_media_id)
        self._client.fetch_library()

    def _on_torrent_search(self, data) -> None:
        self._torrent_model.load(data)
        self._torrent_search_loading = False
        self.torrentStateChanged.emit()

    def _on_torrent_download_ok(self, _data) -> None:
        self._torrent_downloading = False
        self.torrentStateChanged.emit()
        self.torrentDownloadStarted.emit()

    # ---- extensions / providers handlers --------------------------------

    def _on_all_extensions(self, data) -> None:
        self._installed_ext_model.load_installed(data)
        d = data if isinstance(data, dict) else {}
        enabled = d.get("extensions") or []
        disabled = d.get("disabledExtensions") or []
        invalid = d.get("invalidExtensions") or []
        by_type = count_by_type(enabled)
        _log.info(
            "Installed extensions: %d enabled %s, %d disabled, %d invalid",
            len(enabled),
            dict(by_type),
            len(disabled),
            len(invalid),
        )
        if not by_type.get("manga-provider") and not by_type.get("anime-torrent-provider"):
            _log.warning(
                "No manga/torrent PROVIDER extensions installed "
                "(only %s). Install providers from the Marketplace tab or via a "
                "manifest URL.",
                ", ".join(sorted(by_type)) or "none",
            )
        ids: set = set()
        for key in ("extensions", "disabledExtensions"):
            for ext in d.get(key) or []:
                if ext and ext.get("id"):
                    ids.add(ext.get("id"))
        for inv in d.get("invalidExtensions") or []:
            ext = (inv or {}).get("extension") or {}
            if ext.get("id"):
                ids.add(ext.get("id"))
        self._installed_ext_ids = ids
        self._extensions_loading = False
        self.extensionsChanged.emit()
        # Re-mark the marketplace now that we know what's installed.
        self._refresh_marketplace_installed()

    def _on_marketplace(self, data) -> None:
        self._marketplace_raw = list(data or [])
        by_type = count_by_type(self._marketplace_raw)
        _log.info(
            "Marketplace catalogue: %d extensions %s",
            len(self._marketplace_raw),
            dict(by_type),
        )
        providers = by_type.get("manga-provider", 0) + by_type.get("anime-torrent-provider", 0)
        if providers == 0:
            _log.warning(
                "Marketplace has 0 manga/torrent providers - the default "
                "repository does not distribute them. Add a provider extension "
                "by its manifest URL via 'Add extension'."
            )
        self._marketplace_loading = False
        self._refresh_marketplace_installed()
        self.marketplaceChanged.emit()

    def _refresh_marketplace_installed(self) -> None:
        self._marketplace_model.load_marketplace(
            self._marketplace_raw, self._installed_ext_ids
        )

    def _on_extension_fetched(self, data) -> None:
        self._extension_preview = data if isinstance(data, dict) else {}
        self._extension_fetching = False
        self.extensionPreviewChanged.emit()

    def _on_extension_installed(self, _data) -> None:
        _log.info("Extension installed; refreshing installed list")
        self._extension_installing = False
        self._extension_preview = {}
        self.extensionPreviewChanged.emit()
        self.extensionInstalled.emit()
        # Refresh the installed list (which also re-marks the marketplace).
        self._client.fetch_all_extensions()

    def _on_extension_uninstalled(self, _data) -> None:
        self._client.fetch_all_extensions()

    def _on_extension_disabled_set(self, _data) -> None:
        self._client.fetch_all_extensions()

    def _on_extensions_reloaded(self, _data) -> None:
        self._client.fetch_all_extensions()

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

    def _on_manga_search(self, data) -> None:
        self._manga_search_model.load(data, self._base_url)
        self._manga_mapping_searching = False
        self.mangaMappingChanged.emit()

    def _on_manga_mapping(self, data) -> None:
        manga_id = data.get("mangaId") if isinstance(data, dict) else None
        self._manga_mapping_current = manga_id or ""
        self.mangaMappingChanged.emit()

    def _on_manga_mapping_set(self, _data) -> None:
        # The mapping changed, so the cached chapter container is stale — re-fetch
        # chapters for the active provider, then dismiss the dialog.
        self._manga_mapping_busy = False
        self.mangaMappingChanged.emit()
        self._fetch_chapters()
        self.mangaMappingSaved.emit()

    def _on_manga_mapping_removed(self, _data) -> None:
        # Mapping cleared: reflect it and re-fetch chapters (automatic matching).
        self._manga_mapping_busy = False
        self._manga_mapping_current = ""
        self.mangaMappingChanged.emit()
        self._fetch_chapters()

    def _on_error(self, message: str) -> None:
        if self._connection_status == "connecting":
            self._set_connection_status("disconnected")
        # A failed torrent search/download should release the busy flags so the
        # UI doesn't stay stuck in a loading state.
        if self._torrent_search_loading or self._torrent_downloading:
            self._torrent_search_loading = False
            self._torrent_downloading = False
            self.torrentStateChanged.emit()
        # A failed advanced search should release its busy flag too.
        self._set_search_busy(False)
        # Likewise release any extension busy flags so the UI doesn't stay stuck.
        if self._extension_fetching or self._extension_installing:
            self._extension_fetching = False
            self._extension_installing = False
            self.extensionPreviewChanged.emit()
        if self._extensions_loading or self._marketplace_loading:
            self._extensions_loading = False
            self._marketplace_loading = False
            self.extensionsChanged.emit()
            self.marketplaceChanged.emit()
        # Release any mapping busy flags so the dialog doesn't stay stuck loading.
        if self._manga_mapping_searching or self._manga_mapping_busy:
            self._manga_mapping_searching = False
            self._manga_mapping_busy = False
            self.mangaMappingChanged.emit()
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

    def _set_search_busy(self, value: bool) -> None:
        if value != self._search_busy:
            self._search_busy = value
            self.searchStateChanged.emit()

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
        self._detail_tags = []
        self.detailTagsChanged.emit()
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
        # Clear any lingering mapping-dialog state from a previous manga.
        self._manga_search_model.clear()
        self._manga_mapping_current = ""
        self._manga_mapping_searching = False
        self._manga_mapping_busy = False
        self.mangaMappingChanged.emit()
        self.mangaDetailChanged.emit()
