# `Platform.UpdateEntry` takes a params struct

To thread the new `private` and `hiddenFromStatusLists` flags into the entry-update
path, we refactor `Platform.UpdateEntry` to take an `UpdateEntryParams` struct
instead of appending two more positional pointer arguments to its already 7-argument
signature. `UpdateEntryProgress` and `UpdateEntryRepeat` keep their narrow positional
signatures — only the method that is actually growing fields is converted.

## Considered Options

- **Append two positional params** (`private *bool, hiddenFromStatusLists *bool`).
  Rejected: pushes `UpdateEntry` to 9 positional args; every future optional field
  makes call sites less readable.
- **Separate `UpdateEntryPrivacy` method + mutation.** Rejected: a modal save that
  changes status *and* privacy would cost two AniList calls, and the on-add default
  would need a second call.
- **Params struct** (chosen): one mutation per save, readable call sites, room to
  grow. Costs a one-time mechanical refactor of all five `Platform` implementers and
  the `PreUpdateEntryEvent` hook payload.

## Consequences

- `UpdateEntry` becomes stylistically inconsistent with its two sibling methods.
  Accepted deliberately: converting the stable, narrow siblings would be churn with
  no present payoff; the same pattern can be applied to them later if needed.
</content>
