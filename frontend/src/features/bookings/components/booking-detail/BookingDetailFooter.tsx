import { DetailSheetFooter } from '@/shared/components/common/DetailSheetFooter';
import { BookingSecondaryActions } from './BookingSecondaryActions';
import { showsSecondaryButtons } from './secondaryActions';
import { DetailFooterActions } from './DetailFooterActions';
import type { Booking } from '@/shared/types/api.types';

export function BookingDetailFooter({
  booking,
  canMarkNoShow,
  canCancel,
  onNoShow,
  onCancel,
  isSeries,
  onEndSeries,
  canConfirmPayment,
  canMarkManualRefund,
  manualRefundAmount,
  onConfirmPayment,
  onManualRefund,
}: {
  booking: Booking;
  canMarkNoShow: boolean;
  canCancel: boolean;
  onNoShow: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  isSeries: boolean;
  onEndSeries: (() => void) | undefined;
  canConfirmPayment: boolean;
  canMarkManualRefund: boolean;
  manualRefundAmount: number;
  onConfirmPayment: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
}) {
  const hasPrimary = canConfirmPayment || canMarkManualRefund;
  const showSecondary = showsSecondaryButtons({ canMarkNoShow, canCancel, onEndSeries });

  return (
    <DetailSheetFooter
      secondary={
        showSecondary && (
          <BookingSecondaryActions
            booking={booking}
            canMarkNoShow={canMarkNoShow}
            canCancel={canCancel}
            onNoShow={onNoShow}
            onCancel={onCancel}
            isSeries={isSeries}
            onEndSeries={onEndSeries}
          />
        )
      }
    >
      {hasPrimary && (
        <DetailFooterActions
          booking={booking}
          onConfirmPayment={canConfirmPayment ? onConfirmPayment : undefined}
          onManualRefund={canMarkManualRefund ? onManualRefund : undefined}
          manualRefundAmount={manualRefundAmount}
        />
      )}
    </DetailSheetFooter>
  );
}
