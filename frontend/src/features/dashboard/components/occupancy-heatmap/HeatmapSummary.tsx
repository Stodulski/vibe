import { ChevronDown } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { cn } from '@/shared/lib/utils';

const t = ES_AR;

interface HeatmapSummaryProps {
  isLoading: boolean;
  avgOccupancy: number;
  peakDay: string | null;
  peakHour: string | null;
  expanded: boolean;
  onToggle: () => void;
}

export function HeatmapSummary({
  isLoading,
  avgOccupancy,
  peakDay,
  peakHour,
  expanded,
  onToggle,
}: HeatmapSummaryProps) {
  return (
    <>
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <div className="min-w-0">
            <h3 className="text-text-primary text-sm font-semibold">{t.dashboard.occupancy}</h3>
            {isLoading ? (
              <Skeleton className="mt-1 h-3 w-32" />
            ) : (
              <p className="text-text-tertiary truncate text-xs whitespace-nowrap">
                Promedio <span className="text-text-secondary font-medium">{avgOccupancy}%</span>
                {peakDay && peakHour && (
                  <span className="hidden sm:inline">
                    {' · '}Pico:{' '}
                    <span className="text-text-secondary font-medium">
                      {peakDay} {peakHour}
                    </span>
                  </span>
                )}
              </p>
            )}
          </div>
        </div>
        <Button
          variant="ghost"
          size="sm"
          className="text-primary-400 hover:text-primary-300 min-h-0 shrink-0 gap-1.5 text-xs"
          onClick={onToggle}
          aria-expanded={expanded}
          aria-controls="occupancy-heatmap-panel"
        >
          <span className="hidden sm:inline">{expanded ? 'Ocultar' : 'Ver mapa'}</span>
          <span className="sr-only sm:hidden">{expanded ? 'Ocultar' : 'Ver mapa'}</span>
          <ChevronDown className={cn('size-3.5 transition-transform duration-200', expanded && 'rotate-180')} />
        </Button>
      </div>

      {!isLoading && peakDay && peakHour && (
        <p className="text-text-tertiary mt-1.5 text-xs sm:hidden">
          Pico:{' '}
          <span className="text-text-secondary font-medium">
            {peakDay} {peakHour}
          </span>
        </p>
      )}
    </>
  );
}
