# Seanime-Qt (proof of concept)

A Qt/QML desktop frontend for the [Seanime](../README.md) Go backend, written in
Python with **PySide6** but structured so it can be ported to **C++ Qt** with a
mostly mechanical translation.

## What it does

- **Library browser** — a poster grid of your tracked library
  (`GET /api/v1/library/collection`).
- **Detail viewer** — click an anime to see its poster, synopsis, and episode
  list (`GET /api/v1/library/anime-entry/:id`). No video playback: Seanime
  delegates that to external players (mpv/vlc).

## Why it ports cleanly to C++

Only Qt libraries are used — no `requests`, `httpx`, or the Python `json` module:

| Concern      | Library used                    | C++ equivalent            |
|--------------|---------------------------------|---------------------------|
| Networking   | `QNetworkAccessManager`         | identical                 |
| JSON parsing | `QJsonDocument`                 | identical                 |
| List data    | `QAbstractListModel` + roles    | identical                 |
| UI           | QML files                       | **copied verbatim**       |
| Glue objects | `QObject` + `Property`/`Signal`/`Slot` | `.h`/`.cpp` with Q_PROPERTY / signals / Q_INVOKABLE |

## Requirements

- Python ≥ 3.9 and [uv](https://docs.astral.sh/uv/)
- A running Seanime server (default `http://127.0.0.1:43211`)

## Run

```bash
cd seanime-qt
uv sync
uv run seanime-qt
```

The app auto-connects to `127.0.0.1:43211` on startup. Use the fields in the top
bar to point at a different host/port, or to supply a password token (sent as the
`X-Seanime-Token` header) for a password-protected server.

## Project layout

```
seanime-qt/
  pyproject.toml
  src/seanime_qt/
    __main__.py       # QGuiApplication + QQmlApplicationEngine bootstrap
    api_client.py     # ApiClient(QObject)          — QNetworkAccessManager wrapper
    app_controller.py # AppController(QObject)       — state + orchestration ("app")
    library_model.py  # LibraryModel(QAbstractListModel)
    episode_model.py  # EpisodeModel(QAbstractListModel)
  qml/
    Main.qml          # window, connection bar, StackView
    LibraryView.qml   # poster GridView
    AnimeCard.qml     # grid delegate
    DetailView.qml    # poster + synopsis + episode ListView
    EpisodeDelegate.qml
```
