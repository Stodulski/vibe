import { useCallback, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useClientActions, useClient } from '@/features/clients';
import { queryKeys } from '@/shared/lib/queryKeys';
import type { TopClient } from '@/shared/types/api.types';

/**
 * Opens/manages a client's detail sheet (+ its block action) directly on the
 * dashboard, so clicking a row in "Top clientes" doesn't have to leave the
 * page — reuses the exact same modals/actions the /clients page is built on.
 *
 * Takes a resolved `complexId` — both callers (`TodayBookings`,
 * `ClientInsightsCard`) only ever mount once the dashboard has one, so
 * there's no "no complex yet" state here to fall back for.
 */
export function useDashboardClientDetail(complexId: string) {
  const queryClient = useQueryClient();

  const invalidateDashboardClients = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.clients(complexId) });
  }, [queryClient, complexId]);

  const actions = useClientActions(complexId, invalidateDashboardClients);

  // TopClient (the dashboard's own leaner projection) is missing most Client
  // fields, so it can't seed a placeholder `Client` the way a full grid row
  // can. Track the real id instead and read the record from the same cache
  // `ClientDetail`'s own `useClient(complexId, id)` fills.
  const [topClientId, setTopClientId] = useState<string | null>(null);
  const { data: topClientDetail } = useClient(complexId, topClientId);

  const handleSelectTopClient = useCallback(
    (topClient: TopClient) => {
      setTopClientId(topClient.id);
      actions.setDetailOpen(true);
    },
    [actions],
  );

  const setDetailOpen = useCallback(
    (open: boolean) => {
      if (!open) setTopClientId(null);
      actions.setDetailOpen(open);
    },
    [actions],
  );

  const selectedClient = topClientId ? (topClientDetail?.client ?? null) : actions.selectedClient;

  return { ...actions, selectedClient, setDetailOpen, handleSelectTopClient };
}
