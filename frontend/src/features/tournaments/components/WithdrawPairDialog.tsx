import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useWithdrawPair } from '../hooks/useWithdrawPair';
import { pairFullName } from '../lib/tournamentView';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.pairs;

interface WithdrawPairDialogProps {
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  categoryId: string;
  /** The category has its draw: the pair's remaining matches go to the rival. */
  drawn: boolean;
  /** The pair to take out; the dialog is open while one is set. */
  pair: TournamentOwnerPair | null;
}

/**
 * Confirms taking one pair out of the category, the way the category's cancel
 * confirms: the spot frees up, a paid pair gets the whole payment back through
 * Mercado Pago, and the payer is told by email.
 */
export function WithdrawPairDialog({
  onClose,
  complexId,
  tournamentId,
  categoryId,
  drawn,
  pair,
}: WithdrawPairDialogProps) {
  const withdraw = useWithdrawPair(complexId, tournamentId, categoryId);
  const money = pair?.registration.status === 'confirmed' ? t.withdrawDescriptionPaid : t.withdrawDescriptionUnpaid;

  return (
    <ConfirmDialog
      open={!!pair}
      onClose={onClose}
      onConfirm={() => {
        if (pair) withdraw.mutate(pair.registration.id, { onSuccess: onClose });
      }}
      title={t.withdrawTitle(pair ? pairFullName(pair) : '')}
      description={drawn ? `${money} ${t.withdrawDrawnNote}` : money}
      confirmLabel={t.withdrawConfirm}
      cancelLabel={t.withdrawBack}
      variant="destructive"
      isLoading={withdraw.isPending}
    />
  );
}
