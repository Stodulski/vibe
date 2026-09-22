import { useQuery } from '@tanstack/react-query';
import { cashApi } from '../api/cash.api';
import { queryKeys } from '@/shared/lib/queryKeys';

/**
 * One session's full detail — summary and movement ledger.
 *
 * Used for BOTH the currently open session's own page (once `useCashSession`
 * resolves its id — `/cash-session` alone carries no movement list) and a
 * past, closed session's read-only detail page (`/cash/sessions/:id`).
 */
export function useCashSessionDetail(complexId: string | null, sessionId: string | null) {
  const id = complexId ?? '';
  const sid = sessionId ?? '';
  return useQuery({
    queryKey: queryKeys.cash.detail(id, sid),
    queryFn: ({ signal }) => cashApi.getById(id, sid, signal),
    enabled: !!complexId && !!sessionId,
    staleTime: 30 * 1000,
  });
}
