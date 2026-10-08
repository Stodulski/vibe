/**
 * Path of a complex's public storefront. The `/c/` prefix keeps every complex
 * slug off the platform's single-segment routes (`/login`, `/cash`, ...), so a
 * slug can never collide with one. Every link or navigation that builds a
 * complex URL goes through here; the router declares the same prefix in
 * `app/router/publicRoutes.tsx`.
 */
export function publicComplexPath(slug: string): string {
  return `/c/${slug}`;
}
