import { describe, expect, it } from 'vitest';
import { instagramDailyStoryUrl, landingTutorialUrl, publicComplexUrl, socialKitImageUrl } from './publicPaths';

describe('publicComplexUrl', () => {
  it('points a complex storefront at the landing, under the /c/ prefix', () => {
    expect(publicComplexUrl('los-alamos')).toBe('https://vibe.com.ar/c/los-alamos');
  });
});

describe('socialKitImageUrl', () => {
  it.each([
    ['reservar', 'historia', 'https://vibe.com.ar/kit/c/los-alamos/reservar-historia.png'],
    ['reservar', 'post', 'https://vibe.com.ar/kit/c/los-alamos/reservar-post.png'],
    ['como-reservar', 'historia', 'https://vibe.com.ar/kit/c/los-alamos/como-reservar-historia.png'],
    ['como-reservar', 'post', 'https://vibe.com.ar/kit/c/los-alamos/como-reservar-post.png'],
  ] as const)('points the %s %s image at the landing kit endpoint', (piece, format, url) => {
    expect(socialKitImageUrl('los-alamos', piece, format)).toBe(url);
    expect(socialKitImageUrl('los-alamos', piece, format, { download: false })).toBe(url);
  });

  it('adds the descargar flag only when asked for a download', () => {
    expect(socialKitImageUrl('los-alamos', 'reservar', 'post', { download: true })).toBe(
      'https://vibe.com.ar/kit/c/los-alamos/reservar-post.png?descargar=1',
    );
  });

  it('asks for the small version only for a preview, so a download stays full size', () => {
    expect(socialKitImageUrl('los-alamos', 'reservar', 'post', { preview: true })).toBe(
      'https://vibe.com.ar/kit/c/los-alamos/reservar-post.png?email=1',
    );
  });
});

describe('landingTutorialUrl', () => {
  it('points a tutorial at the landing, under /tutoriales/', () => {
    expect(landingTutorialUrl('plazo-de-acreditacion-mercado-pago')).toBe(
      'https://vibe.com.ar/tutoriales/plazo-de-acreditacion-mercado-pago',
    );
  });
});

describe('instagramDailyStoryUrl', () => {
  it("points at the landing's daily story preview for the complex, drawn even with nothing free", () => {
    expect(instagramDailyStoryUrl('los-alamos')).toBe('https://vibe.com.ar/historias/c/los-alamos/libres.jpg?vista=1');
  });
});
