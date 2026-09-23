import { useState } from 'react';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useCashSessionDetail } from '../../hooks/useCashSessionDetail';
import type { CashMovement } from '@/shared/types/api.types';

/** The session and expected-cash figure a just-opened close dialog is working against — captured once, not read live (see `closeTarget` below). */
interface CloseTarget {
  sessionId: string;
  expectedCash: number;
}

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
  // `closeTarget` (not a plain boolean) captures the session id and expected
  // cash the close dialog is working against at the moment it opens, instead
  // of reading them live off `sessionQuery`/`detailQuery`. The close dialog is
  // rendered by `CashPageBody` itself (not `OpenCashView`) precisely so it
  // survives the session flipping to closed mid-dialog — closing invalidates
  // `cash.current`, and once that refetch lands `CashPageBody` swaps to
  // `ClosedCashView`. If the dialog's own data came from the live query, it
  // would go stale/undefined at that exact moment instead of continuing to
  // show the committed closed result (T3 review: "close result unmount").
  const [closeTarget, setCloseTarget] = useState<CloseTarget | null>(null);
  const [voidTarget, setVoidTarget] = useState<CashMovement | null>(null);

  // Defense in depth against the same leak: if a *different* session is open
  // by the time this renders (the previous close was dismissed without going
  // through `setCloseTarget(null)`, or the page re-mounted with stale local
  // state), drop the stale target rather than risk it reopening the close
  // dialog over a session it was never about. Adjusted during render off a
  // tracked previous value (same pattern as `VoidMovementDialog`'s
  // `lastMovementId`), not a `useEffect`, which would setState after commit
  // and cause a second, avoidable render.
  const [lastSessionId, setLastSessionId] = useState(sessionId);
  if (sessionId !== lastSessionId) {
    setLastSessionId(sessionId);
    if (closeTarget && sessionId && sessionId !== closeTarget.sessionId) {
      setCloseTarget(null);
    }
  }

  return {
    complexId: selectedComplexId,
    sessionQuery,
    detailQuery,
    openDialogOpen,
    setOpenDialogOpen,
    movementDialog,
    setMovementDialog,
    closeTarget,
    setCloseTarget,
    voidTarget,
    setVoidTarget,
  };
}
