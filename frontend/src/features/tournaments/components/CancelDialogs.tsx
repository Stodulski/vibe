import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useCancelCategory, useCancelTournament } from '../hooks/useCancelTournament';

const t = ES_AR.tournaments.cancel;

interface CancelDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  /** How many pairs already paid: the ones this refunds. */
  paidCount: number;
}

/**
 * Confirms cancelling the whole tournament. The description says what the
 * owner is about to set off: the courts free up and every paid pair is
 * refunded the whole payment through Mercado Pago and told by email.
 */
export function CancelTournamentDialog({
  open,
  onClose,
  complexId,
  tournamentId,
  paidCount,
}: CancelDialogProps & { tournamentId: string }) {
  const cancel = useCancelTournament(complexId);
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        cancel.mutate(tournamentId, { onSuccess: onClose });
      }}
      title={t.confirmTitle}
      description={paidCount > 0 ? t.confirmDescriptionPaid(paidCount) : t.confirmDescription}
      confirmLabel={t.confirmLabel}
      cancelLabel={t.back}
      variant="destructive"
      isLoading={cancel.isPending}
    />
  );
}

/** The same for one category: only its pairs are refunded and only its matches release their courts. */
export function CancelCategoryDialog({
  open,
  onClose,
  complexId,
  tournamentId,
  categoryId,
  categoryName,
  paidCount,
}: CancelDialogProps & { tournamentId: string; categoryId: string; categoryName: string }) {
  const cancel = useCancelCategory(complexId, tournamentId);
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        cancel.mutate(categoryId, { onSuccess: onClose });
      }}
      title={t.categoryTitle(categoryName)}
      description={paidCount > 0 ? t.categoryDescriptionPaid(paidCount) : t.categoryDescription}
      confirmLabel={t.categoryConfirmLabel}
      cancelLabel={t.back}
      variant="destructive"
      isLoading={cancel.isPending}
    />
  );
}
