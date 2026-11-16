import { describe, expect, it } from 'vitest';
import { landingTutorialUrl, publicComplexUrl } from './publicPaths';

describe('publicComplexUrl', () => {
  it('points a complex storefront at the landing, under the /c/ prefix', () => {
    expect(publicComplexUrl('los-alamos')).toBe('https://vibe.com.ar/c/los-alamos');
  });
});

describe('landingTutorialUrl', () => {
  it('points a tutorial at the landing, under /tutoriales/', () => {
    expect(landingTutorialUrl('plazo-de-acreditacion-mercado-pago')).toBe(
      'https://vibe.com.ar/tutoriales/plazo-de-acreditacion-mercado-pago',
    );
  });
});
