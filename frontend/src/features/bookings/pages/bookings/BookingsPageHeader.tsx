import { Plus, Ban } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { usePageHeading } from '@/shared/components/layout/page-heading/usePageHeading';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface BookingsPageHeaderProps {
  isPast: boolean;
  hasCourts: boolean;
  onBlockSlot: () => void;
  onCreateBooking: () => void;
}

/**
 * Two action buttons (block + create) — doesn't fit the shared `PageHeader`'s
 * single-action shape, so it follows the same md:hidden-title convention by
 * hand instead: on desktop the title moves to the masthead via
 * `usePageHeading`, same as every other page.
 */
export function BookingsPageHeader({ isPast, hasCourts, onBlockSlot, onCreateBooking }: BookingsPageHeaderProps) {
  usePageHeading(t.bookings.title);

  return (
    <div className="mb-3 flex flex-col gap-2.5 sm:mb-5 sm:flex-row sm:items-center sm:justify-between">
      <h1 className="font-display text-text-primary truncate text-xl leading-tight font-semibold tracking-tight sm:text-2xl md:hidden">
        {t.bookings.title}
      </h1>
      {/* Most important action first, so it's what the eye reaches first
          reading left-to-right (and top-down once these stack on a phone). */}
      <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row md:ml-auto">
        <Button onClick={onCreateBooking} disabled={isPast} className="flex-1 gap-2 rounded-xl sm:flex-none">
          <Plus className="size-4" />
          {t.bookings.create}
        </Button>
        <Button
          variant="outline"
          onClick={onBlockSlot}
          disabled={isPast || !hasCourts}
          className="flex-1 gap-2 rounded-xl sm:flex-none"
        >
          <Ban className="size-4" />
          {t.courts.block}
        </Button>
      </div>
    </div>
  );
}
