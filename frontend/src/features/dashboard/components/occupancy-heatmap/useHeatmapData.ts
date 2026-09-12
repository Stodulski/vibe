import { useMemo } from 'react';
import type { OccupancyDataPoint } from '@/shared/types/api.types';
import { DAY_LABELS, HOURS, heatKey } from './heatmapUtils';

/**
 * Keeps its `useMemo` where the rest of the app dropped theirs (PERF-04): the
 * React Compiler refuses this function outright — `count++` on a variable
 * captured by the `forEach` lambda is on its unsupported list — so nothing
 * memoizes this pass but this call.
 */
export function useHeatmapData(occupancy: OccupancyDataPoint[] | undefined) {
  return useMemo(() => {
    const map = new Map<string, number>();
    let totalPct = 0;
    let count = 0;
    let maxPct = 0;
    let maxDay = 0;
    let maxHour = 0;

    occupancy?.forEach((d) => {
      map.set(heatKey(d.day_of_week, d.hour), d.percentage);
      totalPct += d.percentage;
      count++;
      if (d.percentage > maxPct) {
        maxPct = d.percentage;
        maxDay = d.day_of_week;
        maxHour = d.hour;
      }
    });

    return {
      dataMap: map,
      peakDay: maxDay > 0 ? (DAY_LABELS[maxDay - 1] ?? null) : null,
      peakHour: maxHour > 0 ? `${String(maxHour).padStart(2, '0')}:00` : null,
      avgOccupancy: count > 0 ? Math.round(totalPct / (7 * HOURS.length)) : 0,
    };
  }, [occupancy]);
}
