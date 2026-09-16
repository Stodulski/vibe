import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
// @vitest-environment node
import { scrubEvent, scrubBreadcrumb } from './sentry';

const mockInit = vi.fn<(config: Record<string, unknown>) => void>();
const mockReactRouterBrowserTracingIntegration = vi
  .fn<(options: unknown) => { name: string }>()
  .mockReturnValue({ name: 'ReactRouterBrowserTracing' });
const mockReplayIntegration = vi.fn<() => { name: string }>().mockReturnValue({ name: 'Replay' });
const mockSetTag = vi.fn<(key: string, value: string) => void>();

vi.mock('@sentry/react', () => ({
  init: (config: Record<string, unknown>) => {
    mockInit(config);
  },
  reactRouterBrowserTracingIntegration: (options: unknown) => mockReactRouterBrowserTracingIntegration(options),
  replayIntegration: () => mockReplayIntegration(),
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
async function importSentryWith(dsn: string | undefined) {
  vi.resetModules();
  if (dsn === undefined) {
    vi.unstubAllEnvs();
  } else {
    vi.stubEnv('VITE_SENTRY_DSN', dsn);
  }
  return import('./sentry');
}

describe('initSentry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('does not call Sentry.init when VITE_SENTRY_DSN is not set', async () => {
    const { initSentry } = await importSentryWith(undefined);
    initSentry();
    expect(mockInit).not.toHaveBeenCalled();
  });

  it('calls Sentry.init with correct config when DSN is set', async () => {
    // `scrubEvent`/`scrubBreadcrumb` must come from this same re-import: the
    // module-level `import` above resolved to an earlier module-registry
    // instance, so its functions are reference-unequal to the ones this
    // fresh `initSentry` actually passes to `Sentry.init`.
    const {
      initSentry,
      scrubEvent: freshScrubEvent,
      scrubBreadcrumb: freshScrubBreadcrumb,
    } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        dsn: 'https://test@sentry.io/123',
        tunnel: '/_r/e',
        tracesSampleRate: 0.1,
        release: 'test', // stubbed via vitest.config.ts's `define`
        sendDefaultPii: false,
        replaysSessionSampleRate: 0,
        replaysOnErrorSampleRate: 1.0,
        beforeSend: freshScrubEvent,
        beforeBreadcrumb: freshScrubBreadcrumb,
      }),
    );
  });

  it('includes the router-aware tracing integration and session replay', async () => {
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    initSentry();
    expect(mockReactRouterBrowserTracingIntegration).toHaveBeenCalledWith(
      expect.objectContaining({
        useLocation: expect.any(Function) as unknown as () => void,
        useNavigationType: expect.any(Function) as unknown as () => void,
        createRoutesFromChildren: expect.any(Function) as unknown as () => void,
        matchRoutes: expect.any(Function) as unknown as () => void,
      }),
    );
    expect(mockReplayIntegration).toHaveBeenCalled();
    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        integrations: expect.arrayContaining([
          expect.objectContaining({ name: 'ReactRouterBrowserTracing' }),
          expect.objectContaining({ name: 'Replay' }),
        ]) as unknown as unknown[],
      }),
    );
  });
});

// This file runs under `@vitest-environment node`, which has no `window` at
// all but — like production Node itself — does define a bare-bones global
// `navigator` (no `serviceWorker`, no `onLine`) — matching production code
// paths that must tolerate both. See sentry.tags.test.ts for the
// browser-side pwa_standalone/sw_version tags with a full `window`.
describe('initSentry without a DOM', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it('does not throw, skips the window-only pwa_standalone tag, and tags sw_version "none"', async () => {
    const { initSentry } = await importSentryWith('https://test@sentry.io/123');
    expect(() => {
      initSentry();
    }).not.toThrow();
    expect(mockSetTag).not.toHaveBeenCalledWith('pwa_standalone', expect.anything());
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'none');
  });
});

describe('scrubEvent', () => {
  it('redacts an email address in the request URL', () => {
    const event = scrubEvent({
      request: { url: 'https://api.vibe.com.ar/v1/clients?search=juan%40example.com' },
    } as Parameters<typeof scrubEvent>[0]);
    expect(event.request?.url).toBe('https://api.vibe.com.ar/v1/clients?search=%5Bredacted-email%5D');
  });

  it('redacts a phone number appearing in an exception message', () => {
    const event = scrubEvent({
      exception: { values: [{ value: 'Invalid phone +5491123456789 for client' }] },
    } as Parameters<typeof scrubEvent>[0]);
    expect(event.exception?.values?.[0]?.value).toBe('Invalid phone [redacted-phone] for client');
  });

  it('redacts a token in the query string, as a plain string', () => {
    const event = scrubEvent({
      request: { query_string: 'token=abc123&complex=club-padel' },
    } as Parameters<typeof scrubEvent>[0]);
    expect(event.request?.query_string).toBe('token=%5Bredacted%5D&complex=club-padel');
  });

  it('leaves a URL with no PII untouched', () => {
    const event = scrubEvent({
      request: { url: 'https://api.vibe.com.ar/v1/bookings/abc123' },
    } as Parameters<typeof scrubEvent>[0]);
    expect(event.request?.url).toBe('https://api.vibe.com.ar/v1/bookings/abc123');
  });

  // SEC-04: `uploadApi.uploadToR2` PUTs straight to a presigned R2 URL with
  // its own raw `fetch()`, bypassing the shared `ky` client — its
  // AWS SigV4 query params (the actual upload credential) reached Sentry
  // request data intact until `isSensitiveParam` learned to redact them.
  it('redacts every X-Amz-* param in a presigned R2 upload URL, case-insensitively', () => {
    const event = scrubEvent({
      request: {
        url:
          'https://bucket.r2.cloudflarestorage.com/logo.webp' +
          '?X-Amz-Algorithm=AWS4-HMAC-SHA256' +
          '&X-Amz-Credential=AKIDEXAMPLE%2F20260101%2Fauto%2Fs3%2Faws4_request' +
          '&x-amz-date=20260101T000000Z' +
          '&X-Amz-Expires=3600' +
          '&X-Amz-SignedHeaders=host' +
          '&X-Amz-Signature=deadbeefcafef00d' +
          '&complex=club-padel',
      },
    } as Parameters<typeof scrubEvent>[0]);

    expect(event.request?.url).toBe(
      'https://bucket.r2.cloudflarestorage.com/logo.webp' +
        '?X-Amz-Algorithm=%5Bredacted%5D' +
        '&X-Amz-Credential=%5Bredacted%5D' +
        '&x-amz-date=%5Bredacted%5D' +
        '&X-Amz-Expires=%5Bredacted%5D' +
        '&X-Amz-SignedHeaders=%5Bredacted%5D' +
        '&X-Amz-Signature=%5Bredacted%5D' +
        '&complex=club-padel',
    );
  });
});

describe('scrubBreadcrumb', () => {
  it('redacts an email address in breadcrumb.data.url', () => {
    const breadcrumb = scrubBreadcrumb({
      data: { url: 'https://api.vibe.com.ar/v1/auth/forgot-password?email=juan%40example.com' },
    });
    expect(breadcrumb.data?.url).toBe('https://api.vibe.com.ar/v1/auth/forgot-password?email=%5Bredacted-email%5D');
  });

  it('redacts a csrf_token param in breadcrumb.data.url', () => {
    const breadcrumb = scrubBreadcrumb({
      data: { url: 'https://api.vibe.com.ar/v1/auth/refresh?csrf_token=super-secret' },
    });
    expect(breadcrumb.data?.url).toBe('https://api.vibe.com.ar/v1/auth/refresh?csrf_token=%5Bredacted%5D');
  });

  it('redacts a phone number in the breadcrumb message', () => {
    const breadcrumb = scrubBreadcrumb({ message: 'Client 1123456789 blocked' });
    expect(breadcrumb.message).toBe('Client [redacted-phone] blocked');
  });

  it('leaves a breadcrumb with no url and no PII untouched', () => {
    const breadcrumb = scrubBreadcrumb({ message: 'Navigation change' });
    expect(breadcrumb.message).toBe('Navigation change');
  });

  it('redacts X-Amz-Signature in a presigned R2 upload URL fetch breadcrumb, leaving other params', () => {
    const breadcrumb = scrubBreadcrumb({
      category: 'fetch',
      data: {
        url: 'https://bucket.r2.cloudflarestorage.com/logo.webp?X-Amz-Signature=deadbeefcafef00d&X-Amz-Expires=3600&complex=club-padel',
        method: 'PUT',
      },
    });
    expect(breadcrumb.data?.url).toBe(
      'https://bucket.r2.cloudflarestorage.com/logo.webp?X-Amz-Signature=%5Bredacted%5D&X-Amz-Expires=%5Bredacted%5D&complex=club-padel',
    );
  });
});
