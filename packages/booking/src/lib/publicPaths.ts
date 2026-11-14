/**
 * Path of a complex's public storefront. The `/c/` prefix keeps every complex
 * slug off the platform's single-segment routes. Copied from the app's
 * `shared/lib/publicPaths.ts`: the app keeps its own, and the two must agree.
 */
export function publicComplexPath(slug: string): string {
  return `/c/${slug}`;
}

/**
 * Path of one open-play event's sign-up page, the link people share. The
 * checkout returns here too (`booklink.EventSuccess` in the backend), so the
 * shape must agree with it.
 */
export function publicEventPath(slug: string, eventId: string): string {
  return `/c/${slug}/events/${eventId}`;
}
