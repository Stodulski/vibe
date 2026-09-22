import { MovementFormDialog } from '../../components/MovementFormDialog';
import { VoidMovementDialog } from '../../components/VoidMovementDialog';
import type { CashMovement } from '@/shared/types/api.types';

interface OpenCashDialogsProps {
  complexId: string;
  sessionId: string;
  movementDialog: 'income' | 'expense' | null;
  onCloseMovementDialog: () => void;
  voidTarget: CashMovement | null;
  onClearVoidTarget: () => void;
}

/**
 * Every dialog the open-session view itself can show — grouped out of
 * `OpenCashView` so its own body stays a summary + a list. The close dialog
 * is deliberately NOT here: it is rendered by `CashPageBody`, one level above
 * the open/closed view switch, so it survives the switch instead of being
 * unmounted mid-result (see `useCashPage`'s `closeTarget` doc comment).
 */
export function OpenCashDialogs({
  complexId,
  sessionId,
  movementDialog,
  onCloseMovementDialog,
  voidTarget,
  onClearVoidTarget,
}: OpenCashDialogsProps) {
  return (
    <>
      <MovementFormDialog
        open={movementDialog === 'income'}
        onClose={onCloseMovementDialog}
        complexId={complexId}
        sessionId={sessionId}
        kind="income"
      />
      <MovementFormDialog
        open={movementDialog === 'expense'}
        onClose={onCloseMovementDialog}
        complexId={complexId}
        sessionId={sessionId}
        kind="expense"
      />
      <VoidMovementDialog
        movement={voidTarget}
        onClose={onClearVoidTarget}
        complexId={complexId}
        sessionId={sessionId}
      />
    </>
  );
}
