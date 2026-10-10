import type { registerSW } from 'virtual:pwa-register';
import { purgeApiCache } from '@/shared/lib/apiCache';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';

/** How often a long-lived tab asks the browser whether a new worker exists. */
export const SW_UPDATE_INTERVAL_MS = 5 * 60 * 1000;

/**
 * `registerSW`'s "take over now" callback, kept module-level so the triggers
 * below can reach it without threading it through React.
 */
let updateServiceWorker: ((reloadPage?: boolean) => Promise<void>) | undefined;

/** True from the moment a new worker is waiting until this tab reloads. */
let pending = false;

/** Guards the handover: once asked for, asking again would be a second reload. */
let applying = false;

/**
 * Set by `applyUpdateFromNotFound` when the 404 rendered before any new worker
 * was found: the update is then applied the moment it is detected, whatever
 * the tab's visibility.
 */
let applyOnDetection = false;

/**
 * The browser's registration, kept module-level so `applyUpdateFromNotFound`
 * can force a check as well as the interval and visibility triggers.
 */
let registration: ServiceWorkerRegistration | undefined;

/**
 * Asks the browser whether `target` has a new worker.
 *
 * A failed check is harmless — the next interval or return to the tab asks
 * again — but left uncaught it is an unhandled rejection. iOS Safari rejects
 * with `InvalidStateError: newestWorker is null` when the registration has no
 * worker to compare against yet (VIBE-FRONTEND-B).
 */
function checkForUpdate(target: ServiceWorkerRegistration | undefined): void {
  target?.update().catch(() => undefined);
}

/**
 * Whether a new build is waiting for this tab to hand over.
 *
 * A plain getter rather than a subscribable store on purpose: the one reader
 * (`useApplyUpdateOnNavigation`) acts on a navigation, not on the flag
 * flipping, so it needs the current value at that instant and never needs a
 * re-render when it changes.
 */
export function isUpdatePending(): boolean {
  return pending;
}

/**
 * Hands the tab over to the waiting worker: purge, reload, take over.
 *
 * The order matters. `cleanupOutdatedCaches` only drops stale *precaches*; the
 * runtime API cache would survive the takeover and let the new build read the
 * old one's responses until they expire (PWA-08), so it is purged first. The
 * reload is wired here rather than left to the plugin because workbox-window
 * only reports the takeover as an update when the tab already had a
 * controlling worker at registration time — a first visit left open across a
 * deploy would otherwise never reload.
 *
 * Idempotent: a second call while a handover is in flight does nothing, so the
 * navigation trigger, the hide-on-background trigger and the hidden-at-detection
 * trigger can all fire without stacking `controllerchange` listeners or reloads.
 *
 * Callers decide whether it is *safe* to call; this only decides whether it is
 * possible.
 */
export function applyPendingServiceWorkerUpdate(): void {
  if (!pending || applying || !updateServiceWorker) {
    return;
  }
  applying = true;

  purgeApiCache();
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.addEventListener(
      'controllerchange',
      () => {
        window.location.reload();
      },
      { once: true },
    );
  }
  void updateServiceWorker(true);
}

/**
 * Applies a pending update only if nobody is in the middle of something.
 *
 * Used by every trigger below: none of them is a person asking, so all of
 * them defer to `hasUnsavedWork()` instead of forcing the handover.
 */
function applyPendingUpdateIfSafe(): void {
  if (!pending || hasUnsavedWork()) {
    return;
  }
  applyPendingServiceWorkerUpdate();
}

/**
 * Hands a stale tab over to the current build when it lands on the 404 page.
 * Called once by `NotFoundPage` on mount.
 *
 * A 404 is the one screen that can mean the build itself is stale: the tab's
 * router only knows the routes of the build it loaded, so a route added by a
 * later deploy (`/c/<slug>` on a build that predates it) matches nothing and
 * renders this page, while the current build would have served it. The
 * navigation trigger cannot help — the person is already on the dead end and
 * may never navigate again, and a reload alone serves the same cached build.
 *
 * If a new worker is already waiting it is applied now. Otherwise the tab
 * forces a check instead of waiting for the interval and arms
 * `applyOnDetection`, so `onNeedRefresh` applies the update on detection even
 * while the tab is visible. With no newer build nothing happens: the 404 is a
 * real one and the page stays, so there is no reload loop. Both paths still go
 * through `applyPendingUpdateIfSafe`, so unsaved work is never reloaded away.
 */
export function applyUpdateFromNotFound(): void {
  if (pending) {
    applyPendingUpdateIfSafe();
    return;
  }
  applyOnDetection = true;
  checkForUpdate(registration);
}

/**
 * Registers the service worker in prompt mode and turns "a new build exists"
 * into an update that is applied silently at the first safe moment, never
 * into a reload on top of what someone is doing, and never into a prompt the
 * person has to notice and act on.
 *
 * The worker precaches index.html and every bundle, so an open tab keeps
 * running the build it loaded — and because index.html is served cache-first,
 * a plain reload does not pick a new build up either; only the waiting worker
 * taking over does. In autoUpdate mode the new worker took control as soon as
 * it installed and the page reloaded on that controller change: every deploy
 * reloaded every open tab within minutes and wiped whatever the person was
 * typing (a half-filled register form, and with it the lead its abandonment
 * would have reported).
 *
 * Prompt mode keeps that guarantee — the new worker waits, and the old one
 * keeps serving the old build's chunks to the tabs that loaded them — while
 * four triggers spend it at moments that cost nothing:
 *
 * - An in-app navigation that changes the pathname
 *   (`useApplyUpdateOnNavigation`). The screen is being torn down anyway, and
 *   any unsaved-changes blocker has already let the navigation through by the
 *   time the trigger runs, so it is safe by construction.
 * - The tab going to the background (below), unless a form is dirty. Nobody is
 *   looking, and the person comes back to the new build already loaded.
 * - The update being detected while the tab is *already* in the background
 *   (`onNeedRefresh` below), which is what the periodic check finds on a tab
 *   nobody switched away from since the deploy. Waiting for the next
 *   `visibilitychange` would never fire, so this applies the update on the
 *   spot through the same safe path instead.
 * - The 404 page (`applyUpdateFromNotFound`), which a stale build renders for
 *   routes it predates. It applies a waiting update at once, or the next one
 *   detected regardless of visibility.
 *
 * Every trigger refuses while `hasUnsavedWork()`, so a dirty form is never
 * reloaded out from under anyone. There is no visible fallback for someone
 * who sits on one screen, never navigates and never leaves it: that person
 * keeps running the old build until one of the four triggers fires. The
 * owner chose that silence over a visible update prompt.
 *
 * The periodic and visibility checks only make a long-lived tab notice a
 * deploy without waiting for the browser's own 24 hour check.
 */
export function setupServiceWorkerUpdates(register: typeof registerSW): void {
  pending = false;
  applying = false;
  applyOnDetection = false;
  registration = undefined;

  // The periodic and visibility checks stay bound to this setup's own
  // registration rather than the module-level one, so the listeners of an
  // earlier setup never check on behalf of a later one.
  let ownRegistration: ServiceWorkerRegistration | undefined;
  const check = () => {
    checkForUpdate(ownRegistration);
  };

  updateServiceWorker = register({
    immediate: true,
    onRegisteredSW(_swUrl, swRegistration) {
      registration = ownRegistration = swRegistration;
    },
    onNeedRefresh() {
      pending = true;

      // The tab may already be in the background when this fires — the
      // periodic check runs regardless of visibility, so a deploy that lands
      // while nobody switched away is found here, not on a `visibilitychange`
      // that will never come. Applying it now, through the same safe path,
      // closes that gap instead of leaving the build stale until the next
      // hide. A 404 that armed `applyOnDetection` wants it applied now too.
      if (applyOnDetection || document.visibilityState === 'hidden') {
        applyPendingUpdateIfSafe();
      }
    },
  });

  setInterval(check, SW_UPDATE_INTERVAL_MS);

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      check();
      return;
    }
    applyPendingUpdateIfSafe();
  });
}
