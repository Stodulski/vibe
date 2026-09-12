// @vitest-environment node
import { initSentry, scrubEvent, scrubBreadcrumb } from './sentry';

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

describe('initSentry', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does not call Sentry.init when VITE_SENTRY_DSN is not set', () => {
    const original = import.meta.env.VITE_SENTRY_DSN;
    import.meta.env.VITE_SENTRY_DSN = '';
    initSentry();
    expect(mockInit).not.toHaveBeenCalled();
    // `VITE_SENTRY_DSN?: string` is absent-or-present, not
    // present-with-`undefined` — restore by deleting when there was none.
    if (original === undefined) {
      delete import.meta.env.VITE_SENTRY_DSN;
    } else {
      import.meta.env.VITE_SENTRY_DSN = original;
    }
  });

  it('calls Sentry.init with correct config when DSN is set', () => {
    import.meta.env.VITE_SENTRY_DSN = 'https://test@sentry.io/123';
    initSentry();
    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        dsn: 'https://test@sentry.io/123',
        tracesSampleRate: 0.1,
        release: 'test', // stubbed via vitest.config.ts's `define`
        sendDefaultPii: false,
        replaysSessionSampleRate: 0,
        replaysOnErrorSampleRate: 1.0,
        beforeSend: scrubEvent,
        beforeBreadcrumb: scrubBreadcrumb,
      }),
    );
    import.meta.env.VITE_SENTRY_DSN = '';
  });

  it('includes the router-aware tracing integration and session replay', () => {
    import.meta.env.VITE_SENTRY_DSN = 'https://test@sentry.io/123';
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
    import.meta.env.VITE_SENTRY_DSN = '';
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

  it('does not throw, skips the window-only pwa_standalone tag, and tags sw_version "none"', () => {
    import.meta.env.VITE_SENTRY_DSN = 'https://test@sentry.io/123';
    expect(() => {
      initSentry();
    }).not.toThrow();
    expect(mockSetTag).not.toHaveBeenCalledWith('pwa_standalone', expect.anything());
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'none');
    import.meta.env.VITE_SENTRY_DSN = '';
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
});
