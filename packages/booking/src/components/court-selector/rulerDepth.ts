/** Whether the browser animates `.ruler-tick` from the scroll position (CSS `animation-timeline`). */
function supportsScrollDrivenWheel(): boolean {
  return typeof CSS !== 'undefined' && CSS.supports('animation-timeline: view()');
}

/** How high, in pixels, the ends of the ruler climb above its center. */
const ARC_HEIGHT = 96;

/**
 * The ruler as a wheel: each tick is drawn by how far it is from the center. At the
 * center it is full size and lowest; toward each end of the visible ruler it shrinks,
 * fades and rises along half an ellipse, as if the ruler were turning. No rotation: it skewed the times into
 * something hard to read. Written straight to the ticks' style on every scroll frame,
 * so React does not re-render while it moves.
 */
export function applyRulerDepth(track: HTMLElement): void {
  // Where the browser drives the wheel from the scroll itself (storefront.css,
  // .ruler-tick), JavaScript stays out of it: it would only lag behind a fast swipe.
  if (supportsScrollDrivenWheel()) return;
  const middle = track.scrollLeft + track.clientWidth / 2;
  // The whole visible half of the ruler is the slope, so the ruler fills the width and
  // its ticks keep shrinking and rising all the way to the edge.
  const radius = Math.max(1, track.clientWidth / 2);
  const ticks = Array.from(track.children) as HTMLElement[];
  // Every position is read before any style is written: interleaving the two forced the
  // browser to lay the ruler out again for each tick, which stuttered on a fast swipe.
  const centers = ticks.map((tick) => tick.offsetLeft + tick.offsetWidth / 2);
  ticks.forEach((tick, index) => {
    const distance = Math.min(1, Math.abs((centers[index] ?? middle) - middle) / radius);
    const scale = 1.15 - 0.6 * distance;
    // Half an O: the ticks sit on the lower half of an ellipse, flat at the center and
    // climbing steeply at the ends, as if the ruler were a wheel turning toward the visitor.
    // Ticks scale from their bottom (origin-bottom), so every time stays on one line.
    const rise = -ARC_HEIGHT * (1 - Math.sqrt(1 - distance * distance));
    tick.style.transform = `translateY(${rise.toFixed(1)}px) scale(${scale.toFixed(3)})`;
    tick.style.opacity = (1 - 0.55 * distance).toFixed(3);
  });
}
