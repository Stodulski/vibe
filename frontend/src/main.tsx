import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import { App } from './app/App';
import { startObservability } from '@/shared/lib/observability';
import { setupServiceWorkerUpdates } from '@/shared/lib/serviceWorkerUpdate';
import './styles/globals.css';

// `registerSW` must be imported from app code so the plugin injects the
// virtual module; the update policy itself lives in serviceWorkerUpdate.ts.
setupServiceWorkerUpdates(registerSW);

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
