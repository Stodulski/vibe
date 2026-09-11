import { toast } from 'sonner';
import type { registerSW } from 'virtual:pwa-register';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** How often a long-lived tab asks the browser whether a new worker exists. */
export const SW_UPDATE_INTERVAL_MS = 5 * 60 * 1000;

/** Fixed toast id, so repeated update checks refresh one toast instead of stacking. */
export const SW_UPDATE_TOAST_ID = 'sw-update';

/**
 * Registers the service worker in prompt mode and turns "a new build exists"
 * into a toast the person can act on, never into a reload they did not ask
 * for.
 *
 * The worker precaches index.html and every bundle, so an open tab keeps
 * running the build it loaded. In autoUpdate mode the new worker took control
 * as soon as it installed and the page reloaded on that controller change:
 * every deploy reloaded every open tab within minutes and wiped whatever the
 * person was typing (a half-filled register form, and with it the lead its
 * abandonment would have reported). In prompt mode the new worker waits, so
 * the old one keeps serving the old build's chunks to the tabs that loaded
 * them; the person sees the toast and reloads when it suits them.
 *
 * A tab that never accepts keeps the previous build for its lifetime. That is
 * intended: the next fresh load, or closing every tab, picks the new worker up
 * on its own. The periodic and visibility checks below only make a long-lived
 * tab notice a deploy without waiting for the browser's own 24 hour check.
 */
export function setupServiceWorkerUpdates(register: typeof registerSW): void {
  const updateServiceWorker = register({
    immediate: true,
    onRegisteredSW(_swUrl, registration) {
      if (!registration) {
        return;
      }
      const check = () => {
        void registration.update();
      };
      setInterval(check, SW_UPDATE_INTERVAL_MS);
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') {
          check();
        }
      });
    },
    onNeedRefresh() {
      toast(t.common.updateAvailable, {
        id: SW_UPDATE_TOAST_ID,
        duration: Infinity,
        action: {
          label: t.common.updateNow,
          onClick: () => {
            // Tells the waiting worker to skip waiting. Once it takes control
            // the page reloads; this is the only reload, and the person asked
            // for it. The reload is wired here rather than left to the plugin:
            // workbox-window only reports the takeover as an update when the
            // tab already had a controlling worker at registration time, so a
            // first visit left open across a deploy would swallow the click.
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
          },
        },
      });
    },
  });
}
