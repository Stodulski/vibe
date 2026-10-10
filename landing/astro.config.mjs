// @ts-check
import { defineConfig, envField } from 'astro/config';
import vercel from '@astrojs/vercel';
import react from '@astrojs/react';
import tailwindcss from '@tailwindcss/vite';

const EDGE_BUILDER_STUB = '\0vibe:vercel-edge-middleware-builder';

/**
 * @astrojs/vercel 11.0.13 has a function entrypoint that imports constants from the
 * adapter's index.js. index.js imports serverless/middleware.js, which imports
 * rolldown at module top level. Vite inlines that chain into the SSR function, so
 * rolldown's native binding loads at cold start. Vercel does not deploy that binding,
 * and the function crashes on its first request.
 *
 * The edge-middleware builder only runs at build time, and the build loads it from
 * node_modules, not through Vite. So only the function's copy of the import is
 * replaced, with a stub that throws if called.
 */
function keepEdgeMiddlewareBuilderOutOfFunction() {
  return {
    name: 'vibe:keep-vercel-edge-builder-out-of-function',
    enforce: 'pre',
    resolveId(source, importer) {
      if (source === './serverless/middleware.js' && importer?.includes('@astrojs/vercel/dist/index.js')) {
        return EDGE_BUILDER_STUB;
      }
      return null;
    },
    load(id) {
      if (id !== EDGE_BUILDER_STUB) return null;
      return [
        'export function generateEdgeMiddleware() {',
        "  throw new Error('generateEdgeMiddleware runs at build time, not in the function');",
        '}',
      ].join('\n');
    },
  };
}

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
    plugins: [tailwindcss(), keepEdgeMiddlewareBuilderOutOfFunction()],
  },
  build: {
    // The site's whole CSS ships as one sheet (~43 KB raw), well over Astro's
    // default ~4 KB auto-inline threshold, so it always loaded as a render-blocking
    // external stylesheet. Landing visits are almost always a single page, so the
    // cross-page cache an external sheet buys is worth less than first-paint speed.
    inlineStylesheets: 'always',
  },
});
