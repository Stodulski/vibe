// @vitest-environment node
import { describe, expect, it } from 'vitest';
import type { ProxyOptions, UserConfig } from 'vite';

import viteConfig from './vite.config';

/**
 * The dev/E2E half of the Google redirect hop. Vercel rewrites
 * `/auth/google/callback` to the API in production (`vercel.rewrites.test.ts`
 * pins that); neither `pnpm dev` nor `vite preview --mode e2e` has a Vercel,
 * so the same path has to be proxied here or the credential Google POSTs is
 * answered with the SPA shell and sign-in cannot be exercised locally at all.
 */

function resolveProxy() {
  // The default export is the function form, called by Vite with the command
  // and mode. `serve`/`development` is the one branch that skips the
  // production-only `assertBuildEnv` gate.
  const config: UserConfig = viteConfig({ command: 'serve', mode: 'development' });

  return {
    server: config.server?.proxy ?? {},
    preview: config.preview?.proxy ?? {},
  };
}

const GOOGLE_CALLBACK = '/auth/google/callback';

describe('vite proxy', () => {
  it.each(['server', 'preview'] as const)('rewrites the Google callback to the API on the %s', (which) => {
    const entry = resolveProxy()[which][GOOGLE_CALLBACK];

    expect(entry).toBeTypeOf('object');
    const options = entry as ProxyOptions;
    expect(options.changeOrigin).toBe(true);
    expect(options.rewrite?.(GOOGLE_CALLBACK)).toBe('/api/v1/auth/google/redirect');
  });

  it('proxies it to the same target as /api', () => {
    const { server } = resolveProxy();

    expect((server[GOOGLE_CALLBACK] as ProxyOptions).target).toBe((server['/api'] as ProxyOptions).target);
  });
});
