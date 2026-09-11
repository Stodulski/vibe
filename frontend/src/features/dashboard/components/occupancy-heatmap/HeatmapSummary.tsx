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
        <div className="flex items-center gap-3 min-w-0">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-text-primary">{t.dashboard.occupancy}</h3>
            {isLoading ? (
              <Skeleton className="mt-1 h-3 w-32" />
            ) : (
              <p className="truncate whitespace-nowrap text-xs text-text-tertiary">
                Promedio <span className="font-medium text-text-secondary">{avgOccupancy}%</span>
                {peakDay && peakHour && (
                  <span className="hidden sm:inline">
                    {' · '}Pico:{' '}
                    <span className="font-medium text-text-secondary">
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
          className="shrink-0 gap-1.5 text-xs text-primary-400 hover:text-primary-300 min-h-0"
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
        <p className="mt-1.5 text-xs text-text-tertiary sm:hidden">
          Pico:{' '}
          <span className="font-medium text-text-secondary">
            {peakDay} {peakHour}
          </span>
        </p>
      )}
    </>
  );
}
