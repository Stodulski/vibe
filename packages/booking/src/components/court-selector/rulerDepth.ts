/**
 * The ruler as a wheel: each tick is drawn by how far it is from the center. At the
 * center it is full size, and it shrinks and fades steadily toward each end of the
 * visible ruler, as if the ruler curved away. No rotation: it skewed the times into
 * something hard to read. Written straight to the ticks' style on every scroll frame,
 * so React does not re-render while it moves.
 */
export function applyRulerDepth(track: HTMLElement): void {
  const middle = track.scrollLeft + track.clientWidth / 2;
  // The whole visible half of the ruler is the slope, so the ticks keep shrinking
  // all the way to its edge instead of levelling off partway.
  const radius = Math.max(1, track.clientWidth / 2);
  for (const child of Array.from(track.children)) {
    const tick = child as HTMLElement;
    const distance = Math.min(1, Math.abs(tick.offsetLeft + tick.offsetWidth / 2 - middle) / radius);
    const scale = 1.15 - 0.6 * distance;
    // Scaled from the bottom, so every time stays on one line as the ticks grow and shrink.
    tick.style.transformOrigin = 'center bottom';
    tick.style.transform = `scale(${scale.toFixed(3)})`;
    tick.style.opacity = (1 - 0.55 * distance).toFixed(3);
  }
}
