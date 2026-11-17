/** How high, in pixels, the ends of the ruler climb above its center. */
export const ARC_HEIGHT = 96;

/** A tick's width: the gap between two hours on the ruler. */
export function tickWidthFor(containerWidth: number): number {
  return containerWidth >= 640 ? 80 : 48;
}

/** How much of a half circle the arc follows: below 1 it stops short of going vertical at the edge. */
const ARC_SPAN = 0.85;
/** The arc's height at the edge for that span, so the ends still climb exactly ARC_HEIGHT. */
const ARC_EDGE = 1 - Math.sqrt(1 - ARC_SPAN * ARC_SPAN);

/**
 * How a tick `offset` pixels from the center is drawn: full size and lowest at the
 * center, smaller, fainter and higher toward each edge, on an arc that closes like
 * half an O but never goes vertical, so the last ticks keep an even step. A tick
 * reaching the edge fades out instead of being cut in half.
 */
export function tickDepth(
  offset: number,
  half: number,
  width: number,
): { rise: number; scale: number; opacity: number } {
  const distance = Math.min(1, Math.abs(offset) / Math.max(1, half));
  const arc = 1 - Math.sqrt(1 - (ARC_SPAN * distance) ** 2);
  const edgeFade = Math.max(0, Math.min(1, (half - Math.abs(offset)) / (width * 0.5)));
  return {
    rise: (-ARC_HEIGHT * arc) / ARC_EDGE,
    scale: 1.15 - 0.6 * distance,
    opacity: (1 - 0.25 * distance) * edgeFade,
  };
}
