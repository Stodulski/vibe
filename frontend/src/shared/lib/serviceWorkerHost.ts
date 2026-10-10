/**
 * Hosts where the service worker may register whatever `VITE_APP_URL` says,
 * so local dev, preview and e2e behave as they did before the host gate.
 */
const LOCAL_HOSTNAMES: ReadonlySet<string> = new Set(['localhost', '127.0.0.1']);

/** Hostname of an absolute URL, or `''` when the value does not parse. */
function hostnameOf(url: string): string {
  try {
    return new URL(url).hostname;
  } catch {
    return '';
  }
}

/**
 * Whether this page may register the service worker.
 *
 * The worker is scoped to `/` of its origin. On the landing host (vibe.com.ar)
 * the landing owns every path except the complex pages the host proxies to the
 * app, so a worker registered there would take over the landing site and answer
 * its navigations with the app shell. Only the app host registers. Malformed or
 * empty configuration fails closed everywhere except the local hosts.
 *
 * Pure: callers pass `window.location.hostname` and the configured app URL.
 */
export function shouldRegisterServiceWorker(hostname: string, appUrl: string): boolean {
  if (LOCAL_HOSTNAMES.has(hostname)) {
    return true;
  }
  const appHostname = hostnameOf(appUrl);
  return appHostname !== '' && appHostname === hostname;
}

/** Path of the web app manifest, served by the app host only. */
const MANIFEST_PATH = '/manifest.json';

/**
 * Links the web app manifest into the page, on the same hosts that may run the
 * worker. Not static in index.html: the landing owns vibe.com.ar and does not
 * proxy `/manifest.json` there, so a static link would 404 on every complex page
 * and point browsers at an install target that is not ours. Idempotent: a page
 * that already carries the link gets no second one.
 */
export function linkWebManifest(hostname: string, appUrl: string, doc: Document = document): void {
  if (!shouldRegisterServiceWorker(hostname, appUrl) || doc.head.querySelector('link[rel="manifest"]')) {
    return;
  }
  const link = doc.createElement('link');
  link.setAttribute('rel', 'manifest');
  link.setAttribute('href', MANIFEST_PATH);
  doc.head.append(link);
}

/** Script path of this app's service worker, as the build emits it. */
const APP_WORKER_PATH = '/sw.js';

/** Whether the registration runs this app's worker, in any state (active, waiting or installing). */
function runsAppWorker(registration: ServiceWorkerRegistration): boolean {
  return [registration.active, registration.waiting, registration.installing].some(
    (worker) => worker !== null && new URL(worker.scriptURL).pathname === APP_WORKER_PATH,
  );
}

/**
 * Defensive cleanup for a page that must not run this app's worker: unregisters
 * the registrations running `/sw.js` on this origin, so a worker left by an
 * earlier deploy cannot keep controlling pages here. Any other worker on the
 * origin is not ours and is left alone. Best effort; the page works without it,
 * so failures are swallowed.
 */
export async function unregisterServiceWorkers(): Promise<void> {
  if (!('serviceWorker' in navigator)) {
    return;
  }
  try {
    const registrations = await navigator.serviceWorker.getRegistrations();
    await Promise.all(registrations.filter(runsAppWorker).map((registration) => registration.unregister()));
  } catch {
    // Nothing here is worth surfacing to the person.
  }
}
