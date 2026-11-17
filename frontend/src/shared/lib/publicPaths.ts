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

/** Absolute URL of the landing's tutorials, the app's user manual. */
export function landingTutorialsUrl(): string {
  return new URL('/tutoriales', env.VITE_LANDING_URL).href;
}

/**
 * Absolute URL of one landing tutorial. The app explains no screen of Mercado Pago
 * itself: those walkthroughs live on the landing, with their captures.
 */
export function landingTutorialUrl(slug: string): string {
  return new URL(`/tutoriales/${slug}`, env.VITE_LANDING_URL).href;
}
