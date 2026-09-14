import { defineConfig, loadEnv, type Plugin, type UserConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { sentryVitePlugin } from '@sentry/vite-plugin';

import { assertBuildEnv } from './src/shared/lib/assertBuildEnv';
import path from 'path';

// Uploads hidden-sourcemap release artifacts to Sentry so production stack
// traces de-minify without ever serving the .map files (BLD-02/OBS-06). The
// owner loads SENTRY_AUTH_TOKEN/SENTRY_ORG/SENTRY_PROJECT as Vercel env vars;
// locally and in every CI job that doesn't set SENTRY_AUTH_TOKEN, the plugin
// must build (and stay silent) without them, so the array below only appends
// it when a token is actually present.
//
// `errorHandler` rethrows on purpose. `@sentry/bundler-plugins` deletes the
// local .map files matched by `filesToDeleteAfterUpload` in a `finally` block
// that runs even when the upload itself failed (confirmed against a real,
// deliberately invalid token: a 401 is only logged, not thrown, and the
// build still exits 0) — so a bad token in Vercel would otherwise ship a
// build with sourcemaps gone from both `dist/` and Sentry, silently, forever.
// Rethrowing turns that into a failed Vercel build instead: worse locally,
// but the alternative is losing every stack trace for that release with no
// signal at all.
const hasSentryToken = Boolean(process.env.SENTRY_AUTH_TOKEN);
const sentryPlugin = hasSentryToken
  ? [
      sentryVitePlugin({
        org: process.env.SENTRY_ORG,
        project: process.env.SENTRY_PROJECT,
        authToken: process.env.SENTRY_AUTH_TOKEN,
        release: { name: process.env.VERCEL_GIT_COMMIT_SHA },
        sourcemaps: { filesToDeleteAfterUpload: ['./dist/**/*.map'] },
        silent: true,
        errorHandler: (err) => {
          throw err;
        },
      }),
    ]
  : [];

// Third-party packages that get their own bundle, keyed by package name.
//
// leaflet and react-leaflet are deliberately absent. Naming a chunk here
// creates it eagerly, and because react-leaflet pulls in the React runtime
// the resulting `vendor-leaflet` chunk became a static import of
// `vendor-react` — so index.html modulepreloaded 48 kB gzip of map code, and
// link-tagged leaflet's 6 kB gzip stylesheet as render-blocking CSS, on
// /login and every other route (MAP-01/MAP-02/PERF-02). Without the entries
// the only boundary left is `React.lazy(() => import('./ComplexMap'))` in
// ComplexHeader.tsx, so Rollup emits the map and its CSS as an async chunk
// that loads when a complex page actually renders a map.
const VENDOR_CHUNKS: Record<string, string> = {
  react: 'vendor-react',
  'react-dom': 'vendor-react',
  'react-router-dom': 'vendor-react',
  '@tanstack/react-query': 'vendor-query',
  'radix-ui': 'vendor-ui',
  sonner: 'vendor-ui',
  'lucide-react': 'vendor-ui',
};

// Defaults to the dev API (:8080) so `pnpm dev` is unchanged. `make e2e` in
// backend overrides this to its isolated API instance (:8081) so the E2E
// suite never talks to the dev API.
const API_PROXY_TARGET = process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080';

/** Where Google's redirect-mode sign-in POSTs the credential, on this origin. */
const GOOGLE_CALLBACK_PATH = '/auth/google/callback';
/** The API endpoint that path stands in front of. */
const GOOGLE_REDIRECT_PATH = '/api/v1/auth/google/redirect';

const apiProxy = {
  '/api': {
    target: API_PROXY_TARGET,
    changeOrigin: true,
  },
  // Google Identity Services in `ux_mode: 'redirect'` form-POSTs `credential`
  // and `g_csrf_token` to `login_uri` — an app-origin path, because the
  // `g_csrf_token` cookie it double-submits is set on the app origin. In
  // production Vercel rewrites it (see vercel.json); `pnpm dev` and the E2E
  // stack have no Vercel, so the same hop is a proxy rewrite here. Method,
  // body and cookies pass through untouched, which is the whole contract.
  [GOOGLE_CALLBACK_PATH]: {
    target: API_PROXY_TARGET,
    changeOrigin: true,
    rewrite: () => GOOGLE_REDIRECT_PATH,
  },
};

/**
 * Moves the render-blocking stylesheet above the `modulepreload` links Vite
 * emits ahead of it.
 *
 * First paint waits on that one stylesheet and on nothing else — the entry is
 * a module script, which is deferred. But Vite writes it last, after a dozen
 * preloads, and a browser opens a bounded number of connections per origin:
 * measured on `/login` under Lighthouse's mobile throttling, the stylesheet
 * was the fourteenth request in the document and FCP sat at 2.5 s with the
 * page's own bytes already on the wire. Order is the whole fix; nothing about
 * what is fetched changes.
 */
function stylesheetFirst(): Plugin {
  const STYLESHEET = /[^\n]*<link[^>]+rel="stylesheet"[^>]*>/g;
  return {
    name: 'vibe:stylesheet-first',
    enforce: 'post',
    transformIndexHtml: {
      order: 'post',
      handler(html) {
        const links = html.match(STYLESHEET);
        if (!links) return html;
        const withoutLinks = html.replace(STYLESHEET, '');
        // Back in at the top of <head>, where they are ahead of every
        // preload and of the entry script.
        return withoutLinks.replace('<head>', `<head>\n${links.join('\n')}`);
      },
    },
  };
}

const config: UserConfig = {
  // Read once here (not per-request) and inlined as a string literal at
  // build time, so `src/shared/lib/sentry.ts` can tag every event with the
  // exact commit that produced the running bundle. Vercel sets this env var
  // on every build; local dev and any other host fall back to `'dev'`.
  define: {
    APP_RELEASE: JSON.stringify(process.env.VERCEL_GIT_COMMIT_SHA ?? 'dev'),
  },
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      // Prompt mode: a new worker waits instead of taking over, so a deploy
      // never reloads an open tab on its own. src/shared/lib/serviceWorkerUpdate.ts
      // shows a toast and reloads only when the person accepts it.
      registerType: 'prompt',
      manifest: false,
      // Registered from src/main.tsx via virtual:pwa-register; the plugin
      // injects nothing itself.
      injectRegister: null,
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2,webp}'],
        // Declared instead of left to the plugin default, and paired with a
        // denylist: without it every navigation — including one to the API,
        // to /.well-known or to a hashed asset — was answered with the SPA
        // shell (PWA-02/PWA-04). Those three prefixes are served by the host
        // (Vercel rewrites /api to the Railway API), never by the router.
        navigateFallback: 'index.html',
        // `/auth/google/callback` joins them for the same reason: it is a path
        // on this origin that the host (Vercel) rewrites to the API, never a
        // route the router can render. Google's redirect-mode sign-in reaches
        // it as a form POST — which the Workbox router ignores, since its
        // routes are GET-only — but a reload or a retry would arrive as a GET
        // navigation, and answering that with the SPA shell would swallow the
        // credential silently in every installed PWA.
        navigateFallbackDenylist: [
          /^\/api\//,
          /^\/\.well-known\//,
          /^\/assets\//,
          new RegExp(`^${GOOGLE_CALLBACK_PATH}$`),
        ],
        // index.html stays in the precache, so the navigation route above
        // serves it cache-first rather than network-first. That is deliberate
        // under registerType 'prompt' (PWA-05): a tab keeps the build it
        // loaded until the person accepts the update toast, and the old
        // worker keeps serving that build's chunks. A network-first
        // index.html would hand a reloaded tab the *new* HTML — referencing
        // hashed chunks the still-active old worker does not have — which is
        // exactly the torn state prompt mode exists to avoid. The 5 minute
        // update check plus the toast is what makes a new build visible.
        // Declared rather than inherited from the plugin default, so the
        // precache purge is part of this config's contract (PWA-08). It only
        // drops stale *precaches*; the runtime cache below is purged from the
        // client by src/shared/lib/apiCache.ts.
        cleanupOutdatedCaches: true,
        runtimeCaching: [
          {
            // Only the unauthenticated public booking endpoints are cached.
            // Everything else under /api/v1 is deliberately absent from
            // runtimeCaching, so the worker never handles it and the request
            // goes straight to the network: auth travels in a cookie, and a
            // NetworkFirst rule over the whole API left other people's
            // bookings and clients readable in the browser's Cache Storage
            // after logout (PWA-03). The public cache is still purged on
            // logout by src/shared/lib/apiCache.ts, which owns this name.
            urlPattern: /\/api\/v1\/public\//,
            handler: 'NetworkFirst',
            options: {
              // Must match API_CACHE_NAME in src/shared/lib/apiCache.ts, which purges
              // this cache on logout; that module cannot be imported here (it uses
              // the browser Cache API), so apiCache.test.ts pins the two in lockstep.
              cacheName: 'api-cache',
              expiration: { maxEntries: 50, maxAgeSeconds: 300 },
            },
          },
          {
            urlPattern: /\.woff2$/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'font-cache',
              expiration: { maxAgeSeconds: 365 * 24 * 60 * 60 },
            },
          },
        ],
        // No skipWaiting: the new worker waits, so the old one keeps serving
        // the old build's chunks to the tabs that loaded them until they
        // accept the update or close. The generated worker still skips
        // waiting on the SKIP_WAITING message that updateSW(true) sends.
        // clientsClaim stays: once that worker activates it must take over
        // the tab that asked, because the plugin reloads on the "controlling"
        // event and without a claim that event never fires.
        clientsClaim: true,
      },
    }),
    ...sentryPlugin,
    stylesheetFirst(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  optimizeDeps: {
    // leaflet (and its React binding) is a CJS dependency discovered lazily
    // otherwise, which can trigger a mid-session re-optimization and a full
    // dev-server reload the first time a map renders (BLD-05).
    include: ['leaflet', 'react-leaflet'],
  },
  build: {
    // Matches the `browserslist` entry in package.json so the target is
    // explicit and versioned instead of Vite's implicit default (BLD-01).
    target: ['chrome111', 'safari16.4', 'firefox128', 'edge111'],
    // 'hidden' emits .map files (so Sentry's release has real stack traces)
    // without adding a `//# sourceMappingURL` comment to the served JS, so
    // the maps are never fetched by a browser (BLD-02/OBS-06). The Sentry
    // plugin above deletes the on-disk .map files after it uploads them, and
    // it only runs when the token is set: without it (local builds, CI,
    // Vercel previews without the secret) no map is emitted at all, so
    // nothing can ship unminified source in `dist/`.
    sourcemap: hasSentryToken ? 'hidden' : false,
    rollupOptions: {
      output: {
        // Vite 8 dropped the object form of manualChunks, so the same
        // grouping is expressed as a function. Under pnpm a module path
        // looks like .../node_modules/.pnpm/pkg@1.2.3/node_modules/pkg/...,
        // so it is the LAST node_modules segment that names the package.
        manualChunks(id) {
          const marker = 'node_modules/';
          const normalized = id.replace(/\\/g, '/');
          const start = normalized.lastIndexOf(marker);
          if (start === -1) return;

          const segments = normalized.slice(start + marker.length).split('/');
          const firstSegment = segments[0];
          if (!firstSegment) return;

          const pkg = firstSegment.startsWith('@') ? `${firstSegment}/${segments[1] ?? ''}` : firstSegment;

          return VENDOR_CHUNKS[pkg];
        },
      },
    },
  },
  server: {
    port: 5173,
    proxy: apiProxy,
  },
  // `vite preview` serves the production build for the E2E suite, which
  // needs the same `/api` proxy the dev server has.
  preview: {
    proxy: apiProxy,
  },
};

export default defineConfig(({ command, mode }) => {
  // A production bundle without VITE_APP_URL would build fine and fail in the
  // browser (env.ts only runs there), so the build is gated here (BLD-04).
  // Only production mode: the e2e suite builds with `--mode e2e` and no
  // deploy variables, and relies on env.ts falling back to the page origin.
  if (command === 'build' && mode === 'production') {
    assertBuildEnv(loadEnv(mode, import.meta.dirname, 'VITE_'));
  }
  return config;
});
