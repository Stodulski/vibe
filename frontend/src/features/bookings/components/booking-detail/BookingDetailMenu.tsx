import { MoreHorizontal, UserX, Ban, CalendarX } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

export function BookingDetailMenu({
  booking,
  canMarkNoShow,
  canCancel,
  onNoShow,
  onCancel,
  isSeries = false,
  onEndSeries,
}: {
  booking: Booking;
  canMarkNoShow: boolean;
  canCancel: boolean;
  onNoShow: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  /** The booking is one occurrence of a fixed weekly series: cancelling only covers this date. */
  isSeries?: boolean | undefined;
  /** Present when the series can still be ended from this booking's date. */
  onEndSeries?: (() => void) | undefined;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={t.common.rowActionsLabel}>
          <MoreHorizontal className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {canMarkNoShow && (
          <DropdownMenuItem
            onClick={() => {
              onNoShow(booking);
            }}
          >
            <UserX className="size-3.5" />
            {t.bookings.statuses.no_show}
          </DropdownMenuItem>
        )}
        {canCancel && (
          <DropdownMenuItem
            variant="destructive"
            onClick={() => {
              onCancel(booking);
            }}
          >
            <Ban className="size-3.5" />
            {isSeries ? t.bookings.series.cancelOne : t.bookings.cancel}
          </DropdownMenuItem>
        )}
        {onEndSeries && (
          <DropdownMenuItem variant="destructive" onClick={onEndSeries}>
            <CalendarX className="size-3.5" />
            {t.bookings.series.endFromHere}
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
