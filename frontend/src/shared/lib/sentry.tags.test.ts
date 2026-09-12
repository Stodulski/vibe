// Default environment (happy-dom, see vitest.config.ts) — unlike
// sentry.test.ts, these tags need `window`/`navigator` to exist.
import { initSentry } from './sentry';

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

describe('initSentry PWA tags', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    import.meta.env.VITE_SENTRY_DSN = 'https://test@sentry.io/123';
  });

  afterEach(() => {
    import.meta.env.VITE_SENTRY_DSN = '';
    vi.unstubAllGlobals();
  });

  it('tags pwa_standalone as "false" outside an installed standalone app', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('pwa_standalone', 'false');
  });

  it('tags pwa_standalone as "true" inside an installed standalone app', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: true } as MediaQueryList);
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('pwa_standalone', 'true');
  });

  it('tags sw_version as "none" when no service worker controls the page', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'none');
  });

  it('tags sw_version with the build release when a service worker controls the page', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList);
    vi.stubGlobal('navigator', {
      serviceWorker: { controller: {} },
    });
    initSentry();
    expect(mockSetTag).toHaveBeenCalledWith('sw_version', 'test'); // stubbed via vitest.config.ts's `define`
  });
});
