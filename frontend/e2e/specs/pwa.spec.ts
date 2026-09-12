import { test, expect } from '@playwright/test';

// tsconfig.e2e.json's `lib` is `["ES2023"]` (no `"DOM"`): the functions below
// run in the browser via `page.evaluate`/`page.waitForFunction` regardless,
// but TypeScript still checks their source against this file's own lib —
// @types/node's minimal `Navigator` (no `serviceWorker`) and no
// `EventTarget`/`Event` at all. `globalThis as never` casts are the one
// boundary this spec needs to describe the small ServiceWorker shape it
// actually uses, the same "reviewed unsafe boundary" approach as
// api.helper.ts's `typedJson`.
interface BrowserGlobals {
  navigator: {
    serviceWorker: {
      controller: unknown;
      getRegistration: () => Promise<ServiceWorkerRegistrationLike | undefined>;
    };
  };
  EventTarget: new () => {
    dispatchEvent(event: unknown): void;
  };
  Event: new (type: string) => unknown;
}
interface ServiceWorkerRegistrationLike {
  installing: unknown;
  dispatchEvent(event: unknown): void;
}

// TST-09: no e2e spec touched offline behaviour or the service-worker update
// prompt at all — only unit coverage of OfflineBanner and
// serviceWorkerUpdate.ts in isolation. Both need the real PWA build (`vite
// build --mode e2e`, what `make e2e` serves — see backend/scripts/e2e-run.sh)
// since the dev server registers no service worker.
test.describe('PWA — offline and update prompt', () => {
  test('shows the offline banner and keeps serving the cached shell', async ({ page, context }) => {
    // Visit once online so the service worker installs, activates and takes
    // control (`navigator.serviceWorker.controller`) — the precached shell
    // it's about to be asked to serve from cache while offline.
    await page.goto('/login');
    await page.waitForFunction(() => {
      const { navigator } = globalThis as never as BrowserGlobals;
      return Boolean(navigator.serviceWorker.controller);
    });

    await context.setOffline(true);
    try {
      await page.reload();

      // The cached shell: the login form itself, not a browser-level
      // "no internet" error page.
      await expect(page.getByRole('button', { name: 'Iniciar sesión' })).toBeVisible({ timeout: 10_000 });

      // `navigator.onLine` (what OfflineBanner reads) reflects the OS/browser
      // network state, which `context.setOffline` actually changes — unlike
      // a network condition emulated only at the request-interception layer.
      await expect(page.getByRole('alert').filter({ hasText: 'Sin conexión a internet' })).toBeVisible({
        timeout: 10_000,
      });
    } finally {
      await context.setOffline(false);
    }
  });

  // Simulates a new worker reaching the "waiting" state without a second
  // deployment: workbox-window (what vite-plugin-pwa's `virtual:pwa-register`
  // wraps) treats an `installing` worker on the *existing* registration
  // reaching `state === 'installed'` as an update exactly when the page
  // already has a controller — the same condition a real second deploy
  // produces. Faking that transition on the real registration exercises the
  // real onNeedRefresh -> toast wiring in serviceWorkerUpdate.ts, not a
  // stand-in component.
  test('shows the update toast when a new service worker reaches waiting', async ({ page }) => {
    await page.goto('/login');
    await page.waitForFunction(() => {
      const { navigator } = globalThis as never as BrowserGlobals;
      return Boolean(navigator.serviceWorker.controller);
    });

    await page.evaluate(async () => {
      const { navigator, EventTarget, Event } = globalThis as never as BrowserGlobals;
      const registration = await navigator.serviceWorker.getRegistration();
      if (!registration) throw new Error('expected an active service worker registration');

      // Not a private (`#state`) class field: `page.evaluate` sends this
      // function's source to the browser via `Function.prototype.toString`
      // and evaluates it there with no bundler pass over it, so TypeScript's
      // private-field transform helper (`_classPrivateFieldInitSpec`) is
      // never actually defined in that isolated context — a plain public
      // property sidesteps the transform entirely.
      class FakeInstallingWorker extends EventTarget {
        _state = 'installing';
        get state() {
          return this._state;
        }
        set state(value: string) {
          this._state = value;
          this.dispatchEvent(new Event('statechange'));
        }
      }
      const fakeWorker = new FakeInstallingWorker();
      Object.defineProperty(registration, 'installing', { value: fakeWorker, configurable: true });

      registration.dispatchEvent(new Event('updatefound'));
      // A controller already exists (this page loaded under one), which is
      // what tells workbox-window this is an update rather than a first
      // install — the transition to 'installed' is what it's watching for.
      fakeWorker.state = 'installed';
    });

    await expect(page.getByText('Hay una versión nueva de Vibe.')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('button', { name: 'Actualizar' })).toBeVisible();
  });
});
