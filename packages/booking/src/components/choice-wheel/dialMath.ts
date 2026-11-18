/** "1 h", "1 h 30", "2 h": a duration as a player says it. */
export function durationLabel(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${String(hours)} h` : `${String(hours)} h ${String(rest)}`;
}

/** Where stop `index` of `count` sits along the bar, as a percentage of its width. */
export function stopPercent(index: number, count: number): number {
  return count < 2 ? 50 : (100 * index) / (count - 1);
}

/** The stop nearest `percent` along the bar. */
export function nearestStop(percent: number, count: number): number {
  return Math.min(count - 1, Math.max(0, Math.round((percent / 100) * (count - 1))));
}
