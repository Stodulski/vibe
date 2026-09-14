import { z } from 'zod';

/**
 * Turns off Zod's JIT before any schema parses.
 *
 * Zod 4 compiles object parsers with `new Function` for speed, and decides
 * whether it may by probing once: it calls `new Function('')` inside a
 * try/catch. Under a Content-Security-Policy without `'unsafe-eval'` the probe
 * throws and Zod falls back to its interpreter, so nothing breaks — but the
 * browser still files a `securitypolicyviolation` for the attempt, on every
 * page load. The Report-Only policy on vercel.json proved it: that probe was
 * the only violation on /demo, /login and /register, and it would bury every
 * real report in Sentry.
 *
 * `jitless` skips the probe entirely (Zod's own source says so, next to
 * `allowsEval`). The cost is the fast object path, which an enforcing CSP
 * would disable anyway; for the payloads this app parses it is not measurable.
 *
 * Imported as the first import of env.ts, as a side effect. Not main.tsx: the
 * bundler places Zod and env.ts in a shared chunk that the browser evaluates
 * before the entry chunk, and env.ts parses at load, so a config set in
 * main.tsx ran after the probe had already fired (measured: the violation
 * survived that version of this fix).
 */
z.config({ jitless: true });
