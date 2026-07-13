# Adult titles: privacy-by-default, alert instead of enforce

When Seanime first brings an `isAdult` media into the user's AniList list, the new
entry defaults to `private=true` and `hiddenFromStatusLists=true` (gated by a
settings toggle, default on). We deliberately do **not** make this self-healing:
later saves never re-force the flags. Instead, whenever an adult entry is public
(`private=false`) Seanime *alerts* the user (inline modal warning + a dismissible
collection banner offering a one-click bulk "make all private"). This alert fires
regardless of the setting, because it reflects real exposure, not the auto-policy.

## Considered Options

- **Self-healing lock** (re-force private on every write, including automatic
  playback progress updates). Rejected: it fights the user — deliberately making a
  title public would be silently undone by watching an episode.
- **On-add default only, no alert.** Rejected: gives no feedback when an adult title
  is already, or becomes, public.
- **Alert instead of force** (chosen): privacy-by-default at the moment of add, plus
  visibility of exposure, but the user's deliberate choice always wins.

## Consequences

- Adding a media has no edit form (the modal is edit-only; adds go through a quick
  "Add to list" button and `AddMediaToCollection`), so the default is enforced
  **server-side** on the add path: `HandleEditAnilistListEntry` resolves it for new
  adult entries, and `AddMediaToCollection` applies it per media. Both key on
  nil-vs-explicit flag pointers, so an explicit user choice is never overridden.
- The edit modal reflects the resulting state for existing entries (Private / Hide
  toggles) and shows an inline warning when an adult entry is public.
- "Exposure" is keyed on `private` only; `hiddenFromStatusLists` is a softer,
  list-cosmetic preference and does not trigger the alert.
- The setting is enabled by default (`gorm:"default:true"`), which also enables it
  for existing users on migration. This is intentional (privacy-protective) and
  non-destructive — it only affects entries added *after* the migration.
</content>
