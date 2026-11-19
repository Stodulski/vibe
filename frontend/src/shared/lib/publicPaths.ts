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

/** Pieces of the social kit the landing renders: the booking link with its QR, and the three-step guide. */
export const SOCIAL_KIT_PIECES = ['reservar', 'como-reservar'] as const;
export type SocialKitPiece = (typeof SOCIAL_KIT_PIECES)[number];

/** Formats of the social kit: a story (9:16) and a post (3:4). */
export const SOCIAL_KIT_FORMATS = ['historia', 'post'] as const;
export type SocialKitFormat = (typeof SOCIAL_KIT_FORMATS)[number];

/**
 * Absolute URL of one social kit image, on `VITE_LANDING_URL`. `download` adds
 * `?descargar=1`, which the landing answers with an attachment: a plain link
 * downloads the file, since a cross-origin `download` attribute is ignored.
 * `preview` asks for the 560px version (`?email=1`): a third of the bytes and
 * half the render time, plenty for a thumbnail. Downloads stay full size.
 */
export function socialKitImageUrl(
  slug: string,
  piece: SocialKitPiece,
  format: SocialKitFormat,
  { download = false, preview = false }: { download?: boolean; preview?: boolean } = {},
): string {
  const url = new URL(`/kit/c/${slug}/${piece}-${format}.png`, env.VITE_LANDING_URL);
  if (download) url.searchParams.set('descargar', '1');
  if (preview) url.searchParams.set('email', '1');
  return url.href;
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

/**
 * Absolute URL of the owner's preview of today's "turnos libres" story image
 * on `VITE_LANDING_URL`. `vista=1` makes the landing draw an image even on a
 * day with nothing free (the publisher never sends it).
 */
export function instagramDailyStoryUrl(slug: string): string {
  const url = new URL(`/historias/c/${slug}/libres.jpg`, env.VITE_LANDING_URL);
  url.searchParams.set('vista', '1');
  return url.href;
}
