import { useState } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { MoneyPesosField } from '@/shared/components/common/MoneyPesosField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { centavosToPesos, pesosToCentavos } from '@/shared/lib/money';
import { categoryTitle } from '@/shared/lib/traitLabels';
import { getHttpErrorMessage } from '@/shared/lib/utils';
import { useSetCourtFee } from '../hooks/useTournamentChanges';
import { MAX_COURT_FEE } from '../schemas/tournamentForm.schema';
import type { TournamentOwnerCategory } from '@/shared/types/api.types';

const t = ES_AR.tournaments;

interface CourtFeeDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  category: TournamentOwnerCategory;
}

/**
 * Changes what each pair of the category pays per match at the complex: up when
 * the court gets dearer, down, or to zero for none. Charges already recorded keep
 * the amount they were made at. Mounted only while open, so every opening starts
 * from the current fee.
 */
export function CourtFeeDialog({ open, onClose, ...rest }: CourtFeeDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">{open && <CourtFeeBody onClose={onClose} {...rest} />}</DialogContent>
    </Dialog>
  );
}

function CourtFeeBody({ onClose, complexId, tournamentId, category }: Omit<CourtFeeDialogProps, 'open'>) {
  const [pesos, setPesos] = useState<number | undefined>(centavosToPesos(category.court_fee));
  const save = useSetCourtFee(complexId, tournamentId, category.id);
  const valid = pesos !== undefined && pesos >= 0 && pesos <= MAX_COURT_FEE;

  const submit = () => {
    if (pesos === undefined || !valid) return;
    save.mutate(pesosToCentavos(pesos), {
      onSuccess: () => {
        toast.success(t.courtFee.editSuccess);
        onClose();
      },
      onError: (error) => {
        toast.error(t.courtFee.editError, { description: getHttpErrorMessage(error, '') || undefined });
      },
    });
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.courtFee.editTitle}</DialogTitle>
        <DialogDescription>{t.courtFee.editDescription(categoryTitle(category))}</DialogDescription>
      </DialogHeader>
      <MoneyPesosField
        id="court-fee"
        label={t.courtFee.editLabel}
        value={pesos}
        onChange={setPesos}
        error={pesos !== undefined && !valid ? t.validation.courtFeeRange : undefined}
        placeholder="0"
      />
      <p className="text-text-tertiary text-xs">{t.courtFee.editHint}</p>
      <SectionFooter
        onCancel={onClose}
        cancelDisabled={save.isPending}
        submitLabel={t.courtFee.editSubmit}
        pending={save.isPending}
        submitDisabled={!valid}
        onSubmit={submit}
      />
    </>
  );
}
