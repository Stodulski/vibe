# Vibe

Court booking for padel clubs. One repository, three deployables:

| Package | Stack | Deploys to | Docs |
| --- | --- | --- | --- |
| [`vibe-server/`](vibe-server) | Go API, Postgres, Redis | Railway | [README](vibe-server/README.md) |
| [`vibe-client/`](vibe-client) | React, Vite, TypeScript | Vercel (`app.vibe.com.ar`) | [README](vibe-client/README.md) |
| [`vibe-landing/`](vibe-landing) | Astro | Vercel (`vibe.com.ar`) | scripts in `package.json` |

Each package keeps its own toolchain, lockfile, environment file and README. There is no root build: work inside the package you are changing.

## CI

Workflows live in `.github/workflows/` and are filtered by path, so a push that only touches one package runs only that package's checks:

- `server.yml` — lint, unit tests, build, vulnerability audit and integration tests for `vibe-server/`.
- `client.yml` — typecheck, lint, format check and unit tests for `vibe-client/`.
- `e2e.yml` — the client's Playwright suite against an isolated API, triggered by changes to either `vibe-server/` or `vibe-client/`.
- `landing-costos-mercadopago.yml` — daily check that the MercadoPago fees published by the landing match the source.

## Deployment

Railway and Vercel each build one package, so every project is configured with its package directory as the root directory:

- Railway (`vibe-server`): root directory `vibe-server`, Dockerfile and `railway.toml` are read from there.
- Vercel (`vibe-client`, `vibe-landing`): root directory set to the package; `vercel.json` in the landing applies from there.

## History

This repository was assembled on 2026-09-10 from the former `vibe-server`, `vibe-client` and `vibe-landing` repositories. Each package's history was rewritten under its directory and merged, so `git log -- <package>/` still reaches every original commit.
