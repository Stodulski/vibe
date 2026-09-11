import { getHeatColor } from './heatmapUtils';

export function HeatmapLegend() {
  return (
    <div className="mt-3 flex items-center justify-end gap-1 sm:mt-4 sm:gap-1.5">
      <span className="text-micro text-text-tertiary">0%</span>
      {[0, 25, 50, 75, 100].map((pct) => (
        <div
          key={pct}
          className="h-2.5 w-4 rounded-sm sm:h-3 sm:w-5"
          style={{ backgroundColor: getHeatColor(pct) }}
          aria-hidden="true"
        />
      ))}
      <span className="text-micro text-text-tertiary">100%</span>
    </div>
  );
}
