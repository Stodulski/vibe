# Vibe Client

React single-page application for Vibe, a booking platform for sports complexes. It serves the owner dashboard (courts, bookings, clients, reports), a superadmin section, and the public booking pages clients use to reserve a court. Spanish (Argentina) only, currency ARS.

## Stack

- React 19, TypeScript, [Vite](https://vite.dev)
- [TanStack React Query](https://tanstack.com/query) for server state, [Zustand](https://zustand-demo.pmnd.rs) for client state
- [Tailwind CSS 4](https://tailwindcss.com) with [shadcn/ui](https://ui.shadcn.com) components
- [React Hook Form](https://react-hook-form.com) + [Zod](https://zod.dev) for forms and validation
- [ky](https://github.com/sindresorhus/ky) as the HTTP client against the [backend](../backend) API
- [Sentry](https://sentry.io) for optional error tracking
- [Vitest](https://vitest.dev) + Testing Library for unit tests, [Playwright](https://playwright.dev) for E2E
- Deployed on [Vercel](https://vercel.com)

## Prerequisites

- Node.js `>=24 <25` (see `.nvmrc` and `package.json` `engines`)
- pnpm `>=11` (pinned as `pnpm@11.25.0` in `packageManager`)
- The [backend](../backend) API running locally on port `8080` (default), or another API URL configured via `VITE_API_URL`
- For E2E tests only: Docker (the API lives in `../backend`, in this same repository)

## Install and run locally

```bash
git clone https://github.com/Stodulski/vibe.git
cd vibe/frontend
pnpm install
```

Copy the versioned example env file and adjust as needed (defaults work for local development against an API on `:8080`):

```bash
cp .env.example .env
```

Start the dev server:

```bash
pnpm dev
```

The app runs on `http://localhost:5173`. Requests to `/api/*` are proxied by Vite (`vite.config.ts`) to `http://localhost:8080` by default, so the API must be running there (or point `VITE_API_PROXY_TARGET` at a different host, which only affects the dev proxy, not the built app).

## Environment variables

`.env.example` is versioned in this repository and lists every variable. `.env` is gitignored and never committed. The table below is read from `src/shared/lib/env.ts` (the runtime schema) and `src/vite-env.d.ts` (the type declarations); all `VITE_*` variables are optional, since none of them stop the app from building or booting.

| Variable                  | Purpose                                                                                                                                                                                                   | Required | Default               |
| ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- | --------------------- |
| `VITE_API_URL`            | Base URL the app calls for the API.                                                                                                                                                                       | Optional | `/api/v1`             |
| `VITE_APP_URL`            | This app's own URL, used for canonical/OG tags.                                                                                                                                                           | Optional | none                  |
| `VITE_LANDING_URL`        | Base URL of the marketing landing site, used to read its published MercadoPago fees JSON.                                                                                                                 | Optional | `https://vibe.com.ar` |
| `VITE_SENTRY_DSN`         | Sentry DSN. Enables error tracking when set.                                                                                                                                                              | Optional | none                  |
| `VITE_MP_APP_ID`          | MercadoPago application ID used by the owner-facing "connect MercadoPago" OAuth flow.                                                                                                                     | Optional | none                  |
| `VITE_TURNSTILE_SITE_KEY` | Enables the Cloudflare Turnstile challenge on register, login and forgot-password; must pair with the server's `TURNSTILE_SECRET_KEY`.                                                                    | Optional | none                  |
| `VITE_GOOGLE_CLIENT_ID`   | Enables "Continuar con Google" on login/register. The same OAuth client id the server reads as `GOOGLE_OAUTH_CLIENT_ID`; the app's origin must be an authorized JavaScript origin for it in Google Cloud. | Optional | none                  |

All `VITE_*` variables are inlined into the built bundle at build time and are not secret.

## Folder structure

```
src/app/          Router, layouts, top-level providers
src/features/     One folder per domain (admin, auth, bookings, clients, complex, courts,
                  dashboard, public-booking), each with api/hooks/components/schemas/types
src/shared/       Cross-feature code: components, hooks, i18n (es_AR only), lib, schemas,
                  stores, types
src/pages/        Route-level page components (admin, auth, owner, public)
e2e/              Playwright specs, page objects and setup (auth, global truncation)
```

## Useful commands

```bash
pnpm lint              # ESLint
pnpm typecheck          # tsc -b --force
pnpm format:check       # Prettier check
pnpm build              # typecheck then vite build
pnpm test               # full Vitest unit/integration suite
pnpm test src/features/bookings   # a single directory or file
pnpm test:coverage      # Vitest with v8 coverage
pnpm test:e2e           # Playwright, against the dev servers by default
```

The full `pnpm test` run takes about ten minutes: every test file boots its own `happy-dom` instance (`isolate: true` in `vitest.config.ts`, kept for determinism) and workers are capped at half the available cores. When working on one area, run `pnpm test <path>` instead and leave the full run to CI.

### E2E testing (isolated stack)

`pnpm test:e2e` by default targets the dev servers (Vite on `:5173`, the API on `:8080`, the dev Postgres) via `reuseExistingServer`, and its `global.setup.ts` truncates whatever database the E2E healthcheck reaches. That is safe only when that is genuinely an isolated E2E stack, not your development database.

To run the suite without touching the dev servers or dev database, use the isolated target in the sibling `backend` package instead:

```bash
cd ../backend && make e2e
```

This starts an E2E-only Postgres (`:5433`) and Redis (`:6380`), boots a purpose-built API instance on `:8081` with a minimal, secret-free environment, then runs `pnpm test:e2e` here with `PLAYWRIGHT_BASE_URL` / `VITE_API_PROXY_TARGET` / `E2E_*` variables pointed at that stack (client on `:5174`) instead of the dev servers. See `backend/scripts/e2e-run.sh`.

## Deploy

The app deploys to [Vercel](https://vercel.com). `vercel.json` sets the Vite framework preset, `pnpm build` as the build command, `dist` as the output directory, an SPA rewrite (`/(.*) → /index.html`), long-cache headers for static assets, and `X-Robots-Tag: noindex` on authenticated and non-public routes.

`middleware.ts` runs at the edge and rewrites requests from known bot/crawler user agents on public `/:slug` complex pages to the backend's prerender endpoint (`BACKEND_URL` + `/api/v1/public/prerender/:slug`), so social previews and search crawlers see server-rendered HTML instead of the empty SPA shell.

CI runs on GitHub Actions (`.github/workflows/frontend.yml` at the repository root) with four jobs on every push and pull request to `main` that touches `frontend/`: `typecheck` (`tsc -b --force`), `lint` (`pnpm lint`), `format` (`pnpm format:check`), and `test` (`pnpm test`). The end-to-end suite runs from `.github/workflows/e2e.yml`, which is triggered by changes to either `frontend/` or `backend/` and runs `make e2e` from the server package.

## Conventions

Commits follow [Conventional Commits](https://www.conventionalcommits.org/).

## Troubleshooting

- **Port `5173` already in use**: another Vite instance is running. Stop it, or run `pnpm dev -- --port 5174`.
- **The E2E suite is hitting your running dev API**: `global.setup.ts` truncates its target database. If you ran `pnpm test:e2e` directly while a dev API was up on `:8080`, it just wiped your local development data. Always use `cd ../backend && make e2e` for an isolated stack, see above.
- **Public complex pages show no preview when shared on WhatsApp or social media locally**: `middleware.ts` only runs on Vercel's edge runtime, not under `pnpm dev`. Bot-only prerendering can only be verified after a Vercel deploy or with `vercel dev`.

## License

Vibe is free software: you can redistribute it and/or modify it under the terms of the GNU Affero General Public License as published by the Free Software Foundation, version 3. See [LICENSE](LICENSE). If you run a modified version as a network service, the AGPL requires you to offer its source to the users of that service.

Copyright (C) 2026 Vibe.
