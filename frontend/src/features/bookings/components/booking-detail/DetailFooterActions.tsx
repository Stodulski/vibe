import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

export function DetailFooterActions({
  booking,
  onConfirmPayment,
  onManualRefund,
  manualRefundAmount,
}: {
  booking: Booking;
  onConfirmPayment?: ((booking: Booking) => void) | undefined;
  onManualRefund?: ((booking: Booking, amount: number) => void) | undefined;
  manualRefundAmount?: number | undefined;
}) {
  if (onManualRefund) {
    return (
      <Button
        onClick={() => {
          onManualRefund(booking, manualRefundAmount ?? 0);
        }}
      >
        {t.bookings.markManualRefund}
      </Button>
    );
  }

  return (
    <Button
      onClick={() => {
        onConfirmPayment?.(booking);
      }}
    >
      {t.bookings.confirmPayment}
    </Button>
  );
}
