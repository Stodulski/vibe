// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * Google Identity Services in `ux_mode: 'redirect'` form-POSTs the credential
 * to `login_uri` — `/auth/google/callback` on the app's own origin, because
 * the `g_csrf_token` cookie it double-submits is set there. Nothing in the
 * bundle can observe what happens next: on Vercel that path is a rewrite to
 * the API, and if it is ever deleted or ordered after the SPA catch-all,
 * Google's POST is answered with `index.html` and every Google sign-in dies
 * silently. Hence this file, next to `vercel.headers.test.ts`.
 *
 * `vite.config.ts` carries the same hop for `pnpm dev` and the E2E stack; see
 * `vite.proxy.test.ts`.
 */

interface VercelRewrite {
  source: string;
  destination: string;
}

const config = JSON.parse(readFileSync(fileURLToPath(new URL('./vercel.json', import.meta.url)), 'utf8')) as {
  rewrites: VercelRewrite[];
};

const GOOGLE_CALLBACK = '/auth/google/callback';
const SPA_CATCH_ALL = '/(.*)';

describe('vercel.json rewrites', () => {
  it('sends the Google redirect callback to the API', () => {
    const rewrite = config.rewrites.find((r) => r.source === GOOGLE_CALLBACK);
    expect(rewrite?.destination).toBe('https://api.vibe.com.ar/api/v1/auth/google/redirect');
  });

  // Vercel takes the first matching rewrite, and `/(.*)` matches everything.
  it('declares it before the SPA catch-all', () => {
    const callback = config.rewrites.findIndex((r) => r.source === GOOGLE_CALLBACK);
    const catchAll = config.rewrites.findIndex((r) => r.source === SPA_CATCH_ALL);

    expect(callback).toBeGreaterThanOrEqual(0);
    expect(catchAll).toBeGreaterThanOrEqual(0);
    expect(callback).toBeLessThan(catchAll);
  });
});
