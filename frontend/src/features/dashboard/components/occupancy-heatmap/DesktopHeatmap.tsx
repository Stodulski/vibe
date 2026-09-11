import { ES_AR } from '@/shared/i18n/es_AR';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/shared/components/ui/tooltip';
import { DAY_LABELS, DAY_LABELS_SHORT, HOURS, getHeatColor, heatKey } from './heatmapUtils';

const t = ES_AR;

interface DesktopHeatmapRowProps {
  hour: number;
  dataMap: Map<string, number>;
}

function DesktopHeatmapRow({ hour, dataMap }: DesktopHeatmapRowProps) {
  return (
    <div role="row" style={{ display: 'contents' }}>
      <div role="rowheader" className="score-text flex items-center pr-4 text-micro text-text-tertiary sm:pr-5">
        {String(hour).padStart(2, '0')}:00
      </div>
      {Array.from({ length: 7 }, (_, dayIdx) => {
        const dayOfWeek = dayIdx + 1;
        const pct = dataMap.get(heatKey(dayOfWeek, hour)) ?? 0;
        return (
          <Tooltip key={heatKey(dayOfWeek, hour)}>
            <TooltipTrigger asChild>
              <div
                role="cell"
                // Puts the cell in the tab order so the tooltip — otherwise
                // hover-only — also opens on keyboard focus, which Radix's
                // Tooltip already wires up for a focusable trigger. jsx-a11y
                // flags `tabIndex` on a non-interactive `role="cell"`, but a
                // grid cell whose only content is a hover-revealed value
                // (the occupancy %) needs exactly this to give a keyboard
                // user the same access a mouse user gets — there is no
                // interactive-role table-cell alternative.
                // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- see comment above
                tabIndex={0}
                aria-label={`${DAY_LABELS[dayIdx] ?? ''} ${String(hour).padStart(2, '0')}:00: ${String(pct)}%`}
                // A fixed height, not an aspect ratio. At 3:1 a 200px-wide column made
                // every row 67px tall, so twenty-four of them would run past 1600px
                // — the chart would need scrolling to be read as a whole, which is
                // the one thing a heatmap is for.
                className="h-5 w-full cursor-pointer rounded-sm transition-colors duration-150 hover:ring-1 hover:ring-primary-500/30"
                style={{ backgroundColor: getHeatColor(pct) }}
              />
            </TooltipTrigger>
            <TooltipContent>
              <p className="text-xs">
                {DAY_LABELS[dayIdx]} {String(hour).padStart(2, '0')}:00 · {pct}%{' '}
                {t.dashboard.occupancyRate.toLowerCase()}
              </p>
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}

interface DesktopHeatmapProps {
  dataMap: Map<string, number>;
}

export function DesktopHeatmap({ dataMap }: DesktopHeatmapProps) {
  return (
    <div className="hidden sm:block">
      <div
        className="grid gap-[3px]"
        style={{
          gridTemplateColumns: `minmax(32px, auto) repeat(7, minmax(0, 1fr))`,
        }}
        role="table"
        aria-label={t.dashboard.occupancyHeatmapLabel}
      >
        {/* Header row */}
        <div role="row" style={{ display: 'contents' }}>
          <div role="columnheader" aria-hidden="true" />
          {DAY_LABELS_SHORT.map((day, i) => (
            <div
              key={day}
              role="columnheader"
              aria-label={DAY_LABELS[i]}
              className="pb-4 text-center text-micro font-medium text-text-tertiary sm:pb-5"
            >
              {day}
            </div>
          ))}
        </div>

        {/* Data rows */}
        {HOURS.map((hour) => (
          <DesktopHeatmapRow key={hour} hour={hour} dataMap={dataMap} />
        ))}
      </div>
    </div>
  );
}
