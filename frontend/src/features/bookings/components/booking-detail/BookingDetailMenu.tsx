import { MoreHorizontal, UserX, UserCheck, Ban, CalendarX } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/shared/components/ui/dropdown-menu';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { BookingSecondaryActionProps } from './secondaryActions';

const t = ES_AR;

export function BookingDetailMenu({
  booking,
  canMarkNoShow,
  canMarkAttended,
  canCancel,
  onNoShow,
  onAttended,
  onCancel,
  isSeries = false,
  onEndSeries,
}: BookingSecondaryActionProps) {
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
        {canMarkAttended && (
          <DropdownMenuItem
            onClick={() => {
              onAttended(booking);
            }}
          >
            <UserCheck className="size-3.5" />
            {t.bookings.statuses.completed}
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
