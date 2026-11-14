import { toast } from 'sonner';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { useUndoCourtFee, type CourtFeeTarget } from '../hooks/useTournamentChanges';
import { courtFeeErrorMessage } from '../lib/tournamentErrors';

const t = ES_AR.tournaments.courtFee;

interface UndoCourtFeeDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  /** The charge being undone; null while closed. */
  target: CourtFeeTarget | null;
  pairName: string;
  /** What the charge was made at. */
  amount: number;
}

/** Confirms taking back a court fee charge: it voids the income in the open cash session. */
export function UndoCourtFeeDialog({
  open,
  onClose,
  complexId,
  tournamentId,
  target,
  pairName,
  amount,
}: UndoCourtFeeDialogProps) {
  const undo = useUndoCourtFee(complexId, tournamentId);
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        if (!target) return;
        undo.mutate(target, {
          onSuccess: () => {
            toast.success(t.undoSuccess);
            onClose();
          },
          onError: (error) => {
            toast.error(t.errors.undoUnknown, { description: courtFeeErrorMessage(error, 'undo') });
          },
        });
      }}
      title={t.undoTitle}
      description={t.undoDescription(pairName, formatPrice(amount))}
      confirmLabel={t.undoConfirm}
      cancelLabel={ES_AR.tournaments.cancel.back}
      variant="destructive"
      isLoading={undo.isPending}
    />
  );
}
