# Vibe

Court booking for padel clubs. One repository, three deployables:

| Package                 | Stack                   | Deploys to                 | Docs                         |
| ----------------------- | ----------------------- | -------------------------- | ---------------------------- |
| [`backend/`](backend)   | Go API, Postgres, Redis | Railway                    | [README](backend/README.md)  |
| [`frontend/`](frontend) | React, Vite, TypeScript | Vercel (`app.vibe.com.ar`) | [README](frontend/README.md) |
| [`landing/`](landing)   | Astro                   | Vercel (`vibe.com.ar`)     | scripts in `package.json`    |

Each package keeps its own toolchain, lockfile, environment file and README. There is no root build: work inside the package you are changing.

## CI

Workflows live in `.github/workflows/` and are filtered by path, so a push that only touches one package runs only that package's checks:

- `backend.yml` — lint, unit tests, build, vulnerability audit and integration tests for `backend/`.
- `frontend.yml` — typecheck, lint, format check and unit tests for `frontend/`.
- `e2e.yml` — the client's Playwright suite against an isolated API, triggered by changes to either `backend/` or `frontend/`.
- `landing-costos-mercadopago.yml` — daily check that the MercadoPago fees published by the landing match the source.

## Deployment

Railway and Vercel each build one package, so every project is configured with its package directory as the root directory:

- Railway (`backend`): root directory `backend`, Dockerfile and `railway.toml` are read from there.
- Vercel (`frontend`, `landing`): root directory set to the package; `vercel.json` in the landing applies from there.

All three projects watch this one repository, so before 2026-09-14 every push built every one of
them whatever it touched: a backend-only commit rebuilt two frontends, and a copy fix in the app
rebuilt the Go image and ran `preDeployCommand` against production Postgres. On Vercel that ran the
account into its daily deployment limit (`Deployment rate limited — retry in 24 hours`), which stops
production releases and not just previews. Each project now deploys only for its own directory:

| Project           | Where                  | Gate                              |
| ----------------- | ---------------------- | --------------------------------- |
| `vibe-frontend`   | `frontend/vercel.json` | `ignoreCommand`                   |
| `vibe-landing`    | `landing/vercel.json`  | `ignoreCommand`                   |
| backend (Railway) | `backend/railway.toml` | `watchPatterns = ["/backend/**"]` |

Each package is self-contained — its own lockfile, its own `pnpm-workspace.yaml`, its own build
inputs — and the repository root holds only `README.md` and `CLAUDE.md`, so a directory is the whole
of what a build depends on.

The Vercel gate compares against `VERCEL_GIT_PREVIOUS_SHA`, the commit last deployed for that
branch, so the diff spans the whole push; Vercel's own doc example uses `HEAD^ HEAD`, which sees only
the tip commit and would skip a build when the change sat in any earlier commit of the same push. It
is also written to stay inside the two exit codes Vercel documents — `0` skips, `1` builds — because
nothing is said about the rest and `git diff` against a SHA outside a shallow clone exits `128`. The
command resolves the SHA first and funnels every other outcome through an explicit `exit 1`, so only
a clean diff can return `0`: a build skipped by mistake ships nothing and reports nothing.

Railway's patterns are gitignore-style and resolve from the repository root even though the service's
root directory is `backend` — its documentation spells that out for a service rooted at `/app`, whose
pattern is still `/app/**`.

Two caveats worth knowing before trusting a green PR:

- Vercel's checks are not among the required contexts on `main`, so a merge succeeds while production
  stays on the previous build. Verify a release by fetching a string the new build _removed_, never
  one it added — one it added may already have existed.
- The GitHub Actions workflows filter by path on `push` but not on `pull_request`, so every PR still
  runs the backend, frontend and E2E suites. Adding a `paths` filter there would leave the required
  checks permanently pending on a PR that does not touch them, which blocks the merge; the fix is to
  filter inside the jobs so the check still reports, and it has not been done.

## History

This repository was assembled on 2026-09-10 from the former `vibe-server`, `vibe-client` and `vibe-landing` repositories. Each package's history was rewritten under its directory and merged, so `git log -- <package>/` still reaches every original commit.
