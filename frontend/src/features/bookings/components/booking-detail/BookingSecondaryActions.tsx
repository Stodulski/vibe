import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { showsSecondaryButtons, type BookingSecondaryActionFlags } from './secondaryActions';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * The secondary actions as footer buttons: the destructive ones first (they sit
 * at the left of the footer), "No se presentó" after them. Renders nothing when
 * there is none or when they belong in the menu.
 */
export function BookingSecondaryActions({
  booking,
  canMarkNoShow,
  canCancel,
  onNoShow,
  onCancel,
  isSeries,
  onEndSeries,
}: BookingSecondaryActionFlags & {
  booking: Booking;
  onNoShow: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  isSeries: boolean;
}) {
  if (!showsSecondaryButtons({ canMarkNoShow, canCancel, onEndSeries })) return null;

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:gap-3 [&>button]:w-full sm:[&>button]:w-auto">
      {canCancel && (
        <Button
          type="button"
          variant="destructive-outline"
          onClick={() => {
            onCancel(booking);
          }}
        >
          {isSeries ? t.bookings.series.cancelOne : t.bookings.cancel}
        </Button>
      )}
      {onEndSeries && (
        <Button type="button" variant="destructive-outline" onClick={onEndSeries}>
          {t.bookings.series.endFromHere}
        </Button>
      )}
      {canMarkNoShow && (
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            onNoShow(booking);
          }}
        >
          {t.bookings.statuses.no_show}
        </Button>
      )}
    </div>
  );
}
