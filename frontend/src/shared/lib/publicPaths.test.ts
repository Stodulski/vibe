import { describe, expect, it } from 'vitest';
import { publicComplexUrl } from './publicPaths';

describe('publicComplexUrl', () => {
  it('points a complex storefront at the landing, under the /c/ prefix', () => {
    expect(publicComplexUrl('los-alamos')).toBe('https://vibe.com.ar/c/los-alamos');
  });
});
