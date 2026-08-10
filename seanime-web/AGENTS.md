# Seanime Web Agents

Seanime web is Next.js 15 app: primary UI for every Seanime client (browser, desktop). Doc captures "agents" - major runtimes and responsibility slices - inside subproject so contributors find right entry point fast.

## At-a-glance

| Agent | Scope | Key Paths | Runtime | Typical Commands |
| --- | --- | --- | --- | --- |
| Next App Shell | Routing, layouts, streaming UI | `app/`, `pages/` (if present), `app/(main)` | Node.js 20+, Next 15 | `npm run dev`, `npm run build`, `npm run start` |
| Feature Modules | Domain-specific UI, hooks, state | `app/(main)/_features/**` | React 18, TanStack Query, Jotai | Co-located with features; dev via Next |
| API Client (generated) | Typed API access to Go handlers | `src/api/generated/**` | TypeScript (codegen) | Regenerate via `go run ./codegen` in repo root |
| Shared UI System | Design system, atoms, forms | `src/components/**`, `src/ui/**` | React 18, Radix UI, Tailwind | Consumed across features |
| State & Utils | Global atoms, helpers, browser detection | `src/lib/**`, `src/state/**` | TypeScript | Imported by features and layouts |
| Build Targets | Environment-specific builds | `.env.*`, `next.config.{js|mjs}` | Next.js build pipeline | `npm run build[:desktop]` |
| Testing | Unit/integration tests | `src/**/*.{test,spec}.{ts,tsx}` | Vitest 3 | `npm run test` (configure as needed) |
| Static Assets | Logos, public files | `public/`, `src/assets/**` | N/A | Served by Next static pipeline |

## Next App Shell

- `app/` holds Next.js App Router entry points; `app/(main)` is authenticated shell for media experience.
- Middleware (if present) governs auth and localisation.
- Layouts compose global UI: shell chrome, toasts (`sonner`), theme handling (`next-themes`).

## Feature Modules

Each directory under `app/(main)/_features/` encapsulates a functional slice (e.g., library, torrent control, manga reader). Patterns:
- `*.tsx` components backed by hooks in `*.hooks.ts`.
- Shared state via Jotai atoms fed into TanStack Query queries.
- Localised styles from Tailwind CSS utilities.

## API Client (generated)

Files in `src/api/generated/` originate from Go backend via repo-level Codegen agent.
Never hand-edit these files. If you add or modify Go handlers/structs, re-run backend generator:

1. Update Go handlers/structs or plugin event definitions.
2. From repository root, run:
   ```
   go run ./codegen
   ```
   (or `go generate ./codegen`).
3. Commit regenerated files in `src/api/generated/` and `app/(main)/_features/plugin/generated/`.

`src/api/generated/client.ts` exposes typed fetchers; features import these for data.

## Shared UI System

- `src/components/` hosts reusable widgets (tables, dialogs, dropdowns).
- `src/ui/` (if present) provides design primitives built on Radix UI and tailwind utilities for consistent look-and-feel.

## State & Utilities

- `src/state/` contains global atoms (e.g., media sessions, settings).
- `src/lib/utils/` includes helpers such as `browser-detection.ts`, date/number formatting, and history helpers.
- `@total-typescript/ts-reset` ensures modern TypeScript defaults; see `tsconfig.json`.

## Build Targets

`.env.web`, `.env.desktop`, `.env.mobile` configure API endpoints, feature toggles, and analytics per target.
Scripts in `package.json` wrap Next CLI: `npm run dev` (browser), `npm run dev:desktop`, `npm run build`, and `npm run build:desktop` (static exports consumed by Go embedding).
Output directories (`out`, `out-desktop`) must be copied to `web/` or `web-desktop/` before packaging.

## Testing

- Vitest configured (`vitest.config.*`). Add tests alongside components (`*.test.tsx`).
- Use `npm run test` (add script if missing) or `vitest` directly.

## Static Assets

- `public/` holds favicons, manifests, and static JSON files.
- `docs/images` (outside this subproject) referenced via relative imports for marketing and screenshots.

## Maintaining this file

- Update `seanime-web/AGENTS.md` whenever you introduce a new runtime slice (e.g., analytics agent) or significantly reorganise directories.
- Link back to relevant READMEs (`../DEVELOPMENT_AND_BUILD.md`) when workflows evolve.
