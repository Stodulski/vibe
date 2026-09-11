# Vibe

Court booking for padel clubs. One repository, three packages, each with its own toolchain, lockfile and `CLAUDE.md`:

- `backend/` — Go API (Railway)
- `frontend/` — React app (Vercel, app.vibe.com.ar)
- `landing/` — Astro site (Vercel, vibe.com.ar)

Work inside the package you are changing. CI and deployment are described in `README.md`. Refer to packages by directory name; the pre-2026-09 names `padel-server` and `padel-client` are gone.

## Memory (Engram)

The Engram project is pinned to `vibe` by `.engram/config.json`. Memory exists to carry what the repository cannot: decisions and their reasons, non-obvious causes, and facts about the world around the code. It is not a log.

### Save

Call `mem_save` right after any of these happens, one fact per observation:

- A decision and its why: product rule, architecture, money flow, security posture.
- A non-obvious bug: root cause, where it lived, how the fix was proven. Never the diff.
- A discovery the repository cannot record: deploy and infrastructure shape, quirks of MercadoPago, Vercel, Railway or Brevo, CI behaviour, local environment traps.
- A convention or correction stated by the owner, with the why.
- An open decision or piece of debt that belongs to the owner, and what it blocks.

### Do not save

- Anything derivable from the repository: code structure, git history, READMEs, `CLAUDE.md` files, the OpenAPI spec.
- Progress narration, task checklists, tool output, or "done X" without a lesson.
- User prompts: always pass `capture_prompt: false`.
- SDD phase artifacts (proposal, spec, design, tasks). Those are files; memory keeps only the decision they produced.
- Secrets, tokens, `.env` values, customer data.
- More than one session summary per session. The summary is at most 600 characters: what changed and what is open. Skip it when nothing changed.

### Form

- Title required: one sentence that states the fact, at most 80 characters.
- Content in English, at most 1,200 characters. For bugs and discoveries use **What**, **Why**, **Where**, **Learned**.
- `topic_key` is `vibe/<area>/<topic>`, with area one of `server`, `client`, `landing`, `payments`, `bookings`, `auth`, `deploy`, `ci`, `tooling`, `product`.
- `type` is one of `decision`, `bugfix`, `discovery`, `architecture`, `config`, `policy` (owner rules), `project` (open items).
- Before saving, run `mem_search`. If an observation already covers the fact, `mem_update` it instead of adding a twin.

### Recall

- At session start and after compaction: `mem_context`, then `mem_search` for the topic at hand before touching code.
- Recalled memories are dated context. Verify that a path, flag or command still exists before relying on it.
