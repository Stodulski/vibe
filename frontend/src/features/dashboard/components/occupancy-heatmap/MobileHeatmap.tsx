import { DAY_LABELS, DAY_LABELS_SINGLE, HOURS, getHeatColor, heatKey } from './heatmapUtils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;
interface MobileHeatmapProps {
  dataMap: Map<string, number>;
}

export function MobileHeatmap({ dataMap }: MobileHeatmapProps) {
  return (
    <div className="overflow-x-auto sm:hidden">
      <div
        className="grid gap-[2px]"
        style={{
          gridTemplateColumns: `20px repeat(${String(HOURS.length)}, minmax(0, 1fr))`,
        }}
        role="table"
        aria-label={t.dashboard.occupancyHeatmapLabel}
      >
        {/* Header row — hours */}
        <div role="row" style={{ display: 'contents' }}>
          <div role="columnheader" aria-hidden="true" />
          {HOURS.map((hour) => (
            <div key={hour} role="columnheader" className="score-text text-micro text-text-tertiary pb-1 text-center">
              {hour % 2 === 0 ? hour : ''}
            </div>
          ))}
        </div>

        {/* Data rows — one per day */}
        {Array.from({ length: 7 }, (_, dayIdx) => (
          <div role="row" style={{ display: 'contents' }} key={dayIdx}>
            <div role="rowheader" className="text-micro text-text-tertiary flex items-center font-medium">
              {DAY_LABELS_SINGLE[dayIdx]}
            </div>
            {HOURS.map((hour) => {
              const dayOfWeek = dayIdx + 1;
              const pct = dataMap.get(heatKey(dayOfWeek, hour)) ?? 0;
              return (
                <div
                  key={heatKey(dayOfWeek, hour)}
                  role="cell"
                  aria-label={`${DAY_LABELS[dayIdx] ?? ''} ${String(hour).padStart(2, '0')}:00: ${String(pct)}%`}
                  className="aspect-square w-full rounded-[2px]"
                  style={{ backgroundColor: getHeatColor(pct) }}
                />
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}
