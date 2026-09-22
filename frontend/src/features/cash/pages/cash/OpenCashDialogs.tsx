import { MovementFormDialog } from '../../components/MovementFormDialog';
import { CloseCashSessionDialog } from '../../components/CloseCashSessionDialog';
import { VoidMovementDialog } from '../../components/VoidMovementDialog';
import type { CashMovement } from '@/shared/types/api.types';

interface OpenCashDialogsProps {
  complexId: string;
  sessionId: string;
  expectedCash: number;
  movementDialog: 'income' | 'expense' | null;
  onCloseMovementDialog: () => void;
  closeDialogOpen: boolean;
  onCloseCloseDialog: () => void;
  voidTarget: CashMovement | null;
  onClearVoidTarget: () => void;
}

/** Every dialog the open-session view can show — grouped out of `OpenCashView` so its own body stays a summary + a list. */
export function OpenCashDialogs({
  complexId,
  sessionId,
  expectedCash,
  movementDialog,
  onCloseMovementDialog,
  closeDialogOpen,
  onCloseCloseDialog,
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
      <CloseCashSessionDialog
        open={closeDialogOpen}
        onClose={onCloseCloseDialog}
        complexId={complexId}
        sessionId={sessionId}
        expectedCash={expectedCash}
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
