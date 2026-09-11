/**
 * Attempt to compute the pixel width of a text string given a font spec.
 * Falls back to a rough character-width estimate when OffscreenCanvas is not
 * available.
 */
export function measureText(text: string, font: string): number {
  if (typeof OffscreenCanvas !== 'undefined') {
    const c = new OffscreenCanvas(1, 1);
    const ctx = c.getContext('2d');
    if (ctx) {
      ctx.font = font;
      return ctx.measureText(text).width;
    }
  }
  return text.length * 6;
}
