import type { QueryClient, QueryKey } from '@tanstack/react-query';
import { queryKeys } from './queryKeys';

/** The screens outside the calendar that a booking write can leave stale. */
export type BookingSideEffect = 'cash' | 'clients' | 'series';

/**
 * Refreshes what a booking write changes beyond the calendar, so each caller
 * names only what its write touches instead of re-listing the keys:
 *
 * - `cash`: the open session (its booking payments and manual refunds are
 *   part of the expected cash), its ledger, and today's bookings still owing.
 * - `clients`: the clients list (every search) and every client's detail,
 *   whose bookings and balances come from these rows.
 * - `series`: any open fixed-booking panel, which lists its occurrences.
 *
 * Lives in `shared/` because the realtime stream needs the same set.
 */
export function invalidateBookingSideEffects(
  queryClient: QueryClient,
  complexId: string,
  effects: readonly BookingSideEffect[],
) {
  const keys = effects.flatMap((effect): QueryKey[] => {
    switch (effect) {
      case 'cash':
        return [
          queryKeys.cash.current(complexId),
          queryKeys.cash.detailBase(complexId),
          queryKeys.cash.bookingsDue(complexId),
        ];
      case 'clients':
        return [queryKeys.clients.byComplex(complexId)];
      case 'series':
        return [queryKeys.bookings.seriesBase(complexId)];
    }
  });
  return Promise.all(keys.map((queryKey) => queryClient.invalidateQueries({ queryKey })));
}
