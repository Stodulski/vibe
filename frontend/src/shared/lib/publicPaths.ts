import { env } from './env';

/**
 * Path of a complex's public storefront on the landing (vibe.com.ar). The `/c/`
 * prefix keeps every complex slug off the platform's single-segment routes, so a
 * slug can never collide with one. The app serves no public pages: every link to
 * one goes through `publicComplexUrl`, which points at the landing directly.
 */
function publicComplexPath(slug: string): string {
  return `/c/${slug}`;
}

/** Absolute URL of a complex's public storefront, on `VITE_LANDING_URL`. */
export function publicComplexUrl(slug: string): string {
  return new URL(publicComplexPath(slug), env.VITE_LANDING_URL).href;
}
