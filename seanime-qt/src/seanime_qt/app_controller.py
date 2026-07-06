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

from .anilist_queries import (
    discover_bodies,
    humanize,
    media_list_of,
    next_airing_text,
    search_body,
    strip_html,
)
from .api_client import ApiClient
from .character_model import CharacterModel
from .episode_model import EpisodeModel
from .library_model import LibraryModel
from .search_model import SearchModel
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

    def __init__(self, parent: QObject | None = None) -> None:
        super().__init__(parent)

        self._client = ApiClient(self)
        self._token_cache = TokenCache()
        self._library_model = LibraryModel(self)
        self._episode_model = EpisodeModel(self)
        self._search_model = SearchModel(self)
        # Detail-page related media (from anilist/media-details).
        self._relations_model = SearchModel(self)
        self._recommendations_model = SearchModel(self)
        self._character_model = CharacterModel(self)
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

        self._base_url = "http://127.0.0.1:43211"
        self._anilist_client_id = _DEFAULT_ANILIST_CLIENT_ID
        self._anilist_client_secret = _DEFAULT_ANILIST_CLIENT_SECRET
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

    def _get_discover_model(self) -> SearchModel:
        return self._discover_model

    def _get_relations_model(self) -> SearchModel:
        return self._relations_model

    def _get_recommendations_model(self) -> SearchModel:
        return self._recommendations_model

    def _get_character_model(self) -> CharacterModel:
        return self._character_model

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
    discoverModel = Property(QObject, _get_discover_model, constant=True)
    relationsModel = Property(QObject, _get_relations_model, constant=True)
    recommendationsModel = Property(QObject, _get_recommendations_model, constant=True)
    characterModel = Property(QObject, _get_character_model, constant=True)
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
        str, _get_anilist_client_secret, _set_anilist_client_secret
    )

    # ---- slots invoked from QML -----------------------------------------

    @Slot(str, str, str)
    def connectToServer(self, host: str, port: str, token: str) -> None:
        host = (host or "127.0.0.1").strip()
        port = (port or "43211").strip()
        scheme = "http://" if "://" not in host else ""
        self._base_url = f"{scheme}{host}:{port}"
        self._client.set_base_url(self._base_url)
        self._client.set_token(token.strip())
        self._set_error("")
        self._silent_tried = False  # allow one silent re-login for this connection
        self._set_connection_status("connecting")
        self._client.fetch_status()

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

        ``filters`` is a JS/dict of: search, sort, genres[], format, season,
        year, status, minScore. Empty fields are omitted from the query.
        """
        # QML passes a JS object as a QJSValue; unwrap it to a plain dict.
        if isinstance(filters, QJSValue):
            filters = filters.toVariant()
        filters = dict(filters or {})
        search = (filters.get("search") or "").strip()
        # An empty query with no other filters clears the grid.
        meaningful = search or any(
            filters.get(k)
            for k in ("genres", "format", "season", "year", "status", "minScore")
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

    # ---- client signal handlers -----------------------------------------

    def _on_status(self, data) -> None:
        self._set_connection_status("connected")
        self._apply_user(data)
        self._set_error("")
        self._client.fetch_library()
        # If we have a cached AniList token, silently re-authenticate so the user
        # doesn't have to repeat the browser login every launch.
        self._maybe_silent_login()

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
