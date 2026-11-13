import { Skeleton } from '@/shared/components/ui/skeleton';
import { TodayBookingsShell } from './TodayBookingsShell';

/** Loading shape of `TodayBookings`: heading, then four booking rows. Floor is the measured loaded height. */
export function TodayBookingsSkeleton() {
  return (
    <TodayBookingsShell className="min-h-[22rem] gap-2 lg:min-h-[368px]">
      <Skeleton className="mb-1 h-5 w-40 rounded-lg" />
      {Array.from({ length: 4 }, (_, i) => (
        <Skeleton key={i} className="h-14 w-full rounded-xl" />
      ))}
    </TodayBookingsShell>
  );
}
