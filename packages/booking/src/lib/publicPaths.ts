/**
 * Path of a complex's public storefront. The `/c/` prefix keeps every complex
 * slug off the platform's single-segment routes. Copied from the app's
 * `shared/lib/publicPaths.ts`: the app keeps its own, and the two must agree.
 */
export function publicComplexPath(slug: string): string {
  return `/c/${slug}`;
}
