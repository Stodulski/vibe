import { PageHeaderSkeleton } from '@/shared/components/common/PageHeaderSkeleton';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { BOOKINGS_ACTIONS_CLASS } from './bookingsLayout';

/** Loading stand-in for `BookingsPageHeader`: the mobile title and the create / block buttons. */
export function BookingsPageHeaderSkeleton() {
  return (
    <PageHeaderSkeleton
      titleWidth="w-28"
      actions={
        <div className={BOOKINGS_ACTIONS_CLASS}>
          <Skeleton className="h-11 flex-1 rounded-xl sm:w-28 sm:flex-none" />
          <Skeleton className="h-11 flex-1 rounded-xl sm:w-28 sm:flex-none" />
        </div>
      }
    />
  );
}
