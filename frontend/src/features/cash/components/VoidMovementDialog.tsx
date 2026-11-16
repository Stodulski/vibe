import { useState } from 'react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';
import { blankToUndefined } from '@/shared/lib/blankToUndefined';
import { useVoidCashMovement } from '../hooks/useVoidCashMovement';
import type { CashMovement } from '@/shared/types/api.types';

const t = ES_AR;

interface VoidMovementDialogProps {
  movement: CashMovement | null;
  onClose: () => void;
  complexId: string;
  sessionId: string;
}

/**
 * Same "AlertDialog + optional Textarea" shape as `CancelBookingModal` — a
 * plain confirm/cancel would be enough functionally, but "Anular" is a real
 * ledger entry (an opposite-kind movement, not a delete) and a reason is
 * worth capturing the same way a booking cancellation's is.
 */
export function VoidMovementDialog({ movement, onClose, complexId, sessionId }: VoidMovementDialogProps) {
  const [note, setNote] = useState('');
  const voidMovement = useVoidCashMovement(complexId, sessionId);

  // Reset the note when a different (or no) movement is targeted — same
  // "adjust state during render on prop change" pattern as `CancelBookingModal`.
  const [lastMovementId, setLastMovementId] = useState(movement?.id);
  if (movement?.id !== lastMovementId) {
    setLastMovementId(movement?.id);
    if (movement) setNote('');
  }

  const open = !!movement;

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        if (!movement) return;
        voidMovement.mutate({ movementId: movement.id, note: blankToUndefined(note) }, { onSuccess: onClose });
      }}
      title={t.cash.voidConfirmTitle}
      description={t.cash.voidConfirmDescription}
      confirmLabel={t.cash.voidAction}
      // "Cancelar" next to "Anular" would read as the same action.
      cancelLabel={t.common.back}
      variant="destructive"
      isLoading={voidMovement.isPending}
    >
      <div className="space-y-2">
        <Label htmlFor="void-movement-note">{`${t.cash.movementNote} (${t.common.optional})`}</Label>
        <Textarea
          id="void-movement-note"
          value={note}
          onChange={(e) => {
            setNote(e.target.value);
          }}
          placeholder={t.cash.voidNotePlaceholder}
          maxLength={500}
        />
      </div>
    </ConfirmDialog>
  );
}
