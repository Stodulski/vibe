import { useEffect, useMemo, useState } from 'react';
import { useBookingsDue } from '../../hooks/useBookingsDue';
import { loadBalanceCart, saveBalanceCart } from '../../lib/cartStorage';
import {
  addBalanceLine,
  balancesTotal,
  reconcileBalances,
  removeBalanceLine,
  type BalanceLine,
} from '../../lib/balanceCart';
import type { BalanceDue } from '@/shared/types/api.types';

/** Today's list as the sell page last received it. `stamp` changes on every refetch. */
export interface DueSnapshot {
  bookings: BalanceDue[];
  stamp: number;
}

/**
 * The day's list for the sell page: what to show, whether it is loading or
 * failed, and the snapshot `useSellBalances` reconciles the cart against.
 * Only fetched while a till is open (see `useBookingsDue`).
 */
export function useBalanceDue(complexId: string | null, sessionId: string | null) {
  const query = useBookingsDue(complexId, sessionId);
  const snapshot = useMemo<DueSnapshot | null>(
    () => (query.isSuccess ? { bookings: query.data.bookings, stamp: query.dataUpdatedAt } : null),
    [query.isSuccess, query.data, query.dataUpdatedAt],
  );

  return {
    list: {
      bookings: query.data?.bookings ?? [],
      isLoading: query.isLoading,
      isError: query.isError,
      refetch: () => {
        void query.refetch();
      },
    },
    snapshot,
  };
}

interface BalancesState {
  lines: BalanceLine[];
  /** True when a reconcile dropped at least one line — the caller shows a one-time notice. */
  droppedNotice: boolean;
}

/**
 * The booking balances on the cart: tap-to-add from today's list, the
 * sessionStorage copy per complex, and reconciling against the list whenever
 * it loads or refetches. Split out of `useSellPage` the same way `useSellCart`
 * is, so neither function grows past this repo's `max-lines-per-function`.
 *
 * Loading and reconciling run during render, guarded by the complex and the
 * list's `stamp`: the same "adjust state during render on a tracked previous
 * value" shape `useSellCart` uses for its product lines. Nothing here waits
 * on an effect to change state; the effect only writes sessionStorage.
 */
export function useSellBalances(complexId: string | null, due: DueSnapshot | null) {
  const [state, setState] = useState<BalancesState>({ lines: [], droppedNotice: false });
  const [synced, setSynced] = useState<{ complexId: string | null; stamp: number | null }>({
    complexId: null,
    stamp: null,
  });

  if (complexId && due && (synced.complexId !== complexId || synced.stamp !== due.stamp)) {
    const sameComplex = synced.complexId === complexId;
    const base = sameComplex ? state.lines : loadBalanceCart(complexId);
    const { lines, dropped } = reconcileBalances(base, due.bookings);
    setSynced({ complexId, stamp: due.stamp });
    setState({ lines, droppedNotice: dropped || (sameComplex && state.droppedNotice) });
  }

  // Persisting is a real effect: it mirrors the cart into sessionStorage. Only
  // once this complex's lines are in state, so an old complex's cart can
  // never be written under the new complex's key.
  useEffect(() => {
    if (!complexId || synced.complexId !== complexId) return;
    saveBalanceCart(complexId, state.lines);
  }, [complexId, synced.complexId, state.lines]);

  return {
    lines: state.lines,
    total: balancesTotal(state.lines),
    droppedNotice: state.droppedNotice,
    dismissDroppedNotice: () => {
      setState((prev) => ({ ...prev, droppedNotice: false }));
    },
    add: (booking: BalanceDue) => {
      setState((prev) => ({ ...prev, lines: addBalanceLine(prev.lines, booking) }));
    },
    remove: (bookingId: string) => {
      setState((prev) => ({ ...prev, lines: removeBalanceLine(prev.lines, bookingId) }));
    },
    /**
     * Removes exactly the bookings a completed checkout collected, the same
     * reasoning as `useSellCart.removeCharged`: a balance tapped while the
     * request was in flight belongs to the next checkout and must survive.
     */
    removeCharged: (bookingIds: string[]) => {
      setState((prev) => ({ ...prev, lines: prev.lines.filter((line) => !bookingIds.includes(line.bookingId)) }));
    },
  };
}
