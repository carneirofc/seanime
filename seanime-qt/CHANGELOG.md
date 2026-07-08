# Changelog

All notable changes to the Seanime-Qt frontend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Terminal logging**: the app now emits Python logs to stderr
  (`logging.basicConfig`), level via `SEANIME_QT_LOG_LEVEL` (default `INFO`;
  set `DEBUG` to see every HTTP request). Extension loads log a type breakdown
  — e.g. `Marketplace catalogue: 68 extensions {'plugin': 57, 'custom-source': 11}`
  — and warn when no manga/torrent **providers** are present, making it obvious
  that the default marketplace ships none (they must be added by manifest URL).
- **Extension entry details**: each extension row now shows its `ID` and
  clickable `Website` / `Docs` / `Source` (manifest URL) links inline.
- **Extensions / providers**: a new "Extensions" sidebar page mirroring the web
  frontend. An **Installed** tab lists the extensions on the server (torrent /
  manga / streaming providers, plugins and custom sources) with enable/disable
  and uninstall actions (built-ins are protected), surfacing disabled and
  invalid entries with their failure reason. A **Marketplace** tab browses the
  default repository catalogue with search and type filters and one-click
  install, marking already-installed entries. An **Add extension** dialog
  installs from an arbitrary manifest URL: "Find" previews the extension, then
  "Install" adds it. Backed by `ExtensionModel`/`ExtensionFilterProxy` and the
  `/api/v1/extensions/*` endpoints (`all`, `marketplace`, `external/fetch`,
  `external/install`, `external/uninstall`, `external/disabled`,
  `external/reload`).
- **Torrent download**: a "Download" button on the anime detail page and a
  per-episode download button open a torrent browser for the entry. It
  smart-searches the Seanime server (using the default torrent provider),
  lists results with seeders/size/resolution and batch/best-release/confirmed
  badges, and — after confirming an auto-filled destination (existing local
  folder, else `<library path>/<title>`) — sends the chosen torrent to the
  configured torrent client. Batch torrents of a finished series also offer
  "Download missing episodes" (smart-select).
- **Character links**: character cards in the detail-page "Characters" strip are
  now clickable, opening the character's AniList page in the system default
  browser (`Qt.openUrlExternally`). Cards expose a pointing cursor and a button
  accessibility role; the URL comes from the AniList `siteUrl`, falling back to
  `https://anilist.co/character/<id>`.
- **Vector icon set**: bundled the Tabler Icons webfont (`qml/fonts/`) behind an
  `Icons` singleton and a reusable `Icon` component, giving React-Icons-style
  usage (`Icon { name: "search" }`). `AppButton`/`AppToolButton`/`Chip` gained
  optional icon support.
- Advanced-search **tag filter**: a searchable, category-grouped tag picker
  (`TagPopup`) backed by the full AniList tag catalog, wired into the
  `list-anime` query alongside the existing genre filter.
- **Adult-content handling** driven by the server settings
  (`anilist.enableAdultContent` / `blurAdultContent` / `splitAdultContent`):
  - An "Adult only" search toggle, shown only when the server enables adult
    content; adult tags are hidden from the tag picker otherwise.
  - Poster **blur** with click-to-reveal for adult media, plus an `18+` badge.
  - **Split** search results into separate "Results" and "Adult" sections.
  - The **library** honours blur and, when split is enabled, shows separate
    "Library" and "Adult" sections (both still respect the find-in-library
    text filter).
  - **Preview carousels** (the Discover feeds and the detail-page
    relations/recommendations strips) no longer mix adult titles when split is
    enabled: each `MediaCarousel` shows a safe strip and a separate "· adult"
    strip fed by paired `AdultFilterProxy` models over the same source.
  - The **manga library** now carries the `isAdult` flag, so it honours blur
    and, when split is enabled, shows separate "Manga" and "Adult" sections.
  - A client-local **split override** in Appearance settings ("Follow server
    setting" / "Always split" / "Never split"), letting the app force the adult
    split on or off regardless of the server's `splitAdultContent` preference.
    Persisted on this computer via `QSettings`.
- **Poster preview** in the Appearance settings preview card: a stand-in poster
  card sized from the same `Theme.posterCell*` values the real grids use, so
  dragging the "Poster size" slider grows and shrinks it live.
- Poster **score badge** (AniList mean score) on media cards.
- **Genre/tag deep-linking**: tapping a genre or tag chip on the anime detail
  header opens the advanced-search page pre-filtered to it and runs the query.
- **Ranked tag chips** on the anime detail header (from media-details), honouring
  the server's spoiler-hiding and adult-content settings.

### Changed
- Replaced the emoji and stray Unicode glyphs used as UI icons (nav symbols,
  `★ ← ✕ ▾ ✓ − 🔞 🔑`) with tintable Tabler icon glyphs, so icons match the
  accent/theme colours and render identically across platforms.
- Removed the top server-connection bar (host/port/token/Connect). The
  connection status in the sidebar is now a link that opens the server-connection
  fields in Settings › Client; startup auto-connect reads the persisted client
  prefs directly.

### Fixed
- **ComboBox dropdown labels**: options in every `AppComboBox` dropdown rendered
  blank when a `textRole` was set on a JS-array model (theme, density, adult
  split, marketplace type filter, …). The delegate resolved the label from a
  nested `contentItem` where the model roles are out of scope, and its
  `Array.isArray(control.model)` guard is false for the `QVariantList` a `var`
  array becomes — so it read an undefined role. The label is now resolved on the
  delegate root, keyed off `modelData`, so dropdown text is visible again.
