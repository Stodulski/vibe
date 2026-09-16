// @ts-check
import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  site: 'https://vibe.com.ar',
  output: 'static',
  publicDir: 'public',
  outDir: 'dist',
  srcDir: 'src',
  build: {
    // The site's whole CSS ships as one sheet (~43 KB raw), well over Astro's
    // default ~4 KB auto-inline threshold, so it always loaded as a render-blocking
    // external stylesheet. Landing visits are almost always a single page, so the
    // cross-page cache an external sheet buys is worth less than first-paint speed.
    inlineStylesheets: 'always',
  },
});
