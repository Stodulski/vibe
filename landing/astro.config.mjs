// @ts-check
import { defineConfig, envField } from 'astro/config';
import vercel from '@astrojs/vercel';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';

// https://astro.build/config
export default defineConfig({
  site: 'https://vibe.com.ar',
  // Pages are still prerendered at build time. Only a route that exports
  // `prerender = false` runs on the adapter, so the hand-written pages stay static.
  output: 'static',
  adapter: vercel(),
  publicDir: 'public',
  outDir: 'dist',
  srcDir: 'src',
  integrations: [react()],
  env: {
    schema: {
      PUBLIC_API_URL: envField.string({
        context: 'server',
        access: 'public',
        default: 'https://api.vibe.com.ar/api/v1',
      }),
    },
  },
  vite: {
    // Tailwind only processes CSS that imports it (src/styles/booking.css), so the
    // existing pages, which do not import it, get no preflight and no utilities.
    plugins: [tailwindcss()],
  },
  build: {
    // The site's whole CSS ships as one sheet (~43 KB raw), well over Astro's
    // default ~4 KB auto-inline threshold, so it always loaded as a render-blocking
    // external stylesheet. Landing visits are almost always a single page, so the
    // cross-page cache an external sheet buys is worth less than first-paint speed.
    inlineStylesheets: 'always',
  },
});
