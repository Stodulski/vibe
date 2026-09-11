import { BookingDetailModals, CreateBookingModal } from '@/features/bookings';
import { BlockSlotSection } from './BlockSlotSection';
import { ClientDrawerModals, useClientActions } from '@/features/clients';
import type { useBookingsPage } from './useBookingsPage';
import type { useBlockSlotSection } from './useBlockSlotSection';

// An explicit prop list instead of `state: ReturnType<typeof useBookingsPage>`
// — this component only ever reads these fields, so its interface says
// exactly that instead of silently tracking the page hook's entire shape
// (and a future test can pass just these, not a fabricated whole-hook
// object). `Pick` keeps every field's type in sync with the hook itself.
type BookingsPageState = Pick<
  ReturnType<typeof useBookingsPage>,
  | 'complex'
  | 'courts'
  | 'schedules'
  | 'createOpen'
  | 'setCreateOpen'
  | 'createPrefill'
  | 'detailOpen'
  | 'setDetailOpen'
  | 'selectedBooking'
  | 'handleCancelBooking'
  | 'handleConfirmPaymentOpen'
  | 'handleNoShow'
  | 'handleManualRefundOpen'
  | 'cancelOpen'
  | 'setCancelOpen'
  | 'handleConfirmCancel'
  | 'cancelBooking'
  | 'cancelBookingInfo'
  | 'paymentOpen'
  | 'setPaymentOpen'
  | 'handlePaymentSubmit'
  | 'confirmPayment'
  | 'manualRefundOpen'
  | 'setManualRefundOpen'
  | 'handleConfirmManualRefund'
  | 'markManualRefund'
  | 'manualRefundAmount'
>;

interface BookingsPageModalsProps {
  state: BookingsPageState;
  blockSlot: ReturnType<typeof useBlockSlotSection>;
  selectedComplexId: string;
}

export function BookingsPageModals({ state, blockSlot, selectedComplexId }: BookingsPageModalsProps) {
  // Opens a client's own drawer from the booking sheet's client row — a
  // separate `useClientActions` instance from /clients' own, scoped to this
  // page only (no dashboard cache to invalidate here).
  const clientDrawer = useClientActions(selectedComplexId);

  return (
    <>
      <CreateBookingModal
        open={state.createOpen}
        onClose={() => {
          state.setCreateOpen(false);
        }}
        complexId={selectedComplexId}
        courts={state.courts}
        depositPercentage={state.complex?.deposit_percentage ?? 0}
        prefill={state.createPrefill}
        schedules={state.schedules}
      />
      {/* The detail sheet and its cancel, payment and refund modals, the same
          composition the dashboard and the client drawer use. */}
      <BookingDetailModals
        detail={state}
        complexId={selectedComplexId}
        onOpenClient={clientDrawer.handleSelectClient}
      />
      <BlockSlotSection
        complexId={selectedComplexId}
        courts={state.courts}
        blockOpen={blockSlot.blockOpen}
        onBlockClose={() => {
          blockSlot.setBlockOpen(false);
        }}
        selectedSlot={blockSlot.selectedSlot}
        onSelectedSlotClose={() => {
          blockSlot.setSelectedSlot(null);
        }}
        onDeleteSlot={(slotId, onSuccess) => {
          blockSlot.deleteBlockedSlot.mutate(slotId, { onSuccess });
        }}
        isDeletingSlot={blockSlot.deleteBlockedSlot.isPending}
      />
      <ClientDrawerModals detail={clientDrawer} complexId={selectedComplexId} />
    </>
  );
}
