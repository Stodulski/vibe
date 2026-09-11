import { useState, useCallback } from 'react';
import { toast } from 'sonner';
import type { UseMutationResult } from '@tanstack/react-query';
import { useUpdateClient } from '@/features/clients/hooks/useUpdateClient';
import { useClient } from '@/features/clients/hooks/useClient';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Client } from '@/shared/types/api.types';

const t = ES_AR;

// `complexId` is required by `useUpdateClient`, but this hook itself can be
// mounted before the owner's complex resolves. Rather than falling back to
// `?? ''` and risking a `PATCH /complexes//clients/:id` if that "never
// happens" assumption is ever wrong (03-owner-pages-dashboard.md M1), the
// mutation stays wired to `''` for the query keys it touches but its
// `mutate`/`mutateAsync` become explicit no-ops until a real id exists.
function disabledMutation<TData, TError, TVariables, TContext>(
  mutation: UseMutationResult<TData, TError, TVariables, TContext>,
): UseMutationResult<TData, TError, TVariables, TContext> {
  return {
    ...mutation,
    mutate: () => {
      /* no complex selected yet — never reachable from the UI, kept for safety */
    },
    mutateAsync: () => Promise.reject(new Error('useClientActions: no complex selected')),
  };
}

/**
 * A selected client's id is the real state — the object is resolved from the
 * same query cache `ClientDetail`'s own `useClient` fills, so it can't hold a
 * copy that goes stale the way keeping the whole `Client` in state could (see
 * 03-owner-pages-dashboard.md M8: "guardá el id, no el objeto"). `seed` only
 * covers the render before that query has data.
 */
function useSelectedClient(complexId: string | null) {
  const [id, setId] = useState<string | null>(null);
  const [seed, setSeed] = useState<Client | null>(null);

  const { data: detail } = useClient(complexId, id);
  const client = detail?.client ?? seed;

  const select = useCallback((next: Client | null) => {
    setId(next?.id ?? null);
    setSeed(next);
  }, []);

  return { client, id, select };
}

export function useClientActions(
  selectedComplexId: string | null,
  // Called after a block/unblock succeeds, in addition to closing the modal
  // — lets a caller outside the /clients page (e.g. the dashboard's own
  // detail drawer) refresh its own cache, which this mutation doesn't touch.
  onActionSuccess?: () => void,
) {
  // The mutation hook only builds a closure here; it's never actually
  // triggered before a client row exists to act on, which only renders once
  // `useClients` above (gated on `selectedComplexId`) has real data. Guarded
  // below regardless, so that assumption isn't load-bearing for safety.
  const updateClientMutation = useUpdateClient(selectedComplexId ?? '');
  const updateClient = selectedComplexId ? updateClientMutation : disabledMutation(updateClientMutation);

  const selected = useSelectedClient(selectedComplexId);
  const [detailOpen, setDetailOpen] = useState(false);
  // A separate selection from `selected`: blocking can be started straight
  // from a grid row's menu, for a client whose detail drawer was never opened.
  const toBlock = useSelectedClient(selectedComplexId);

  const handleSelectClient = useCallback(
    (client: Client) => {
      selected.select(client);
      setDetailOpen(true);
    },
    [selected],
  );

  const handleBlockClient = useCallback(
    (client: Client) => {
      toBlock.select(client);
    },
    [toBlock],
  );

  const handleConfirmBlock = useCallback(() => {
    const blockClient = toBlock.client;
    if (!blockClient) return;
    const isBlocking = !blockClient.is_blocked;
    updateClient.mutate(
      { clientId: blockClient.id, data: { is_blocked: isBlocking } },
      {
        onSuccess: () => {
          toast.success(isBlocking ? t.clients.blockSuccess : t.clients.unblockSuccess);
          toBlock.select(null);
          onActionSuccess?.();
        },
      },
    );
  }, [toBlock, updateClient, onActionSuccess]);

  return {
    updateClient,
    selectedClient: selected.client,
    setSelectedClient: selected.select,
    detailOpen,
    setDetailOpen,
    blockClient: toBlock.client,
    setBlockClient: toBlock.select,
    handleSelectClient,
    handleBlockClient,
    handleConfirmBlock,
  };
}
