import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'path';

// Third-party packages that get their own bundle, keyed by package name.
const VENDOR_CHUNKS: Record<string, string> = {
  react: 'vendor-react',
  'react-dom': 'vendor-react',
  'react-router-dom': 'vendor-react',
  '@tanstack/react-query': 'vendor-query',
  '@tanstack/react-table': 'vendor-table',
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
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: false,
      // Registered from src/main.tsx via virtual:pwa-register so the page
      // reloads when a new service worker takes control.
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
        skipWaiting: true,
        clientsClaim: true,
      },
    }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
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
