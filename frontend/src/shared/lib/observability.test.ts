import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const sentryMocks = vi.hoisted(() => ({
  init: vi.fn(),
  captureException: vi.fn(),
  setUser: vi.fn(),
  setTag: vi.fn(),
}));

vi.mock('@sentry/react', () => sentryMocks);
vi.mock('./sentry', () => ({
  initSentry: () => {
    sentryMocks.init();
  },
}));

const dsn = vi.hoisted(() => ({ value: 'https://key@example.ingest.sentry.io/1' }));
vi.mock('./env', () => ({
  get env() {
    return { VITE_SENTRY_DSN: dsn.value };
  },
}));

/**
 * A fresh copy of the module per test: the queue and the "has the SDK
 * arrived" flag are module state, and a test that starts with the SDK
 * already attached is not testing the window this exists to cover.
 */
async function loadObservability() {
  vi.resetModules();
  return import('./observability');
}

/** Waits for the dynamic imports inside `startObservability` to settle. */
async function settle() {
  await vi.waitFor(() => {
    expect(sentryMocks.init).toHaveBeenCalled();
  });
}

beforeEach(() => {
  dsn.value = 'https://key@example.ingest.sentry.io/1';
  // `whenIdle` prefers requestIdleCallback; running it inline keeps the test
  // about the queue rather than about the scheduler.
  vi.stubGlobal('requestIdleCallback', (cb: () => void) => {
    cb();
    return 1;
  });
  for (const mock of Object.values(sentryMocks)) mock.mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('observability', () => {
  it('holds calls made before the SDK arrives and replays them in order', async () => {
    const { captureException, setUser, startObservability } = await loadObservability();

    const first = new Error('before init');
    captureException(first, { tags: { route: '/login' } });
    setUser({ id: 'u1' });
    // Nothing can have reached the SDK yet — it has not been imported.
    expect(sentryMocks.captureException).not.toHaveBeenCalled();

    startObservability();
    await settle();

    expect(sentryMocks.init).toHaveBeenCalledOnce();
    expect(sentryMocks.captureException).toHaveBeenCalledWith(first, { tags: { route: '/login' } });
    expect(sentryMocks.setUser).toHaveBeenCalledWith({ id: 'u1' });
  });

  it('passes a later call straight through once the SDK is attached', async () => {
    const { captureException, startObservability } = await loadObservability();
    startObservability();
    await settle();
    sentryMocks.captureException.mockClear();

    const error = new Error('after init');
    captureException(error);
    expect(sentryMocks.captureException).toHaveBeenCalledWith(error, undefined);
  });

  // The reason `main.tsx` could stop initializing Sentry synchronously: an
  // error thrown while the SDK is still downloading is reported anyway, once
  // it arrives.
  it('reports an unhandled error thrown before the SDK has loaded', async () => {
    let releaseIdle = () => {
      /* replaced below */
    };
    vi.stubGlobal('requestIdleCallback', (cb: () => void) => {
      releaseIdle = cb;
      return 1;
    });

    const { startObservability } = await loadObservability();
    startObservability();

    const error = new Error('during first render');
    window.dispatchEvent(new ErrorEvent('error', { error, message: 'during first render' }));
    expect(sentryMocks.captureException).not.toHaveBeenCalled();

    releaseIdle();
    await settle();

    expect(sentryMocks.captureException).toHaveBeenCalledWith(error, undefined);
  });

  // Once Sentry's own global handlers are live, this module's stand-ins must
  // be gone, or one thrown error becomes two reports.
  it('stops listening for itself once the SDK has taken over', async () => {
    const { startObservability } = await loadObservability();
    startObservability();
    await settle();
    sentryMocks.captureException.mockClear();

    window.dispatchEvent(new ErrorEvent('error', { error: new Error('after init'), message: 'after init' }));
    expect(sentryMocks.captureException).not.toHaveBeenCalled();
  });

  it('never fetches the SDK when no DSN is configured', async () => {
    dsn.value = '';
    const { captureException, startObservability } = await loadObservability();

    startObservability();
    captureException(new Error('nowhere to send this'));
    await Promise.resolve();

    expect(sentryMocks.init).not.toHaveBeenCalled();
    expect(sentryMocks.captureException).not.toHaveBeenCalled();
  });
});
