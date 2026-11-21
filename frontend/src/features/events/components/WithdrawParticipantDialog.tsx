import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useWithdrawParticipant } from '../hooks/useWithdrawParticipant';
import type { OpenPlayParticipant } from '@/shared/types/api.types';

const t = ES_AR.events.withdraw;

interface WithdrawParticipantDialogProps {
  onClose: () => void;
  complexId: string;
  eventId: string;
  /** The participant to take out; the dialog is open while one is set. */
  participant: OpenPlayParticipant | null;
}

/**
 * Confirms taking one participant out of the event. The description says what
 * it sets off: the spot frees up, a paid participant gets the whole payment
 * back through Mercado Pago, and the person is told by email.
 */
export function WithdrawParticipantDialog({
  onClose,
  complexId,
  eventId,
  participant,
}: WithdrawParticipantDialogProps) {
  const withdraw = useWithdrawParticipant(complexId, eventId);
  const name = participant ? `${participant.first_name} ${participant.last_name}` : '';

  return (
    <ConfirmDialog
      open={!!participant}
      onClose={onClose}
      onConfirm={() => {
        if (participant) withdraw.mutate(participant.id, { onSuccess: onClose });
      }}
      title={t.confirmTitle(name)}
      description={participant?.status === 'confirmed' ? t.confirmDescriptionPaid : t.confirmDescriptionUnpaid}
      confirmLabel={t.confirmLabel}
      cancelLabel={t.back}
      variant="destructive"
      isLoading={withdraw.isPending}
    />
  );
}
