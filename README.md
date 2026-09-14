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

Both Vercel projects are linked to this one repository, so every push used to start a build in each
of them whatever it touched — a backend-only commit paid for two frontend builds. On 2026-09-14 that
ran the account into Vercel's daily deployment limit (`Deployment rate limited — retry in 24 hours`),
which stops production releases, not just previews. The landing now carries an `ignoreCommand` in
`landing/vercel.json` that skips a build when nothing under `landing/` changed since the commit last
deployed for that branch (`VERCEL_GIT_PREVIOUS_SHA`, which spans the whole push — `HEAD^ HEAD` would
miss a landing change made in any but the last commit).

Vercel documents exactly two exit codes for that command: `0` skips the build and `1` runs it.
Nothing is said about the rest, and `git diff` against a SHA outside a shallow clone exits `128`, so
the command resolves the SHA first and funnels every other outcome through an explicit `exit 1`. It
can only ever exit `0` or `1`, and every one of those paths except a clean diff builds: a build
skipped by mistake ships nothing and reports nothing.

`vibe-frontend` is **not** gated this way — it still builds on commits that touch only `backend/` or
`landing/`, and still spends the same budget. Gating it the same way is the other half of this fix.

Note that Vercel's checks are not among the required contexts on `main`, so a merge succeeds while
production stays on the previous build — verify a release by fetching a string the new build
_removed_, never one it added.

## History

This repository was assembled on 2026-09-10 from the former `vibe-server`, `vibe-client` and `vibe-landing` repositories. Each package's history was rewritten under its directory and merged, so `git log -- <package>/` still reaches every original commit.
