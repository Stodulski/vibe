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

/**
 * Path of one tournament's public page. The checkout returns here too
 * (`booklink.TournamentSuccess` in the backend), so the shape must agree with it.
 */
export function publicTournamentPath(slug: string, tournamentId: string): string {
  return `/c/${slug}/tournaments/${tournamentId}`;
}

/**
 * Path of the page where one category's pair signs up: its own checkout, with
 * the category in the query (`?category=<id>`). The tournament page and the
 * agenda link here.
 */
export function publicTournamentSignUpPath(slug: string, tournamentId: string, categoryId: string): string {
  return `${publicTournamentPath(slug, tournamentId)}/inscripcion?category=${encodeURIComponent(categoryId)}`;
}
