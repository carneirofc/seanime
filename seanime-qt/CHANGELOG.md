# Changelog

All notable changes to the Seanime-Qt frontend will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Advanced-search **tag filter**: a searchable, category-grouped tag picker
  (`TagPopup`) backed by the full AniList tag catalog, wired into the
  `list-anime` query alongside the existing genre filter.
- **Adult-content handling** driven by the server settings
  (`anilist.enableAdultContent` / `blurAdultContent` / `splitAdultContent`):
  - An "Adult only" search toggle, shown only when the server enables adult
    content; adult tags are hidden from the tag picker otherwise.
  - Poster **blur** with click-to-reveal for adult media, plus an `18+` badge.
  - **Split** search results into separate "Results" and "Adult" sections.
- Poster **score badge** (AniList mean score) on media cards.
