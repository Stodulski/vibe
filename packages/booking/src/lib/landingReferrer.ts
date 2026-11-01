/**
 * The landing page a visitor reached this storefront from, as a path, or null.
 *
 * Only a page of the public site itself counts, and not another storefront page:
 * a visitor arriving from Google, Instagram or a shared link has no landing page to
 * go back to, and one moving between storefront steps already has their own way back.
 */
export function landingReferrerPath(referrer: string, siteUrl: string): string | null {
  if (!referrer) return null;
  let from: URL;
  let site: URL;
  try {
    from = new URL(referrer);
    site = new URL(siteUrl);
  } catch {
    return null;
  }
  if (from.origin !== site.origin) return null;
  if (from.pathname === '/c' || from.pathname.startsWith('/c/')) return null;
  return `${from.pathname}${from.search}`;
}
