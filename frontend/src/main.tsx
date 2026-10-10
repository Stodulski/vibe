import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import { App } from './app/App';
import { env } from '@/shared/lib/env';
import { startObservability } from '@/shared/lib/observability';
import { setupServiceWorkerUpdates } from '@/shared/lib/serviceWorkerUpdate';
import { linkWebManifest, shouldRegisterServiceWorker, unregisterServiceWorkers } from '@/shared/lib/serviceWorkerHost';
import './styles/globals.css';

// `registerSW` must be imported from app code so the plugin injects the
// virtual module; the update policy itself lives in serviceWorkerUpdate.ts.
// The worker is origin-scoped, so only the app host may register it; any
// other host (vibe.com.ar, where the landing lives) keeps none.
if (shouldRegisterServiceWorker(window.location.hostname, env.VITE_APP_URL)) {
  setupServiceWorkerUpdates(registerSW);
} else {
  void unregisterServiceWorkers();
}
// The install manifest follows the same host rule: it is linked only where the app lives.
linkWebManifest(window.location.hostname, env.VITE_APP_URL);

// Error reporting starts here, before the first render, but the SDK itself
// loads at idle time. The two are separable now: `startObservability`
// attaches its own `error`/`unhandledrejection` handlers synchronously and
// queues whatever they catch, so the earliest failures still get reported —
// the objection that made this a synchronous `initSentry()` — while the
// 88 kB gzip of `@sentry/react` no longer sits on the critical path of every
// route. Session Replay is the one thing that genuinely starts late: its
// on-error buffer covers from the load onwards, not from navigation start.
startObservability();

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('Root element #root not found in index.html');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
