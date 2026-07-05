# Seanime-Qt Frontend — Proof of Concept Design

**Date:** 2026-07-05
**Status:** Approved for planning
**Scope:** Proof-of-concept Qt/QML desktop frontend for the Seanime Go backend, built in
Python (PySide6) but structured so a later port to C++ Qt is a mostly mechanical translation.

## Goal

Prove that a Qt+QML frontend can replace the current web frontend by consuming the existing
Go REST API. Deliver two working screens — a **library browser** and a **detail/episode
viewer** — using **only Qt libraries**, so the same design can later be ported to C++ Qt with
minimal rework.

Non-goals for the PoC: video playback (Seanime delegates to mpv/vlc), AniList OAuth login
flow, settings persistence, manga, torrents/streaming, and feature parity with the web app.

## Constraints

- **Qt-only dependencies.** Networking uses `QNetworkAccessManager` (not `requests`/`httpx`),
  JSON uses `QJsonDocument`/`QJsonValue` (not the Python `json` module), and lists use
  `QAbstractListModel`. Every construct must have a direct C++ Qt equivalent.
- **PySide6** binding (official Qt for Python). API names, signals/slots, and QML integration
  match C++ Qt almost 1:1.
- **uv** for Python project/dependency management.
- Assumes the Go server is already running and authenticated (offline or AniList account).

## Architecture (Approach A: PySide6 + QML + QtNetwork, model-based)

UI is declarative QML. Logic lives in `QObject` subclasses exposed to QML. Data is delivered
to views through `QAbstractListModel` subclasses using named roles. This mirrors idiomatic
C++ Qt so that:

- `.qml` files port to C++ **verbatim**.
- Each Python `QObject`/model class maps to a `.h/.cpp` pair with the same shape (properties,
  signals, slots, roles).

### Component boundaries

| Unit | Type | Responsibility | Depends on |
|------|------|----------------|------------|
| `ApiClient` | `QObject` | Wrap `QNetworkAccessManager`; build requests against a configurable base URL; attach optional `X-Seanime-Token`; parse replies with `QJsonDocument`; emit typed result signals and `errorOccurred(str)`. | QtNetwork, QtCore |
| `AppController` | `QObject` | App state/orchestration exposed to QML as context property `app`. Holds connection config (host, port, token), current screen, and connection status. Invokes `ApiClient`, feeds the models, exposes slots (`connectToServer`, `openAnime`, `goBack`). | `ApiClient`, models |
| `LibraryModel` | `QAbstractListModel` | Flat list of anime cards for the grid, with roles: `mediaId`, `title`, `posterUrl`, `status`, `progress`, `episodeCount`. | QtCore |
| `EpisodeModel` | `QAbstractListModel` | Episodes for the detail view, with roles: `number`, `title`, `thumbnailUrl`, `isDownloaded`, `progressWatched`. | QtCore |

### QML tree

| File | Responsibility |
|------|----------------|
| `Main.qml` | `ApplicationWindow` + `StackView`; top bar with connection status/banner. |
| `LibraryView.qml` | `GridView` of `AnimeCard`, section headers by watch status. |
| `AnimeCard.qml` | Poster `Image` + title; click → `app.openAnime(mediaId)`. |
| `DetailView.qml` | Poster, title, synopsis, and the episode `ListView`. |
| `EpisodeDelegate.qml` | One episode row (number, title, thumbnail). |

Posters/thumbnails load via QML `Image` pointing directly at API image URLs (or the backend
image-proxy), so no manual image-download code is needed.

## Data flow

1. QML action (e.g. card click) calls an `AppController` slot.
2. `AppController` calls an `ApiClient` method, which issues a `QNetworkAccessManager` request.
3. On reply, `ApiClient` parses JSON with `QJsonDocument` and emits a result signal.
4. `AppController` populates the relevant `QAbstractListModel`.
5. QML views update automatically through role bindings.

## API endpoints consumed

- `GET /api/v1/status` — connection check + auth/server state for the top bar.
- `GET /api/v1/library/collection` — library grouped by watch status → `LibraryModel`.
- `GET /api/v1/anime/episode-collection/:id` — episodes for the detail view → `EpisodeModel`
  (fallback: `GET /api/v1/library/anime-entry/:id` if the episode-collection shape is
  insufficient).

Base URL defaults to `http://127.0.0.1:43211`. Optional password token is sent via the
`X-Seanime-Token` header for servers configured with a password.

## Error handling

`ApiClient` emits `errorOccurred(str)` for network failures and non-2xx responses.
`AppController` exposes the latest error and connection status as QML properties; `Main.qml`
renders a dismissible banner. When the server is unreachable, the app shows the banner and an
empty state rather than crashing.

## Project layout

```
seanime-qt/
  pyproject.toml            # uv project; single runtime dep: PySide6
  README.md                 # how to run against a local server
  src/seanime_qt/
    __init__.py
    __main__.py             # QGuiApplication + QQmlApplicationEngine bootstrap
    api_client.py           # ApiClient(QObject)
    app_controller.py       # AppController(QObject)
    library_model.py        # LibraryModel(QAbstractListModel)
    episode_model.py        # EpisodeModel(QAbstractListModel)
  qml/
    Main.qml
    LibraryView.qml
    AnimeCard.qml
    DetailView.qml
    EpisodeDelegate.qml
```

## Verification

- `uv run seanime-qt` (or `uv run python -m seanime_qt`) launches the window.
- Against a live local Seanime server: the library grid populates with posters grouped by
  status; clicking a card opens the detail view with synopsis and a populated episode list.
- With the server stopped: the app launches and shows a connection-error banner + empty state.

## Migration path to C++ (informational)

- `qml/` copies over unchanged.
- Each Python class becomes a `.h/.cpp` with the same properties/signals/slots/roles.
- QtNetwork/QJsonDocument/QAbstractListModel calls translate line-for-line.
- `pyproject.toml`/uv is replaced by CMake + Qt; the QML registration pattern is the analog of
  `qmlRegisterType` / context properties.
