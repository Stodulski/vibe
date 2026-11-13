# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

Install once from the repository root (`pnpm install`): `frontend/` and `landing/` share one workspace and `pnpm-lock.yaml`. Run these from `frontend/`:

```bash
pnpm dev              # Start Vite dev server (port 5173, proxies /api/* to localhost:8080)
pnpm build            # Type-check (tsc -b) then Vite build
pnpm lint             # ESLint
pnpm test             # Vitest run (all unit/integration tests)
pnpm test:watch       # Vitest watch mode
pnpm test src/features/bookings/hooks/useBookings.test.ts  # Single test file
pnpm test:coverage    # Coverage report (v8)
pnpm test:e2e         # Playwright E2E tests
pnpm test:e2e:ui      # Playwright UI mode
```

## Architecture

React 19 SPA for a padel court booking platform. Spanish (Argentina) only — all UI strings live in `src/shared/i18n/es_AR.ts` (no i18n library, just a constant object). Currency is ARS.

### Routing & Layouts

Three layout groups in `src/app/router.tsx`, all pages lazy-loaded:

- **Auth pages** (no layout): `/login`, `/register`, `/verify-email`, `/forgot-password`, `/reset-password`
- **Owner dashboard** (`DashboardLayout`): `/dashboard`, `/bookings`, `/courts`, `/clients`, `/reports`, `/settings`, `/profile`
- **Admin** (`AdminLayout`, superadmin only): `/admin` only, a placeholder (`AdminPlaceholderPage`) while the admin area is rebuilt from scratch
- **Public pages: none.** Storefronts and the booking flow live on vibe.com.ar (`landing/`). Every link to one goes through `publicComplexUrl` (`src/shared/lib/publicPaths.ts`). Unknown paths, `/c/*` included, render the 404 page.

### Feature Module Pattern

Every feature under `src/features/` follows this structure:

```
feature/
├── api/          # API methods using the shared ky client
├── hooks/        # React Query wrappers (useQuery/useMutation)
├── components/   # Feature-specific UI
├── schemas/      # Zod validation schemas
├── types/        # Feature-specific types (shared types go in src/shared/types/)
└── store/        # Feature-specific Zustand store (rare)
```

### Data Flow

1. **API calls**: `src/shared/lib/ky.ts` — ky-based HTTP client with automatic CSRF token injection (`X-CSRF-Token` header) and 401 → token refresh → retry logic.
2. **Server state**: TanStack React Query. All query keys defined in `src/shared/lib/queryKeys.ts` factory. Config: 5min staleTime, 10min gcTime, 1 retry.
3. **Client state**: Zustand store (`src/shared/stores/`) with auth slice (user, csrfToken) and UI slice (theme).
4. **Forms**: React Hook Form + Zod via `@hookform/resolvers`.

### Auth

Cookie-based sessions with CSRF protection. The ky client auto-handles token refresh on 401. Role-based access: `superadmin`, `admin`, `owner`, `staff`. `ProtectedRoute` component guards routes.

### UI Stack

- Tailwind CSS 4 with CSS custom properties for theming (defined in `src/styles/globals.css`)
- shadcn/ui components in `src/shared/components/ui/` (configured via `components.json`)
- `cn()` utility from `src/shared/lib/utils.ts` (clsx + tailwind-merge)

### Testing

- The full unit suite takes about ten minutes: every test file boots its own happy-dom (`isolate: true` in `vitest.config.ts`, kept for determinism) and workers are capped at half the cores. Run the directory you are working on (`pnpm test src/features/bookings`); leave the full run to CI or to the end of a task, and never run two full suites at once on one machine.
- React Compiler is deliberately not enabled (see README). Manual `useMemo`/`useCallback` is kept only where a test or a measurement justifies it; do not add it preventively.

- **Unit/Integration**: Vitest + Testing Library. Tests colocated with source (`*.test.ts(x)`). Setup in `src/test/setup.ts`.
- **E2E**: Playwright. Specs in `e2e/specs/`, auth setup in `e2e/setup/`. Locale `es-AR`, timezone `America/Argentina/Buenos_Aires`.

### Environment Variables

Defined in `.env.example`:

- `VITE_API_URL` — API base URL (default `/api/v1`, proxied in dev)
- `VITE_APP_URL` — This app's own URL; the service worker and web manifest register only on this host
- `VITE_LANDING_URL` — Landing URL (default `https://vibe.com.ar`): the MercadoPago fees JSON and links to public complex pages
- `VITE_SENTRY_DSN`

### Deployment

Vercel, with a `vercel.json` SPA rewrite and one `X-Robots-Tag: noindex` rule on `/(.*)`. The app serves no public pages, so there is no crawler prerender function and no API rewrite for them. PWA via vite-plugin-pwa with workbox: cache-first for fonts, and no API response is cached.

### Loading skeletons

Every component that renders loaded data (card, table, panel, summary) has its loading version in the same folder, so a layout change touches both.

- **Co-location**: `X.tsx` gets `XSkeleton.tsx` next to it. Never put the skeleton in `X.tsx`: route skeletons are imported eagerly by `routeSkeletons.tsx`, and importing `X.tsx` would drag the real component and its hooks into the main bundle.
- **Shared shell**: whatever the component and its skeleton have in common (card/panel container, padding, grid columns and breakpoints, header row) lives in one light file both import: a shell component (`XShell.tsx`, e.g. `PaymentOverviewShell`) or class constants (`xLayout.ts`, e.g. `courtLayout.ts`). Share the container and the grid, not every inner line. Generic building blocks stay in `@vibe/ui` (`Skeleton`, `SkeletonTable`) and `shared/components/common` (`PageHeaderSkeleton`, `TableFrameSkeleton`, `BackLinkSkeleton`, the section-nav and cash-tabs skeletons).
- **Page skeletons compose component skeletons**: `FooPageSkeleton` lives in the feature that owns the page (`features/foo/pages/`), is built from the component skeletons plus the shared header skeletons, and is registered in `src/app/router/routeSkeletons.tsx`, the single route-to-skeleton table (`ownerRoutes` fallbacks and `AppShellLoader` both read it). A page body that loads in place gets its own `FooContentSkeleton`/`XSkeleton` and is not duplicated in the page skeleton.
- **No `animate-fade-in`** on a skeleton or a page root: it blinks when the skeleton remounts across loading phases.
- **Keep `role="status"` and `aria-label={ES_AR.common.loading}`** on the root of a page or body skeleton (exactly one per rendered tree); nested placeholders are `aria-hidden`.
- **Check at 1280 and 320 px** against the real page: same heights, no layout shift when the data arrives.

## Code Principles

### DRY (Don't Repeat Yourself)

- Reuse the hooks in `src/features/*/hooks/` instead of making direct API calls from components.
- Shared Zod schemas live in `src/features/*/schemas/`; do not duplicate validations across forms.
- Repeated UI logic should be extracted into components in `src/shared/components/common/` or hooks in `src/shared/hooks/`.
- Use the `queryKeys` factory (`src/shared/lib/queryKeys.ts`) for all query keys. Never hardcode key arrays.
- UI strings always come from `src/shared/i18n/es_AR.ts`. Do not hardcode Spanish text in components.
- Formatting utilities (dates, currency, errors) are centralized in `src/shared/lib/utils.ts`.

### SOLID

- **S — Single Responsibility**: Each feature module has a single responsibility. Components render UI, hooks handle data logic, schemas validate, APIs communicate with the backend. Do not mix layers.
- **O — Open/Closed**: Extend behavior through composition (new hooks, new components) instead of modifying the existing ones in `src/shared/`. shadcn/ui components are extended with variants (CVA), not edited directly.
- **L — Liskov Substitution**: Components that accept the same props must be interchangeable. Respect the defined prop interfaces. Do not add hidden side effects.
- **I — Interface Segregation**: Component props should be specific to what the component needs. Do not pass whole objects (e.g. `Booking`) when the component only needs `bookingId` and `status`.
- **D — Dependency Inversion**: Components and hooks depend on abstractions (`api` from ky, the `queryKeys` factory, types from `api.types.ts`), not on concrete implementations. Features never import directly from other features; shared code goes in `src/shared/`.
