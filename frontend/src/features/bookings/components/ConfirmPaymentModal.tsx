import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { BookingSummary } from './confirm-payment-modal/BookingSummary';
import { PaymentTypeAndAmountFields } from './confirm-payment-modal/PaymentTypeAndAmountFields';
import { PaymentMethodField } from './confirm-payment-modal/PaymentMethodField';
import { useConfirmPaymentForm } from './confirm-payment-modal/useConfirmPaymentForm';
import type { ConfirmPaymentDto } from '../schemas/booking.schema';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

interface ConfirmPaymentModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: (data: ConfirmPaymentDto) => void;
  booking: Booking | null;
  isLoading: boolean;
}

/**
 * Mounted only while `open`, keyed by which booking it charges.
 *
 * The form used to stay mounted across every open/close cycle, with an
 * effect (deferred a microtask to dodge `react-hooks/set-state-in-effect`)
 * resetting it whenever `open`/`booking` changed. Remounting a fresh
 * instance per booking gets the right `defaultValues` from the start, the
 * same "key instead of a reset effect" pattern as `CourtForm`/`BlockSlotModal`.
 */
export function ConfirmPaymentModal({ open, onClose, onConfirm, booking, isLoading }: ConfirmPaymentModalProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && (
          <ConfirmPaymentModalBody
            key={booking?.id ?? 'none'}
            onClose={onClose}
            onConfirm={onConfirm}
            booking={booking}
            isLoading={isLoading}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ConfirmPaymentModalBody({
  onClose,
  onConfirm,
  booking,
  isLoading,
}: {
  onClose: () => void;
  onConfirm: (data: ConfirmPaymentDto) => void;
  booking: Booking | null;
  isLoading: boolean;
}) {
  const { handleSubmit, setValue, register, errors, isDepositPaid, remaining, paymentType, handlePaymentTypeChange } =
    useConfirmPaymentForm({ booking });

  return (
    <>
      <DialogHeader>
        <DialogTitle>{isDepositPaid ? t.bookings.confirmRemainingPayment : t.bookings.confirmPayment}</DialogTitle>
      </DialogHeader>

      {booking && <BookingSummary isDepositPaid={isDepositPaid} remaining={remaining} />}

      <form onSubmit={submitHandler(handleSubmit, onConfirm)} className="space-y-4">
        <input type="hidden" {...register('amount', { valueAsNumber: true })} />

        <PaymentTypeAndAmountFields
          isDepositPaid={isDepositPaid}
          paymentType={paymentType}
          onPaymentTypeChange={handlePaymentTypeChange}
          booking={booking}
          setValue={setValue}
          errors={errors}
        />

        <PaymentMethodField setValue={setValue} errors={errors} />

        <SectionFooter onCancel={onClose} submitLabel={t.bookings.confirmPayment} pending={isLoading} />
      </form>
    </>
  );
}
