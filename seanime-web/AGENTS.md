# Seanime Web Agents

Seanime web is the primary UI, served in development by Rsbuild and in production as a
static build embedded into the Go binary. This doc captures the major responsibility slices
so contributors find the right entry point fast.

> This is **not** a Next.js app. Upstream migrated from Next.js to TanStack Router + Rsbuild.
> Some directory names (`src/app/`, `(main)`, `_components`) survived that migration and
> still look like the Next App Router — they are ordinary directories with no routing
> meaning. Routes live in `src/routes/`.

## At-a-glance

| Agent | Scope | Key Paths | Runtime | Typical Commands |
| --- | --- | --- | --- | --- |
| App Shell | Root render, providers, routing | `src/main.tsx`, `src/routes/`, `src/app/client-providers.tsx` | React 19, TanStack Router | `npm run dev` |
| Feature Modules | Domain UI, hooks, state | `src/app/(main)/**` | React 19, TanStack Query, Jotai | Co-located with features |
| API Client | Typed access to Go handlers | `src/api/**` | TypeScript (partly generated) | Regenerate via `go generate ./codegen` at repo root |
| Shared UI System | Design system, forms, primitives | `src/components/**` | Radix UI, Tailwind 4 | Consumed across features |
| State & Utils | Global atoms, helpers | `src/lib/**`, `src/app/(main)/_atoms/**` | Jotai | Imported by features |
| Validation | Schemas for untrusted input | `src/lib/validation/**` | zod 4 | See "Validation" below |
| Build Targets | Per-target env config | `.env.web`, `.env.mobile`, `rsbuild.config.ts` | Rsbuild / Rspack | `npm run build` |
| Testing | Unit tests | `src/**/*.test.ts` | Vitest 4 | `npm run test` |
| Static Assets | Icons, public files | `public/` | N/A | Served by Rsbuild |

## App Shell

- `src/main.tsx` is the entry point; `index.html` is the Rsbuild template.
- Routing is **TanStack Router, file-based** over `src/routes/`. The route tree is generated
  into `src/routeTree.gen.ts` by `@tanstack/router-plugin` during dev/build — never edit it.
- `src/app/client-providers.tsx` composes global providers; `src/app/websocket-provider.tsx`
  owns the server websocket connection.

## Feature Modules

`src/app/(main)/` holds the authenticated UI, split by domain (`manga/`, `entry/`,
`discover/`, `settings/`, …) plus shared slices in `_features/`, `_hooks/`, `_atoms/`,
`_listeners/`. Patterns:

- `*.tsx` components backed by hooks in `*.hooks.ts`.
- Shared state via Jotai atoms; server state via TanStack Query.
- Tailwind utilities for styling.

## API Client

Three layers, outermost last:

1. `src/api/client/requests.ts` — hand-written axios wrapper exposing `useServerQuery`,
   `useServerMutation`, and `buildSeaQuery`. Owns auth headers, client-id handshake, and
   error toasts. **Edit this by hand.**
2. `src/api/generated/` — emitted by the Go backend generator. **Never hand-edit.**
   `types.ts` (domain types), `endpoint.types.ts` (per-endpoint request/response),
   `endpoints.ts` (route table), `hooks_template.ts`.
3. `src/api/hooks/*.hooks.ts` — per-domain hooks features actually import.

To regenerate after a backend contract change, from the **repository root**:

```bash
go generate ./codegen
```

Run it exactly like that — never `go run ./codegen`. See
[`../codegen/README.md`](../codegen/README.md) for why, and for what the generator writes.

Commit the regenerated output in `src/api/generated/` and
`src/app/(main)/_features/plugin/generated/`. CI fails if the tree is stale.

See `../internal/AGENTS.md` for generator internals and known constraints (notably the
hand-maintained struct allowlist — a struct reachable only via a websocket event or plugin
payload will be silently missing from `types.ts` until it is added there).

## Shared UI System

- `src/components/ui/` — design primitives built on Radix UI and Tailwind.
- `src/components/shared/` — cross-feature widgets.
- There is no `src/ui/` directory.

### Forms

Forms use `react-hook-form` with zod through the project's own wrapper — do not wire
`useForm` up by hand:

- `src/components/ui/form/define-schema.ts` — `defineSchema(({ z, presets }) => …)`
- `src/components/ui/form/schema-presets.ts` — shared field presets
- `src/components/ui/form/zod-resolver.ts`, `form.tsx` — resolver and `<Form>` binding

## Validation

zod 4 covers two distinct jobs:

- **Form input** — via `defineSchema`, as above.
- **Untrusted runtime input** — schemas in `src/lib/validation/`, applied where external
  data enters the app: persisted `localStorage` state, websocket frames, and third-party
  plugin payloads.

Rules for the runtime schemas:

- Persisted state uses `atomWithValidatedStorage` (`src/lib/validation/storage.ts`) rather
  than jotai's `atomWithStorage`, which returns whatever was stored without checking its
  shape — a value written by an older Seanime version otherwise flows straight into state.
- Validation failures **degrade, never throw**: fall back to the default, drop the frame,
  log once in dev. A schema rejection must not blank a working screen.
- API responses are **not** runtime-validated. `requests.ts` casts the response to the
  generated type. Generated types are a compile-time contract only.

## Build Targets

- `npm run dev` — Rsbuild dev server with `.env.web`. `npm run dev:mobile` uses `.env.mobile`.
- `npm run build` — runs `typecheck` then `rsbuild build`, emitting to `out/`.
- `make build-web` — clean build copied into `../web/` for Go to embed.
- Env vars must be prefixed `SEA_` to reach the client (`loadEnv` in `rsbuild.config.ts`).
- Desktop targets no longer exist; there is no `build:desktop` or `out-desktop/`.

## Code Quality

- `npm run typecheck` — `tsgo` (`@typescript/native-preview`). Gated in CI; keep it at zero.
- `npm run test` — Vitest. Gated in CI.
- `npm run lint` — Biome. CI lints only files this fork changed against upstream, so the
  inherited upstream backlog is not a merge blocker; new code meets the full rule set.
- The Biome **formatter is configured but not enforced**. It matches house style, so running
  it is safe, but it is deliberately not a gate — reformatting the tree would conflict with
  every upstream merge.
- House style: 4-space indent, double quotes, no semicolons. Match the surrounding code.

## Maintaining this file

- Update when a runtime slice is added or directories are reorganised.
- Keep the codegen instructions aligned with `../internal/AGENTS.md` and `../CONTRIBUTING.md`.
- See `../DEVELOPMENT_AND_BUILD.md` for full setup and build instructions.
