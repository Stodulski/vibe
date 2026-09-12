import { DetailSheetHeader } from '@/shared/components/common/DetailSheetHeader';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingDetailMenu } from './BookingDetailMenu';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

export function BookingDetailSheetHeader({
  booking,
  canMarkNoShow,
  canCancel,
  onNoShow,
  onCancel,
}: {
  booking: Booking;
  canMarkNoShow: boolean;
  canCancel: boolean;
  onNoShow: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
}) {
  return (
    <DetailSheetHeader
      title={t.publicBooking.bookingRef}
      subtitle={<span className="text-xs font-semibold tracking-wider uppercase">{t.bookings.bookingDetail}</span>}
      menu={
        (canMarkNoShow || canCancel) && (
          <BookingDetailMenu
            booking={booking}
            canMarkNoShow={canMarkNoShow}
            canCancel={canCancel}
            onNoShow={onNoShow}
            onCancel={onCancel}
          />
        )
      }
    />
  );
}
