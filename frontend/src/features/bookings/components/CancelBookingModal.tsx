import { useState } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { ES_AR } from '@/shared/i18n/es_AR';
import { BookingSummaryCard } from './cancel-booking-modal/BookingSummaryCard';

const t = ES_AR;

interface CancelBookingModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: (reason?: string) => void;
  isLoading: boolean;
  booking?:
    | {
        court_name: string;
        date: string;
        /** RFC3339, Argentina offset — rendered together as one range. */
        starts_at: string;
        ends_at: string;
        client_name: string;
      }
    | undefined;
}

export function CancelBookingModal({ open, onClose, onConfirm, isLoading, booking }: CancelBookingModalProps) {
  const [reason, setReason] = useState('');
  // A successful cancel clears the selected booking in the same tick it closes
  // this dialog, so the summary card blinked out while the dialog was still
  // fading. Keep showing what the reader just confirmed.
  const shown = useExitingValue(booking);

  // The dialog's content stays mounted between opens (Radix only unmounts the
  // exit animation, not the component), so a plain `useState('')` would keep
  // showing the previous booking's reason on reopen. Reset it during render
  // when `open` flips to true — the same "adjust state during render on prop
  // change" pattern `useExitingValue` uses above, so no stale frame ever
  // paints and no effect is needed.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setReason('');
  }

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        onConfirm(reason.trim() || undefined);
      }}
      title={t.bookings.cancel}
      description={t.bookings.cancelConfirm}
      confirmLabel={t.bookings.confirmCancel}
      // "Cancelar" next to a primary that is itself a cancellation would read
      // as the same action.
      cancelLabel={t.common.back}
      variant="destructive"
      isLoading={isLoading}
    >
      {shown && <BookingSummaryCard booking={shown} />}

      <div className="space-y-2">
        <Label htmlFor="cancel-reason">{`${t.bookings.cancelReason} (${t.common.optional})`}</Label>
        <Textarea
          id="cancel-reason"
          value={reason}
          onChange={(e) => {
            setReason(e.target.value);
          }}
          placeholder={t.bookings.cancelReasonPlaceholder}
          maxLength={500}
        />
      </div>
    </ConfirmDialog>
  );
}
