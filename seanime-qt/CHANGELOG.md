# Changelog

All notable changes to the Seanime-Qt frontend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
- Poster **score badge** (AniList mean score) on media cards.
- **Genre/tag deep-linking**: tapping a genre or tag chip on the anime detail
  header opens the advanced-search page pre-filtered to it and runs the query.
- **Ranked tag chips** on the anime detail header (from media-details), honouring
  the server's spoiler-hiding and adult-content settings.

### Changed
- Replaced the emoji and stray Unicode glyphs used as UI icons (nav symbols,
  `★ ← ✕ ▾ ✓ − 🔞 🔑`) with tintable Tabler icon glyphs, so icons match the
  accent/theme colours and render identically across platforms.
