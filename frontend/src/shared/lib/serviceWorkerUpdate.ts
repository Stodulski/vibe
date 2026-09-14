import { toast } from 'sonner';
import type { registerSW } from 'virtual:pwa-register';
import { ES_AR } from '@/shared/i18n/es_AR';
import { purgeApiCache } from '@/shared/lib/apiCache';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';

const t = ES_AR;

/** How often a long-lived tab asks the browser whether a new worker exists. */
export const SW_UPDATE_INTERVAL_MS = 5 * 60 * 1000;

/** Fixed toast id, so repeated update checks refresh one toast instead of stacking. */
export const SW_UPDATE_TOAST_ID = 'sw-update';

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
 * navigation trigger, the visibility trigger and the toast can all fire
 * without stacking `controllerchange` listeners or reloads.
 *
 * Callers decide whether it is *safe* to call; this only decides whether it is
 * possible.
 */
export function applyPendingServiceWorkerUpdate(): void {
  if (!pending || applying || !updateServiceWorker) {
    return;
  }
  applying = true;

  // The toast is the fallback for someone parked on one screen. Once the
  // update is being applied it has nothing left to offer, and leaving it up
  // through the reload would flash a stale prompt.
  toast.dismiss(SW_UPDATE_TOAST_ID);

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
 * Used by the triggers that fire without the person asking. The toast action
 * deliberately does not go through here: clicking "Actualizar" *is* the person
 * asking.
 */
function applyPendingUpdateIfSafe(): void {
  if (!pending || hasUnsavedWork()) {
    return;
  }
  applyPendingServiceWorkerUpdate();
}

/**
 * Registers the service worker in prompt mode and turns "a new build exists"
 * into an update that lands at the first safe moment, never into a reload on
 * top of what someone is doing.
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
 * two triggers spend it at moments that cost nothing:
 *
 * - An in-app navigation that changes the pathname
 *   (`useApplyUpdateOnNavigation`). The screen is being torn down anyway, and
 *   any unsaved-changes blocker has already let the navigation through by the
 *   time the trigger runs, so it is safe by construction.
 * - The tab going to the background (below), unless a form is dirty. Nobody is
 *   looking, and the person comes back to the new build already loaded.
 *
 * Both refuse while `hasUnsavedWork()`, so a dirty form is never reloaded out
 * from under anyone. The toast stays as the way in for someone who sits on one
 * screen and never navigates or switches away.
 *
 * The periodic and visibility checks only make a long-lived tab notice a
 * deploy without waiting for the browser's own 24 hour check.
 */
export function setupServiceWorkerUpdates(register: typeof registerSW): void {
  pending = false;
  applying = false;

  let registration: ServiceWorkerRegistration | undefined;

  updateServiceWorker = register({
    immediate: true,
    onRegisteredSW(_swUrl, swRegistration) {
      registration = swRegistration;
    },
    onNeedRefresh() {
      pending = true;
      toast(t.common.updateAvailable, {
        id: SW_UPDATE_TOAST_ID,
        duration: Infinity,
        action: {
          label: t.common.updateNow,
          onClick: () => {
            applyPendingServiceWorkerUpdate();
          },
        },
      });
    },
  });

  const check = () => {
    void registration?.update();
  };
  setInterval(check, SW_UPDATE_INTERVAL_MS);

  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      check();
      return;
    }
    applyPendingUpdateIfSafe();
  });
}
