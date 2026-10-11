import { describe, expect, it } from 'vitest';
import { assetUrl } from './assetUrl';

describe('assetUrl', () => {
  it('returns a Vite string as is', () => {
    expect(assetUrl('/assets/marker.png')).toBe('/assets/marker.png');
  });

  it('reads src from an Astro image object', () => {
    expect(assetUrl({ src: '/_astro/marker.abc.png' })).toBe('/_astro/marker.abc.png');
  });
});
