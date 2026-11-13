import type { ComponentProps } from 'react';
import { Panel } from '@/shared/components/common/Panel';
import { cn } from '@/shared/lib/utils';

/** The panel container of `OccupancyHeatmap` and its skeleton: same Panel recipe, so they cannot drift. */
export function OccupancyHeatmapPanel({ className, ...props }: ComponentProps<typeof Panel>) {
  return <Panel as="section" size="md" className={cn('sm:p-5', className)} {...props} />;
}
