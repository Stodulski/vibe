import type { ReactNode } from 'react';
import { Sheet, SheetContent, SheetFooter } from '@/shared/components/ui/sheet';
import { useOutsideClickGrace } from '@/shared/hooks/useOutsideClickGrace';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { useBooking } from '../hooks/useBooking';
import { BookingDetailSheetHeader } from './booking-detail/BookingDetailSheetHeader';
import { BookingInfoSection } from './booking-detail/BookingInfoSection';
import { ClientInfoRow } from './booking-detail/ClientInfoRow';
import { NotesSection } from './booking-detail/NotesSection';
import { DetailFooterActions } from './booking-detail/DetailFooterActions';
import { manualRefundOwed } from '../lib/manualRefundOwed';
import type { Booking, Client, Payment } from '@/shared/types/api.types';

function getBookingDetailFlags(b: Booking, payments: Payment[]) {
  const isCancelled = b.status === 'cancelled';
  const isCompleted = b.status === 'completed';
  const isNoShow = b.status === 'no_show';
  const canCancel = !isCancelled && !isCompleted && !isNoShow;
  const canMarkNoShow = b.status === 'confirmed' || b.status === 'completed';
  const canConfirmPayment = b.collection_status !== 'fully_paid' && b.refund_status === 'none' && !isCancelled;
  const canMarkManualRefund = isCancelled && b.refund_status === 'partial';
  const manualRefundAmount = canMarkManualRefund ? manualRefundOwed(payments) : 0;

  return { canCancel, canMarkNoShow, canConfirmPayment, canMarkManualRefund, manualRefundAmount };
}

/** The detail query plus the "keep showing the last thing while it exits"
 *  and "fall back to the list's own booking" adjustments layered on top of it,
 *  pulled out so `BookingDetail` itself reads as the sheet, not the fetch. */
function useBookingDetailState(complexId: string, open: boolean, booking: Booking | null) {
  const {
    data: detail,
    isPending: isDetailPending,
    isError: isDetailError,
    refetch: refetchDetail,
  } = useBooking(complexId, open && booking ? booking.id : null);

  // Retained across the exit: cancelling clears the selected booking in the
  // same tick it closes the sheet, and the detail query goes disabled with it,
  // so both sources drop to nothing at once and the sheet used to disappear
  // mid-slide instead of animating away.
  const b = useExitingValue(detail?.booking ?? booking);

  return {
    b,
    client: detail?.client,
    payments: detail?.payments ?? [],
    isDetailPending,
    isDetailError,
    refetchDetail,
  };
}

function BookingDetailFooter({
  booking,
  canConfirmPayment,
  canMarkManualRefund,
  manualRefundAmount,
  onConfirmPayment,
  onManualRefund,
}: {
  booking: Booking;
  canConfirmPayment: boolean;
  canMarkManualRefund: boolean;
  manualRefundAmount: number;
  onConfirmPayment: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
}) {
  if (!canConfirmPayment && !canMarkManualRefund) return null;

  return (
    <SheetFooter>
      <DetailFooterActions
        booking={booking}
        onConfirmPayment={canConfirmPayment ? onConfirmPayment : undefined}
        onManualRefund={canMarkManualRefund ? onManualRefund : undefined}
        manualRefundAmount={manualRefundAmount}
      />
    </SheetFooter>
  );
}

interface BookingDetailProps {
  open: boolean;
  onClose: () => void;
  booking: Booking | null;
  complexId: string;
  onCancel: (booking: Booking) => void;
  onConfirmPayment: (booking: Booking) => void;
  onNoShow: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
  /** Hides the client row — the drawer underneath is already that client. */
  hideClient?: boolean | undefined;
  /** Opens the client's own drawer from the client row. */
  onOpenClient?: ((client: Client) => void) | undefined;
}

interface BookingDetailBodyProps {
  booking: Booking;
  clientRow: ReactNode;
  payments: Payment[];
  isDetailPending: boolean;
  isDetailError: boolean;
  refetchDetail: () => unknown;
  canMarkNoShow: boolean;
  canCancel: boolean;
  onNoShow: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  canConfirmPayment: boolean;
  canMarkManualRefund: boolean;
  manualRefundAmount: number;
  onConfirmPayment: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
}

function BookingDetailBody({
  booking,
  clientRow,
  payments,
  isDetailPending,
  isDetailError,
  refetchDetail,
  canMarkNoShow,
  canCancel,
  onNoShow,
  onCancel,
  canConfirmPayment,
  canMarkManualRefund,
  manualRefundAmount,
  onConfirmPayment,
  onManualRefund,
}: BookingDetailBodyProps) {
  return (
    <>
      <BookingDetailSheetHeader
        booking={booking}
        canMarkNoShow={canMarkNoShow}
        canCancel={canCancel}
        onNoShow={onNoShow}
        onCancel={onCancel}
      />

      {/* Only the body scrolls: the header and the action footer stay put, and
          the bottom padding keeps the last row off the edge. */}
      <div className="min-h-0 flex-1 space-y-8 overflow-y-auto px-4 pt-4 pb-8 sm:px-6">
        <BookingInfoSection
          booking={booking}
          payments={payments}
          clientRow={clientRow}
          paymentsPending={isDetailPending}
          paymentsError={isDetailError}
          onRetryPayments={() => {
            void refetchDetail();
          }}
        />
        {booking.notes && <NotesSection notes={booking.notes} />}
      </div>

      <BookingDetailFooter
        booking={booking}
        canConfirmPayment={canConfirmPayment}
        canMarkManualRefund={canMarkManualRefund}
        manualRefundAmount={manualRefundAmount}
        onConfirmPayment={onConfirmPayment}
        onManualRefund={onManualRefund}
      />
    </>
  );
}

export function BookingDetail({
  open,
  onClose,
  booking,
  complexId,
  onCancel,
  onConfirmPayment,
  onNoShow,
  onManualRefund,
  hideClient,
  onOpenClient,
}: BookingDetailProps) {
  const { b, client, payments, isDetailPending, isDetailError, refetchDetail } = useBookingDetailState(
    complexId,
    open,
    booking,
  );
  const onPointerDownOutside = useOutsideClickGrace(open);

  if (!b) return null;

  const { canCancel, canMarkNoShow, canConfirmPayment, canMarkManualRefund, manualRefundAmount } =
    getBookingDetailFlags(b, payments);

  const clientRow = hideClient ? undefined : <ClientInfoRow booking={b} client={client} onOpenClient={onOpenClient} />;

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <SheetContent
        side="right"
        className="sm:max-w-lg"
        showCloseButton={false}
        onPointerDownOutside={onPointerDownOutside}
      >
        <BookingDetailBody
          booking={b}
          clientRow={clientRow}
          payments={payments}
          isDetailPending={isDetailPending}
          isDetailError={isDetailError}
          refetchDetail={refetchDetail}
          canMarkNoShow={canMarkNoShow}
          canCancel={canCancel}
          onNoShow={onNoShow}
          onCancel={onCancel}
          canConfirmPayment={canConfirmPayment}
          canMarkManualRefund={canMarkManualRefund}
          manualRefundAmount={manualRefundAmount}
          onConfirmPayment={onConfirmPayment}
          onManualRefund={onManualRefund}
        />
      </SheetContent>
    </Sheet>
  );
}
