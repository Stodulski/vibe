// @vitest-environment node
import { initSentry } from './sentry';

const mockInit = vi.fn<(config: Record<string, unknown>) => void>();
const mockBrowserTracingIntegration = vi.fn<() => { name: string }>().mockReturnValue({ name: 'BrowserTracing' });

vi.mock('@sentry/react', () => ({
  init: (config: Record<string, unknown>) => {
    mockInit(config);
  },
  browserTracingIntegration: () => mockBrowserTracingIntegration(),
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
      }),
    );
    import.meta.env.VITE_SENTRY_DSN = '';
  });

  it('includes browserTracingIntegration', () => {
    import.meta.env.VITE_SENTRY_DSN = 'https://test@sentry.io/123';
    initSentry();
    expect(mockBrowserTracingIntegration).toHaveBeenCalled();
    expect(mockInit).toHaveBeenCalledWith(
      expect.objectContaining({
        integrations: expect.arrayContaining([
          expect.objectContaining({ name: 'BrowserTracing' }),
        ]) as unknown as unknown[],
      }),
    );
    import.meta.env.VITE_SENTRY_DSN = '';
  });
});
