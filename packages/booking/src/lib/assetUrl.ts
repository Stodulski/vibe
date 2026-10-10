/**
 * The URL of an imported image. Vite gives a string; Astro's asset pipeline gives an
 * object with `src`. Both hosts import this package, so both shapes are accepted.
 */
export function assetUrl(asset: string | { src: string }): string {
  return typeof asset === 'string' ? asset : asset.src;
}
