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
  src/seanime_qt/agent/  # dev-only: agent control harness (see below)
    control_server.py    #   in-app JSON/TCP command socket (GUI thread)
    log_capture.py       #   unified log ring buffer
    mcp_server.py        #   MCP server: lifecycle + control tools
```

## Agentic control (Claude Code)

The app ships an **opt-in harness** that lets Claude Code launch, inspect, and
drive it — no human at the mouse — for agentic development and debugging.

**How it fits together**

```
Claude Code ──stdio──▶ MCP server ──JSON/TCP──▶ Control server (inside the app)
  (tool calls)        seanime-qt-mcp   127.0.0.1:43299    runs on the GUI thread
```

- The **control server** is embedded in the Qt process and enabled only when
  `SEANIME_QT_AGENT=1`. Because it lives on the GUI thread it can safely walk the
  QML tree, grab the window, and synthesise input events. It binds to localhost
  only and speaks newline-delimited JSON.
- The **MCP server** (`seanime-qt-mcp`) runs as a separate process, owns the app
  subprocess (`app_start`/`app_stop`/`app_restart`), and forwards the rest of the
  tools over the socket.

**Setup.** The repo root has a `.mcp.json` registering the server, so in Claude
Code you just approve the `seanime-qt` MCP server (install the extra once with
`uv sync --extra agent`). Then, from a Claude Code session:

1. `app_start` — launches the app with the control server on.
2. `screenshot` / `dump_tree` — see what's on screen (pixels, or a JSON tree of
   objectNames, geometry, and text).
3. `click` / `type_text` — interact by `objectName` (e.g. `nav_discover`,
   `searchField`, `connectButton`, or a poster `card_<mediaId>`).
4. `press_key` / `active_focus` — drive and inspect keyboard behaviour: send
   `tab`/`return`/arrow keys and read which item holds focus (Tab order,
   Enter/Space activation, arrow-key grid navigation).
5. `accessible` — read an element's accessibility interface (role/name/
   description) as an assistive tool would see it.
6. `invoke` — drive state deterministically via the controller's slots, e.g.
   `invoke("app", "openAnime", [21])`, `invoke("app", "searchAnilist", ["naruto"])`.
7. `get_logs` — QML warnings, Python logs, and uncaught exceptions from a shared
   ring buffer (pass the returned `cursor` back as `since` to tail only new ones).
8. `app_restart` — pick up QML/Python edits.

Data-bearing screens (library, detail, search, discover) still need a running
Seanime backend and a cached AniList token; `screenshot`/`dump_tree`/navigation
work regardless.

**Manual use.** You can also run the control server without MCP:

```bash
SEANIME_QT_AGENT=1 uv run seanime-qt
printf '{"cmd":"health"}\n' | nc 127.0.0.1 43299
```
