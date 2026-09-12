import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import { sentryVitePlugin } from '@sentry/vite-plugin';
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
const VENDOR_CHUNKS: Record<string, string> = {
  react: 'vendor-react',
  'react-dom': 'vendor-react',
  'react-router-dom': 'vendor-react',
  '@tanstack/react-query': 'vendor-query',
  'radix-ui': 'vendor-ui',
  sonner: 'vendor-ui',
  'lucide-react': 'vendor-ui',
  leaflet: 'vendor-leaflet',
  'react-leaflet': 'vendor-leaflet',
};

// Defaults to the dev API (:8080) so `pnpm dev` is unchanged. `make e2e` in
// backend overrides this to its isolated API instance (:8081) so the E2E
// suite never talks to the dev API.
const apiProxy = {
  '/api': {
    target: process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080',
    changeOrigin: true,
  },
};

export default defineConfig({
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
        runtimeCaching: [
          {
            urlPattern: /\/api\/v1\//,
            handler: 'NetworkFirst',
            options: {
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
});
