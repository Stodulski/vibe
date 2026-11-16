import { useState } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/shared/components/common/AppDialog';
import { FormField } from '@/shared/components/common/FormField';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useMovePair } from '../hooks/useTournamentChanges';
import { drawErrorMessage } from '../lib/tournamentErrors';
import { pairName } from '../lib/tournamentView';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.pairs;

interface SwapPairDialogProps {
  open: boolean;
  onClose: () => void;
  complexId: string;
  tournamentId: string;
  categoryId: string;
  /** The pair being moved; null while closed. */
  pair: TournamentOwnerPair | null;
  /** Every pair of the category, drawn or not. */
  pairs: TournamentOwnerPair[];
  /** In a groups category two pairs of the same group cannot trade places. */
  needsOtherGroup: boolean;
}

/** Trades the places of two drawn pairs (bracket positions or groups) before any result is loaded. Mounted only while open. */
export function SwapPairDialog({ open, onClose, pair, ...rest }: SwapPairDialogProps) {
  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {open && pair && <SwapBody pair={pair} onClose={onClose} {...rest} />}
      </DialogContent>
    </Dialog>
  );
}

type BodyProps = Omit<SwapPairDialogProps, 'open' | 'pair'> & { pair: TournamentOwnerPair };

function SwapBody({ onClose, complexId, tournamentId, categoryId, pair, pairs, needsOtherGroup }: BodyProps) {
  const [withId, setWithId] = useState('');
  const move = useMovePair(complexId, tournamentId, categoryId);
  const options = pairs.filter(
    (other) =>
      other.id !== pair.id &&
      other.seed !== null &&
      other.status === 'active' &&
      (!needsOtherGroup || other.group_id !== pair.group_id),
  );

  const confirm = () => {
    move.mutate(
      { pairId: pair.id, withPairId: withId },
      {
        onSuccess: onClose,
        onError: (error) => {
          toast.error(t.swapError, { description: drawErrorMessage(error) });
        },
      },
    );
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t.swapTitle}</DialogTitle>
        <DialogDescription>{t.swapDescription(pairName(pair))}</DialogDescription>
      </DialogHeader>
      <FormField label={t.swapWith} htmlFor="swap-with">
        <Select value={withId} onValueChange={setWithId}>
          <SelectTrigger id="swap-with">
            <SelectValue placeholder={t.swapPlaceholder} />
          </SelectTrigger>
          <SelectContent>
            {options.map((other) => (
              <SelectItem key={other.id} value={other.id}>
                {pairName(other)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
      {needsOtherGroup && <p className="text-text-tertiary text-xs">{t.swapGroupHint}</p>}
      <SectionFooter
        onCancel={onClose}
        cancelDisabled={move.isPending}
        submitLabel={t.swapConfirm}
        pending={move.isPending}
        submitDisabled={!withId}
        onSubmit={confirm}
      />
    </>
  );
}
