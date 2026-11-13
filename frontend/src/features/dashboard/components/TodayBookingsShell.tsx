import type { ComponentProps } from 'react';
import { Panel } from '@/shared/components/common/Panel';
import { cn } from '@/shared/lib/utils';

/** The panel container of `TodayBookings` and its skeleton: same Panel recipe, so they cannot drift. */
export function TodayBookingsShell({ className, ...props }: ComponentProps<typeof Panel>) {
  return <Panel as="section" size="sm" className={cn('flex h-full flex-col py-4 sm:p-4', className)} {...props} />;
}
