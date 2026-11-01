import { z } from 'zod';

/**
 * Turns off Zod's JIT before any schema parses.
 *
 * Zod 4 probes once, at the first parse, whether it may compile parsers with
 * `new Function`. Under the storefront's Content-Security-Policy, which has no
 * `'unsafe-eval'`, that probe files a violation on every page load, though Zod
 * falls back to its interpreter and nothing breaks. `jitless` skips the probe.
 * The app does the same in frontend/src/shared/lib/zodConfig.ts.
 *
 * Every module here that imports Zod imports this file first. package.json
 * lists it under `sideEffects`, so the bundler keeps the import.
 */
z.config({ jitless: true });
