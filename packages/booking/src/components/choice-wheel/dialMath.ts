/** The dial's geometry: a half circle, center at the bottom, in SVG units. */
export const CX = 150;
export const CY = 150;
export const R = 120;

/** "1 h", "1 h 30", "2 h": a duration as a player says it. */
export function durationLabel(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${String(hours)} h` : `${String(hours)} h ${String(rest)}`;
}

/** The point on the arc at `angle` degrees (180 is the left end, 0 the right). */
export function pointAt(angle: number, radius = R): { x: number; y: number } {
  const rad = (angle * Math.PI) / 180;
  return { x: CX + radius * Math.cos(rad), y: CY - radius * Math.sin(rad) };
}

/** The angle of stop `index` of `count`, spread over the arc with room at its ends. */
export function stopAngle(index: number, count: number): number {
  return 165 - (150 * index) / Math.max(1, count - 1);
}

/** The arc from the left end to `angle`, as an SVG path. */
export function arcTo(angle: number): string {
  const start = pointAt(180);
  const end = pointAt(angle);
  return `M ${start.x.toFixed(1)} ${start.y.toFixed(1)} A ${String(R)} ${String(R)} 0 0 1 ${end.x.toFixed(1)} ${end.y.toFixed(1)}`;
}

