# Context — Glossary

Canonical vocabulary for this project. Definitions only — no implementation details.

## AniList list-entry privacy

- **Private entry** — a user's list entry with AniList's `private` flag set. It is
  hidden from *other* AniList users but always fully visible to, and returned to,
  the owner. Seanime queries with the owner's token, so private entries are never
  filtered out of the owner's collection.

- **Hidden from status lists** — a user's list entry with AniList's
  `hiddenFromStatusLists` flag set. The entry is excluded from the owner's public
  status lists (Watching/Completed/etc. as shown on their profile) while remaining
  a normal entry to the owner.

- **Adult media** — media AniList marks with `isAdult`. Distinct from a per-entry
  privacy flag: it is a property of the *media*, not of the user's *entry*.

- **Adult privacy default** — the policy where, when Seanime *first brings* an adult
  media into the user's list, the resulting entry defaults to both [[private-entry]]
  and [[hidden-from-status-lists]]. Afterwards Seanime never re-forces the flags:
  the user's manual choice always persists.

- **Adult exposure alert** — because the [[adult-privacy-default]] is not
  self-healing, Seanime instead *warns* the user whenever an [[adult-media]] entry
  is not private (publicly visible). The alert never changes state on its own; it
  only informs and offers the user a one-click way to re-privatize. The alert is
  *independent* of whether the [[adult-privacy-default]] setting is enabled — it
  reflects real exposure regardless of the auto-default policy.
</content>
