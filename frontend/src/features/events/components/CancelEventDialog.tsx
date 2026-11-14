import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCancelEvent } from '../hooks/useCancelEvent';

const t = ES_AR.events.cancel;

interface CancelEventDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  eventId: string;
  /** How many people already paid: the ones this refunds. */
  paidCount: number;
}

/**
 * Confirms cancelling an event. The description says what the owner is about to
 * set off: the courts free up and every paid participant is refunded the whole
 * payment through Mercado Pago and told by email.
 */
export function CancelEventDialog({ open, onClose, complexId, eventId, paidCount }: CancelEventDialogProps) {
  const cancelEvent = useCancelEvent(complexId);

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        cancelEvent.mutate(eventId, { onSuccess: onClose });
      }}
      title={t.confirmTitle}
      description={paidCount > 0 ? t.confirmDescriptionPaid(paidCount) : t.confirmDescription}
      confirmLabel={t.confirmLabel}
      // "Cancelar" beside "Cancelar evento" would read as the same action.
      cancelLabel={t.back}
      variant="destructive"
      isLoading={cancelEvent.isPending}
    />
  );
}
