import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/shared/lib/queryKeys';
import { getHttpStatus } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { instagramApi } from '../api/instagram.api';

const t = ES_AR;

/**
 * The complex's Instagram connection. Also what decides whether the panel
 * offers Instagram at all: `enabled` is false until the feature is switched on
 * server-side. Disabled until there is a complex to ask about.
 */
export function useInstagramStatus(complexId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.complexes.instagramStatus(complexId ?? ''),
    queryFn: ({ signal }) => instagramApi.getStatus(complexId ?? '', signal),
    enabled: Boolean(complexId),
    // Connecting leaves the page and comes back, which mounts a fresh query;
    // disconnecting invalidates the key. Nothing else changes it.
    staleTime: 60 * 1000,
    // The card renders the error with its own retry button.
    throwOnError: false,
  });
}

/**
 * Asks the server for a consent URL and sends the whole page there. The
 * browser comes back through the API callback to `/settings?tab=instagram`
 * with the outcome in the query string (see `useInstagramCallback`).
 */
export function useConnectInstagram(complexId: string) {
  return useMutation({
    mutationFn: () => instagramApi.connect(complexId),
    onSuccess: ({ authorize_url }) => {
      window.location.assign(authorize_url);
    },
    onError: (error) => {
      toast.error(getHttpStatus(error) === 501 ? t.instagram.connectUnavailable : t.instagram.connectError);
    },
  });
}

export function useDisconnectInstagram(complexId: string, onSettled: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => instagramApi.disconnect(complexId),
    onSuccess: () => {
      toast.success(t.instagram.disconnectSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.instagramStatus(complexId) });
    },
    onError: () => {
      toast.error(t.instagram.disconnectError);
    },
    onSettled,
  });
}
