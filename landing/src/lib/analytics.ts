/**
 * Cloudflare Web Analytics for the server-rendered public pages. The beacon is cookieless
 * and loads only when PUBLIC_CF_BEACON_TOKEN is set at build time; without it no script is
 * rendered at all.
 */
export const CF_BEACON_SRC = 'https://static.cloudflareinsights.com/beacon.min.js';

/**
 * The value of the beacon's `data-cf-beacon` attribute for a token, which Cloudflare
 * documents as `{"token": "<token>"}`. Null when there is no usable token. A token is
 * letters, digits, `_` and `-`; anything else would break out of the attribute, so it is
 * refused rather than escaped.
 */
export function cfBeaconAttribute(token: string | null | undefined): string | null {
  const value = (token ?? '').trim();
  if (!/^[A-Za-z0-9_-]+$/.test(value)) return null;
  return `{"token": "${value}"}`;
}
