import { useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useOccupancyData } from '../hooks/useOccupancyData';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { Button } from '@/shared/components/ui/button';
import { TooltipProvider } from '@/shared/components/ui/tooltip';
import { useHeatmapData } from './occupancy-heatmap/useHeatmapData';
import { HeatmapSummary } from './occupancy-heatmap/HeatmapSummary';
import { DesktopHeatmap } from './occupancy-heatmap/DesktopHeatmap';
import { MobileHeatmap } from './occupancy-heatmap/MobileHeatmap';
import { HeatmapLegend } from './occupancy-heatmap/HeatmapLegend';
import { Panel } from '@/shared/components/common/Panel';

const t = ES_AR;

interface OccupancyHeatmapProps {
  complexId: string;
}

export function OccupancyHeatmap({ complexId }: OccupancyHeatmapProps) {
  const [expanded, setExpanded] = useState(true);
  const { data: occupancy, isLoading, isError, refetch } = useOccupancyData(complexId);
  const { dataMap, peakDay, peakHour, avgOccupancy } = useHeatmapData(occupancy);

  if (isError) {
    return (
      <Panel
        as="section"
        size="md"
        className="flex flex-col items-center justify-center gap-2 text-center sm:p-5"
        aria-label={t.dashboard.occupancy}
      >
        <AlertTriangle className="size-5 text-error-icon" aria-hidden="true" />
        <p className="text-sm text-text-tertiary">{t.common.loadError}</p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            void refetch();
          }}
        >
          {t.layout.retry}
        </Button>
      </Panel>
    );
  }

  return (
    <Panel as="section" size="md" className="sm:p-5" aria-label={t.dashboard.occupancy}>
      <HeatmapSummary
        isLoading={isLoading}
        avgOccupancy={avgOccupancy}
        peakDay={peakDay}
        peakHour={peakHour}
        expanded={expanded}
        onToggle={() => {
          setExpanded((v) => !v);
        }}
      />

      {/* Full heatmap — expandable */}
      {expanded && (
        <div id="occupancy-heatmap-panel" className="mt-5 animate-fade-in">
          {isLoading ? (
            <Skeleton className="h-[300px] w-full rounded-xl" />
          ) : (
            <TooltipProvider delayDuration={100}>
              <DesktopHeatmap dataMap={dataMap} />
              <MobileHeatmap dataMap={dataMap} />
              <HeatmapLegend />
            </TooltipProvider>
          )}
        </div>
      )}
    </Panel>
  );
}

export default OccupancyHeatmap;
