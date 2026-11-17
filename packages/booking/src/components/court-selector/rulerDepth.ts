/**
 * The ruler as a wheel: each tick is drawn by how far it is from the center. At the
 * center it is full size; toward the ends it shrinks and fades a little, as if the
 * ruler curved away. No rotation: it skewed the times into something hard to read. Written straight to the
 * ticks' style on every scroll frame, so React does not re-render while it moves.
 */
export function applyRulerDepth(track: HTMLElement): void {
  const middle = track.scrollLeft + track.clientWidth / 2;
  // The wheel's radius in pixels: a phone's half width, and no wider on a desktop,
  // so the ends of a wide ruler stay readable instead of shrinking to nothing.
  const radius = Math.max(1, Math.min(track.clientWidth / 2, 240));
  for (const child of Array.from(track.children)) {
    const tick = child as HTMLElement;
    const offset = (tick.offsetLeft + tick.offsetWidth / 2 - middle) / radius;
    const distance = Math.min(1, Math.abs(offset));
    const scale = 1.15 - 0.3 * distance;
    // Grown from the top, so a larger tick pushes its time down, away from the marker.
    tick.style.transformOrigin = 'center top';
    // Scaled from the bottom, so every time stays on one line as the ticks grow and shrink.
    tick.style.transformOrigin = 'center bottom';
    tick.style.transform = `scale(${scale.toFixed(3)})`;
    tick.style.opacity = (1 - 0.35 * distance).toFixed(3);
  }
}
