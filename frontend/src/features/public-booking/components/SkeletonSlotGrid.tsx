import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function SkeletonSlotGrid() {
  return (
    <div className="animate-fade-in space-y-8">
      <p className="text-text-tertiary text-center text-sm">{t.publicBooking.checkingAvailability}</p>
      {Array.from({ length: 2 }, (_, courtIdx) => (
        <div key={courtIdx}>
          <div className="mb-4 flex flex-wrap items-center gap-2 sm:gap-2.5">
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-5 w-12 rounded-full" />
            <Skeleton className="h-5 w-16 rounded-full" />
          </div>
          <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-5">
            {Array.from({ length: 10 }, (_, slotIdx) => (
              <Skeleton key={slotIdx} className="h-[52px] rounded-xl" />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
