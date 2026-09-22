import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import { AlertDialog, AlertDialogContent } from '@/shared/components/common/AppAlertDialog';
import {
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/shared/components/ui/alert-dialog';
import { Label } from '@/shared/components/ui/label';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useVoidCashMovement } from '../hooks/useVoidCashMovement';
import { blankNoteToUndefined } from '../lib/blankNoteToUndefined';
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
    <AlertDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t.cash.voidConfirmTitle}</AlertDialogTitle>
          <AlertDialogDescription>{t.cash.voidConfirmDescription}</AlertDialogDescription>
        </AlertDialogHeader>

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

        <AlertDialogFooter>
          <AlertDialogCancel disabled={voidMovement.isPending}>{t.common.cancel}</AlertDialogCancel>
          <AlertDialogAction
            onClick={(e) => {
              e.preventDefault();
              if (!movement) return;
              voidMovement.mutate(
                { movementId: movement.id, note: blankNoteToUndefined(note) },
                { onSuccess: onClose },
              );
            }}
            disabled={voidMovement.isPending}
            className="bg-destructive hover:bg-destructive/90 text-white"
          >
            {voidMovement.isPending && <Loader2 className="size-4 animate-spin" />}
            {t.cash.voidAction}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
