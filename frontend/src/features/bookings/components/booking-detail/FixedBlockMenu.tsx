import { MoreHorizontal, Ban, CalendarX } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The header menu of a fixed booking's date that has no booking yet: the same
 * trigger, items and destructive variant as `BookingDetailMenu`, so the two
 * sheets keep their cancellations in the same place.
 */
export function FixedBlockMenu({ onSkip, onEnd }: { onSkip: () => void; onEnd: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem variant="destructive" onClick={onSkip}>
          <Ban className="size-3.5" />
          {t.bookings.series.cancelOne}
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={onEnd}>
          <CalendarX className="size-3.5" />
          {t.bookings.series.endFromHere}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
