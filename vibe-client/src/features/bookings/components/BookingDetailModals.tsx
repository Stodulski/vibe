import { BookingDetail } from './BookingDetail';
import { CancelBookingModal } from './CancelBookingModal';
import { ConfirmPaymentModal } from './ConfirmPaymentModal';
import { ManualRefundDialog } from './ManualRefundDialog';
import type { Booking, Client } from '@/shared/types/api.types';
import type { ConfirmPaymentDto } from '../schemas/booking.schemas';

// Structural shape shared by any "open a booking's detail sheet from
// elsewhere in the app" hook (dashboard's `useDashboardBookingDetail`,
// the client drawer's `useClientBookingDetail`, ...) — every one of them is
// just `useBookingModals` + `useBookingActions` spread together, so a single
// modals component works for all of them without importing across features.
interface BookingDetailState {
  detailOpen: boolean;
  setDetailOpen: (open: boolean) => void;
  selectedBooking: Booking | null;
  handleCancelBooking: (booking: Booking) => void;
  handleConfirmPaymentOpen: (booking: Booking) => void;
  handleNoShow: (booking: Booking) => void;
  cancelOpen: boolean;
  setCancelOpen: (open: boolean) => void;
  handleConfirmCancel: (reason?: string) => void;
  cancelBooking: { isPending: boolean };
  cancelBookingInfo:
    | {
        court_name: string;
        date: string;
        /** RFC3339, Argentina offset — rendered together as one range. */
        starts_at: string;
        ends_at: string;
        client_name: string;
      }
    | undefined;
  paymentOpen: boolean;
  setPaymentOpen: (open: boolean) => void;
  handlePaymentSubmit: (data: ConfirmPaymentDto) => void;
  confirmPayment: { isPending: boolean };
  manualRefundOpen: boolean;
  setManualRefundOpen: (open: boolean) => void;
  manualRefundAmount: number;
  handleConfirmManualRefund: () => void;
  markManualRefund: { isPending: boolean };
  handleManualRefundOpen: (booking: Booking, amount: number) => void;
}

interface BookingDetailModalsProps {
  detail: BookingDetailState;
  complexId: string | null;
  /** Set when these modals open on top of a client's own drawer, so the
   *  booking sheet doesn't repeat the client it was opened from. */
  hideClient?: boolean | undefined;
  /** Opens the client's own drawer from the booking sheet's client row. */
  onOpenClient?: ((client: Client) => void) | undefined;
}

export function BookingDetailModals({ detail, complexId, hideClient, onOpenClient }: BookingDetailModalsProps) {
  return (
    <>
      <BookingDetail
        open={detail.detailOpen}
        onClose={() => {
          detail.setDetailOpen(false);
        }}
        booking={detail.selectedBooking}
        complexId={complexId ?? ''}
        onCancel={detail.handleCancelBooking}
        onConfirmPayment={detail.handleConfirmPaymentOpen}
        onNoShow={detail.handleNoShow}
        onManualRefund={detail.handleManualRefundOpen}
        hideClient={hideClient}
        onOpenClient={onOpenClient}
      />
      <CancelBookingModal
        open={detail.cancelOpen}
        onClose={() => {
          detail.setCancelOpen(false);
        }}
        onConfirm={detail.handleConfirmCancel}
        isLoading={detail.cancelBooking.isPending}
        booking={detail.cancelBookingInfo}
      />
      <ConfirmPaymentModal
        open={detail.paymentOpen}
        onClose={() => {
          detail.setPaymentOpen(false);
        }}
        onConfirm={detail.handlePaymentSubmit}
        booking={detail.selectedBooking}
        isLoading={detail.confirmPayment.isPending}
      />
      <ManualRefundDialog
        open={detail.manualRefundOpen}
        onClose={() => {
          detail.setManualRefundOpen(false);
        }}
        onConfirm={detail.handleConfirmManualRefund}
        isLoading={detail.markManualRefund.isPending}
        amount={detail.manualRefundAmount}
      />
    </>
  );
}
