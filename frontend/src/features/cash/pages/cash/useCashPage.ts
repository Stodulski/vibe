import { useState } from 'react';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '../../hooks/useCashSession';
import { useCashSessionDetail } from '../../hooks/useCashSessionDetail';
import type { CashMovement } from '@/shared/types/api.types';

/**
 * All the `/cash` page's state: which complex, the currently open session (or
 * "closed"), its full detail (once its id is known — see `useCashSessionDetail`'s
 * own doc comment on why that's a second query), and every dialog's open/target
 * state.
 */
export function useCashPage() {
  const { selectedComplexId } = useSelectedComplex();
  const sessionQuery = useCashSession(selectedComplexId);
  const sessionId = sessionQuery.data?.cash_session.id ?? null;
  const detailQuery = useCashSessionDetail(selectedComplexId, sessionQuery.isClosed ? null : sessionId);

  const [openDialogOpen, setOpenDialogOpen] = useState(false);
  const [movementDialog, setMovementDialog] = useState<'income' | 'expense' | null>(null);
  const [closeDialogOpen, setCloseDialogOpen] = useState(false);
  const [voidTarget, setVoidTarget] = useState<CashMovement | null>(null);

  return {
    complexId: selectedComplexId,
    sessionQuery,
    detailQuery,
    openDialogOpen,
    setOpenDialogOpen,
    movementDialog,
    setMovementDialog,
    closeDialogOpen,
    setCloseDialogOpen,
    voidTarget,
    setVoidTarget,
  };
}
