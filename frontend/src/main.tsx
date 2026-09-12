import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import { App } from './app/App';
import { initSentry } from '@/shared/lib/sentry';
import { setupServiceWorkerUpdates } from '@/shared/lib/serviceWorkerUpdate';
import './styles/globals.css';

// `registerSW` must be imported from app code so the plugin injects the
// virtual module; the update policy itself lives in serviceWorkerUpdate.ts.
setupServiceWorkerUpdates(registerSW);

// Initialized synchronously, before the first render. A deferred
// (requestIdleCallback) init used to run after createRoot().render() —
// cheaper for first paint, but it meant Sentry's own global handlers
// (unhandledrejection/error) and Session Replay were not yet wired for
// anything that happened during the initial render or before idle time
// arrived, so the earliest failures were exactly the ones never reported.
initSentry();

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('Root element #root not found in index.html');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
