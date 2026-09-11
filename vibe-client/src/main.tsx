import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { registerSW } from 'virtual:pwa-register';
import { App } from './app/App';
import { env } from '@/shared/lib/env';
import './styles/globals.css';

// The service worker precaches index.html and every bundle, so an open tab
// keeps running the previous deploy until the new worker takes control.
// registerSW in autoUpdate mode reloads the page on that controller change,
// and the update checks below make long-lived tabs notice a deploy without
// waiting for the browser's own 24 hour check.
const SW_UPDATE_INTERVAL_MS = 5 * 60 * 1000;

registerSW({
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
});

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
