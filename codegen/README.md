# Codegen

Generates the contract between the Go backend and the TypeScript frontend.

Run it from the **repo root**:

```bash
go generate ./codegen     # or: npm run codegen
```

Use `go generate`, never `go run ./codegen` — the paths are resolved relative to
this directory, and only `go generate` runs the directive from the package
directory.

Run it after adding, removing or changing:

- a struct returned by a route handler,
- a route handler or its `@route`/`@param`/`@returns` doc comment,
- a plugin UI event or hook type.

The generated files are committed, and CI's "Codegen up to date" job fails if
regenerating changes anything. Never hand-edit them.

## What it writes

| Output | Produced by |
| --- | --- |
| `generated/handlers.json` | `GenerateHandlers` — parses `internal/handlers` doc comments |
| `generated/public_structs.json` | `ExtractStructs` — walks `internal` for exported types |
| `../seanime-web/src/api/generated/types.ts` | `GenerateTypescriptFile` |
| `../seanime-web/src/api/generated/endpoints.ts`, `endpoint.types.ts`, `hooks_template.ts` | `GenerateTypescriptEndpointsFile` |
| `../internal/events/endpoints.go` | `generateEventFile` (gofmt'd) |
| `../seanime-web/.../plugin/generated/plugin-events.ts` | `GeneratePluginEventFile` |
| `../seanime-web/src/api/generated/schemas.ts`, `endpoint.schemas.ts`, `schemas.assert.ts` | `GenerateZodSchemasFile` |
| `../internal/extension_repo/goja_plugin_types/app.d.ts`, `generated/hooks.mdx`, `generated/hooks.json` | `GeneratePluginHooksDefinitionFile` |

The two JSON files are intermediates: the first two stages write them, and the
later stages read them back. That is why order matters in `main.go`.

## Changing the generation pattern

**`internal/config.go` is the single place to change naming and inclusion.** It
holds the per-package TypeScript type prefixes, the lists of extra structs to
pull in even when no route references them, the endpoint-key acronym repairs, and
the shared Go→TypeScript scalar table.

Read the comment on `scalarGoToTS` before widening that table — the three
conversion functions deliberately disagree about `byte`, and
`goTypeToTypescriptType` returning `"unknown"` is how `isCustomStruct` decides a
type is a struct, which in turn decides which generated fields become pointers.

To add a route handler, annotate it; the grammar lives in
`internal/parse_handler_doc.go`:

```go
// @summary gets a thing.
// @desc Longer explanation.
// @route /api/v1/thing/{id} [GET]
// @param id - int - true - "The thing id."
// @returns models.Thing
func HandleGetThing(c *RouteCtx) error {
	type body struct {
		MediaId int `json:"mediaId"`
	}
}
```

A malformed tag is skipped silently — the generator cannot report diagnostics —
so a handler that does not show up in `endpoints.ts` usually has a typo in its
`@route`.

## Zod schemas

`schemas.ts` declares a runtime zod schema for every type in `types.ts`, and
`endpoint.schemas.ts` maps each endpoint to the schema for its response. The frontend uses
them in `src/lib/validation/api-contract.ts` to check responses **in development only**, as
a probe: a mismatch is logged and the original data is returned unchanged, never rejected.
The dynamic import there is inside a `import.meta.env.MODE === "development"` branch so a
production build drops the schema chunk entirely.

Two details are load-bearing and easy to undo by accident:

- **Optional fields are `.nullish()`, not `.optional()`.** Go marshals a nil pointer, slice
  or map as `"field": null` rather than omitting the key, and more than half of all
  generated fields are optional that way. `.optional()` alone would reject nearly every
  real response.
- **Objects are `z.looseObject`, not `z.object`.** Zod strips unknown keys by default, and
  this generator does not capture every field, so stripping would quietly discard data the
  app may read.

`schemas.assert.ts` asserts at compile time that each schema declares the same field names
as its type, comparing `keyof typeof XSchema.shape` against `keyof T.X`. It compares key
sets rather than whole types because a `.nullish()` field infers `X | null | undefined`
against the type's `X | undefined`; and it compares `.shape` rather than `z.infer` because a
loose object's inferred type carries an index signature that widens `keyof` to `string`,
which would make the assertion pass unconditionally. Add a field to a Go struct without
regenerating and `npm run typecheck` fails.

Schemas are emitted in dependency order. Where a cycle makes that impossible the property
that closes it becomes a getter, which is zod 4's form for a recursive schema — exactly one
type needs this today (`LibraryExplorer_FileTreeNodeJSON`).

## Tests

```bash
go test ./codegen/internal/...
```

Fixtures live in `internal/testdata/`: `structs/` and `handlers/` are Go sources
parsed by the generator, and `golden/` holds expected emitted output. They are
committed deliberately — `.gitignore` excludes `testdata/` everywhere else.

When a change to the emitters is intended, refresh the golden files and **read
the diff** before committing it:

```bash
go test ./codegen/internal -run TestEmit -update
```

`determinism_test.go` runs the pipeline repeatedly and compares bytes. Several
stages iterate Go maps, whose order is randomized per run, so every one of them
sorts before emitting; without that the "Codegen up to date" job would fail
intermittently on unrelated pull requests. If you add a stage that iterates a
map, sort it.

Frontend tests live in `seanime-web/src/lib/validation/`: `api-contract.test.ts` covers
endpoint resolution and the report-but-never-throw behavior, and `api-schemas.test.ts`
asserts properties over the whole generated schema set — most importantly that every object
schema parses a payload with `null` in every optional position.

Two known inconsistencies are pinned by tests that explain them rather than
fixed, because changing either would move committed output:

- `[]byte` becomes `Array<string>` in TypeScript while its Go type is recorded as
  `string` (`TestByteSliceGoTypeAndTypescriptTypeDisagree` — fixing it retypes 41
  frontend fields).
- `writeTypescriptType` tracks written type names to detect collisions but never
  acts on one.
