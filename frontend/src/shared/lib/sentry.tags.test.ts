import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
// Default environment (happy-dom, see vitest.config.ts) — unlike
// sentry.test.ts, these tags need `window`/`navigator` to exist.
const mockSetTag = vi.fn<(key: string, value: string) => void>();

vi.mock('@sentry/react', () => ({
  init: () => {
    /* no-op */
  },
  reactRouterBrowserTracingIntegration: () => ({ name: 'ReactRouterBrowserTracing' }),
  replayIntegration: () => ({ name: 'Replay' }),
  setTag: (key: string, value: string) => {
    mockSetTag(key, value);
  },
}));

/**
 * `initSentry` reads `env.VITE_SENTRY_DSN`, and `env` is computed once at
 * module load (`src/shared/lib/env.ts`), not re-read live. So each test stubs
 * `VITE_SENTRY_DSN` first, then resets the module registry and re-imports
 * `./sentry` fresh — that re-evaluates `env.ts` against the currently stubbed
 * value, the same way a real build only ever sees one value.
 */
async function importSentryWith(dsn: string) {
  vi.resetModules();
  vi.stubEnv('VITE_SENTRY_DSN', dsn);
  return import('./sentry');
}

describe('initSentry PWA tags', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it('tags pwa_standalone as "false" outside an installed standalone app', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('pwa_standalone', 'false');
  });

  it('tags pwa_standalone as "true" inside an installed standalone app', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList);
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('pwa_standalone', 'true');
  });

  it('tags sw_version as "none" when no service worker controls the page', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'none');
  });

  it('tags sw_version with the build release when a service worker controls the page', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    vi.stubGlobal('navigator', {
      serviceWorker: { controller: {} },
    });
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'test'); // stubbed via vitest.config.ts's `define`
  });
});
