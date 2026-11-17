/** How high, in pixels, the ends of the ruler climb above its center. */
export const ARC_HEIGHT = 96;

/** A tick's width: the gap between two hours on the ruler. */
export function tickWidthFor(containerWidth: number): number {
  return containerWidth >= 640 ? 80 : 48;
}

/** `value` modulo `size`, never negative: the hour a tick of the endless ruler shows. */
export function wrapIndex(value: number, size: number): number {
  return ((value % size) + size) % size;
}

/**
 * How a tick `offset` pixels from the center is drawn: full size and lowest at the
 * center, smaller, fainter and higher toward each edge, on half an ellipse.
 */
export function tickDepth(offset: number, half: number): { rise: number; scale: number; opacity: number } {
  const distance = Math.min(1, Math.abs(offset) / Math.max(1, half));
  return {
    rise: -ARC_HEIGHT * (1 - Math.sqrt(1 - distance * distance)),
    scale: 1.15 - 0.6 * distance,
    opacity: 1 - 0.55 * distance,
  };
}
