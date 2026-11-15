import { DetailSheetFooter } from '@/shared/components/common/DetailSheetFooter';
import { BookingSecondaryActions } from './BookingSecondaryActions';
import { showsSecondaryButtons, type BookingSecondaryActionProps } from './secondaryActions';
import { DetailFooterActions } from './DetailFooterActions';
import type { Booking } from '@/shared/types/api.types';

interface BookingDetailFooterProps extends BookingSecondaryActionProps {
  canConfirmPayment: boolean;
  canMarkManualRefund: boolean;
  manualRefundAmount: number;
  onConfirmPayment: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
}

export function BookingDetailFooter({
  booking,
  canMarkNoShow,
  canMarkAttended,
  canCancel,
  onNoShow,
  onAttended,
  onCancel,
  isSeries,
  onEndSeries,
  canConfirmPayment,
  canMarkManualRefund,
  manualRefundAmount,
  onConfirmPayment,
  onManualRefund,
}: BookingDetailFooterProps) {
  const hasPrimary = canConfirmPayment || canMarkManualRefund;
  const showSecondary = showsSecondaryButtons({ canMarkNoShow, canMarkAttended, canCancel, onEndSeries });

  return (
    <DetailSheetFooter
      secondary={
        showSecondary && (
          <BookingSecondaryActions
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
