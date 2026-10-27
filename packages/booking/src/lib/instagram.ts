/**
 * The club's Instagram profile, from the bare handle the API stores.
 *
 * The server reduces whatever the owner typed to a handle without the `@`, so
 * the page only has to build the link and the `@handle` it shows. The same URL
 * is the club's schema.org `sameAs`: its own profile elsewhere.
 */
export function instagramProfileUrl(handle: string): string {
  return `https://www.instagram.com/${handle}/`;
}
