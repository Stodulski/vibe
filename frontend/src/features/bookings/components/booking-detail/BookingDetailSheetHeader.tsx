import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatEventDay } from '@/shared/lib/eventDates';
import { BookingDetailMenu } from './BookingDetailMenu';
import { usesActionsMenu, type BookingSecondaryActionProps } from './secondaryActions';

const t = ES_AR;

export function BookingDetailSheetHeader({
  booking,
  canMarkNoShow,
  canMarkAttended,
  canCancel,
  onNoShow,
  onAttended,
  onCancel,
  isSeries,
  onEndSeries,
}: BookingSecondaryActionProps) {
  return (
    <DetailSheetHeader
      title={t.publicBooking.bookingRef}
      subtitle={
        <span className="inline-block first-letter:uppercase">{formatEventDay(booking.date.slice(0, 10))}</span>
      }
      menu={
        usesActionsMenu({ canMarkNoShow, canMarkAttended, canCancel, onEndSeries }) && (
          <BookingDetailMenu
            booking={booking}
            canMarkNoShow={canMarkNoShow}
            canMarkAttended={canMarkAttended}
            canCancel={canCancel}
            onNoShow={onNoShow}
            onAttended={onAttended}
            onCancel={onCancel}
            isSeries={isSeries}
            onEndSeries={onEndSeries}
          />
        )
      }
    />
  );
}
