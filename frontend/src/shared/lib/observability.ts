import { env } from './env';

/**
 * The app's error-reporting surface, decoupled from the SDK that answers it.
 *
 * `@sentry/react` and its replay integration are 88 kB gzip — 26% of
 * everything `index.html` preloads, and none of it is needed to paint a
 * screen. Importing it from `main.tsx` put all of it on the critical path of
 * every route, so the browser downloaded the crash reporter before it could
 * render the login form.
 *
 * The seven call sites this app has (`captureException` in five places,
 * `setUser` in two) go through the functions below instead. Calls made
 * before the SDK finishes loading are queued and replayed against it, so
 * moving the download off the critical path does not lose the reports that
 * happen while it is in flight — which is the property `main.tsx`'s old
 * synchronous `initSentry()` was there to protect.
 */

type Sdk = typeof import('@sentry/react');
type CaptureContext = Parameters<Sdk['captureException']>[1];
type SentryUser = Parameters<Sdk['setUser']>[0];

let sdk: Sdk | null = null;

/**
 * Calls made before the SDK arrived, in order.
 *
 * Bounded because the window it covers is a few hundred milliseconds of page
 * load: a tab that manages more than this many reports in that time is in a
 * render loop, and keeping every one of them would grow this array until the
 * tab died. The first {@link QUEUE_LIMIT} are the ones that say what went
 * wrong; the rest are the same error again.
 */
const queued: ((s: Sdk) => void)[] = [];
const QUEUE_LIMIT = 50;

function run(call: (s: Sdk) => void): void {
  if (sdk) {
    call(sdk);
    return;
  }
  if (queued.length < QUEUE_LIMIT) queued.push(call);
}

/** Report an error. A no-op when no DSN is configured, exactly as before. */
export function captureException(error: unknown, context?: CaptureContext): void {
  run((s) => {
    s.captureException(error, context);
  });
}

/** Attach (or clear) the signed-in user every later event is tagged with. */
export function setUser(user: SentryUser): void {
  run((s) => {
    s.setUser(user);
  });
}

// Until the SDK installs its own global handlers, these stand in for them.
// Without this, an error thrown during the first render — the single most
// valuable one to see, and the reason init used to be synchronous — would
// reach nothing at all.
function onWindowError(event: ErrorEvent): void {
  captureException(event.error ?? new Error(event.message));
}

function onUnhandledRejection(event: PromiseRejectionEvent): void {
  captureException(event.reason);
}

function detachEarlyHandlers(): void {
  window.removeEventListener('error', onWindowError);
  window.removeEventListener('unhandledrejection', onUnhandledRejection);
}

/** Runs `callback` once the browser is idle, or after `timeout` at the latest. */
function whenIdle(callback: () => void, timeout: number): void {
  if (typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(
      () => {
        callback();
      },
      { timeout },
    );
    return;
  }
  window.setTimeout(callback, timeout);
}

/**
 * Start reporting: catch failures immediately, load the SDK off the critical
 * path, then hand it everything caught in between.
 *
 * The load waits for idle time so the SDK's bytes never compete with the
 * render-blocking stylesheet and the route chunk for a throttled connection.
 * With no DSN configured (dev, the e2e build, a preview) the SDK is never
 * fetched at all — `initSentry` was already a no-op there, so there was
 * never anything for those bytes to do.
 */
export function startObservability(): void {
  window.addEventListener('error', onWindowError);
  window.addEventListener('unhandledrejection', onUnhandledRejection);

  if (!env.VITE_SENTRY_DSN) {
    detachEarlyHandlers();
    queued.length = 0;
    return;
  }

  whenIdle(() => {
    void (async () => {
      const [{ initSentry }, sentry] = await Promise.all([import('./sentry'), import('@sentry/react')]);
      initSentry();
      // Both of these happen before the next await point, so there is no
      // moment where this module's handlers and Sentry's own are both live
      // and one error becomes two reports.
      detachEarlyHandlers();
      sdk = sentry;
      for (const call of queued.splice(0)) call(sentry);
    })();
  }, 2000);
}
