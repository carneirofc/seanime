"""AniList query building and payload massaging — pure functions, no Qt.

Extracted from AppController so the controller keeps only its QML-facing surface
(Q_PROPERTY / slots / signals). Everything here is stateless and unit-testable:
request-body builders for the ``anilist/list-anime`` endpoint, the current/previous
season computation, and small display formatters for AniList's data.
"""

from __future__ import annotations

import datetime

# Page size requested for every search / discover query.
PER_PAGE = 30

SEASONS = ["WINTER", "SPRING", "SUMMER", "FALL"]


# ---- season computation -------------------------------------------------

def current_season_year() -> tuple[str, int]:
    today = datetime.date.today()
    return SEASONS[(today.month - 1) // 3], today.year


def prev_season_year() -> tuple[str, int]:
    season, year = current_season_year()
    idx = SEASONS.index(season)
    return (SEASONS[3], year - 1) if idx == 0 else (SEASONS[idx - 1], year)


# ---- request bodies -----------------------------------------------------

def search_body(filters: dict, page: int) -> dict:
    """Build a ``list-anime`` body from an advanced-search filter object.

    ``filters`` keys: search, sort, genres[], tags[], format, season, year,
    status, minScore, isAdult. Empty fields are omitted so they don't constrain
    the query.

    ``isAdult`` is always sent as a bool so the server can honour it: ``False``
    (the default) restricts results to non-adult media, ``True`` returns only
    adult media (and only when the server has adult content enabled).
    """
    body: dict = {"page": page, "perPage": PER_PAGE}
    search = (filters.get("search") or "").strip()
    if search:
        body["search"] = search
    body["sort"] = [filters.get("sort") or ("SEARCH_MATCH" if search else "TRENDING_DESC")]
    genres = [g for g in (filters.get("genres") or []) if g]
    if genres:
        body["genres"] = genres
    tags = [t for t in (filters.get("tags") or []) if t]
    if tags:
        body["tags"] = tags
    if filters.get("format"):
        body["format"] = filters["format"]
    if filters.get("season"):
        body["season"] = filters["season"]
    year = int(filters.get("year") or 0)
    if year:
        body["seasonYear"] = year
    if filters.get("status"):
        body["status"] = [filters["status"]]
    min_score = int(filters.get("minScore") or 0)
    if min_score:
        body["averageScore_greater"] = min_score
    body["isAdult"] = bool(filters.get("isAdult"))
    return body


def discover_bodies() -> dict[str, dict]:
    """The ``list-anime`` bodies for each Discover carousel, keyed by category."""
    season, year = current_season_year()
    prev_season, prev_year = prev_season_year()
    base = {"page": 1, "perPage": PER_PAGE}
    return {
        "trending": {**base, "sort": ["TRENDING_DESC"]},
        "season": {**base, "sort": ["SCORE_DESC"], "season": season, "seasonYear": year},
        "prev_season": {**base, "sort": ["SCORE_DESC"], "season": prev_season, "seasonYear": prev_year},
        "upcoming": {**base, "sort": ["TRENDING_DESC"], "status": ["NOT_YET_RELEASED"]},
        "movies": {**base, "sort": ["TRENDING_DESC"], "format": "MOVIE"},
    }


# ---- payload extraction / formatting ------------------------------------

def media_list_of(data) -> list:
    """Extract a media list from the several shapes the backend returns:
    a bare ``[media, ...]`` array, ``{media: [...]}``, or ``{Page: {media: [...]}}``."""
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        if isinstance(data.get("Page"), dict):
            return data["Page"].get("media") or []
        if isinstance(data.get("media"), list):
            return data["media"]
    return []


def humanize(value: str) -> str:
    """AniList enums (e.g. ``NOT_YET_RELEASED``) → title case (``Not Yet Released``)."""
    return (value or "").replace("_", " ").title()


def next_airing_text(next_airing) -> str:
    """Render ``nextAiringEpisode`` as e.g. 'Ep 5 in 3d 4h'."""
    if not isinstance(next_airing, dict):
        return ""
    episode = next_airing.get("episode")
    seconds = next_airing.get("timeUntilAiring")
    if not episode or not seconds or seconds < 0:
        return ""
    days, rem = divmod(int(seconds), 86400)
    hours = rem // 3600
    when = f"{days}d {hours}h" if days else f"{hours}h"
    return f"Ep {episode} in {when}"


def strip_html(text: str) -> str:
    """AniList descriptions contain light HTML; strip tags for plain display."""
    out = []
    depth = 0
    for ch in text:
        if ch == "<":
            depth += 1
        elif ch == ">":
            if depth > 0:
                depth -= 1
        elif depth == 0:
            out.append(ch)
    return "".join(out).replace("&mdash;", "—").replace("&amp;", "&").strip()
