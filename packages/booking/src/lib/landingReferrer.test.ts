import { describe, it, expect } from 'vitest';
import { landingReferrerPath } from './landingReferrer';

const SITE = 'https://vibe.com.ar';

describe('landingReferrerPath', () => {
  it('returns the landing page the visitor came from, with its query', () => {
    expect(landingReferrerPath('https://vibe.com.ar/canchas/rosario?deporte=padel', SITE)).toBe(
      '/canchas/rosario?deporte=padel',
    );
    expect(landingReferrerPath('https://vibe.com.ar/', SITE)).toBe('/');
  });

  it('returns null for another site, so a visitor from Google or Instagram sees no back link', () => {
    expect(landingReferrerPath('https://www.google.com/', SITE)).toBeNull();
    expect(landingReferrerPath('https://app.vibe.com.ar/dashboard', SITE)).toBeNull();
  });

  it('returns null for a storefront page, which is not the landing', () => {
    expect(landingReferrerPath('https://vibe.com.ar/c/afa', SITE)).toBeNull();
    expect(landingReferrerPath('https://vibe.com.ar/c/afa/book/confirm', SITE)).toBeNull();
  });

  it('returns null when there is no referrer or it cannot be parsed', () => {
    expect(landingReferrerPath('', SITE)).toBeNull();
    expect(landingReferrerPath('not a url', SITE)).toBeNull();
  });
});
