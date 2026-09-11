import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import { App } from './app/App';
import { env } from '@/shared/lib/env';
import { setupServiceWorkerUpdates } from '@/shared/lib/serviceWorkerUpdate';
import './styles/globals.css';

// `registerSW` must be imported from app code so the plugin injects the
// virtual module; the update policy itself lives in serviceWorkerUpdate.ts.
setupServiceWorkerUpdates(registerSW);

const rootEl = document.getElementById('root');
if (!rootEl) {
  throw new Error('Root element #root not found in index.html');
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);

// Load Sentry after first paint to avoid blocking FCP.
if (env.VITE_SENTRY_DSN) {
  const loadSentry = () => {
    void import('./shared/lib/sentry').then(({ initSentry }) => {
      initSentry();
    });
  };

  if ('requestIdleCallback' in window) {
    requestIdleCallback(loadSentry);
  } else {
    setTimeout(loadSentry, 0);
  }
}
