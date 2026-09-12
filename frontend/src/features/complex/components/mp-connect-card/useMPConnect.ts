import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { HTTPError } from 'ky';
import api, { withSignal } from '@/shared/lib/ky';
import { queryKeys } from '@/shared/lib/queryKeys';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { safeSessionStorage } from '@/shared/lib/safeStorage';
import { ES_AR } from '@/shared/i18n/es_AR';
import { buildMPAuthUrl, generatePKCE } from '@/shared/lib/mpAuth';
import { parseWith } from '@/shared/lib/apiParse';
import { mpStatusResponseSchema } from '@/shared/schemas/publicBooking.schema';
import type { MPStatusResponse } from '@/shared/types/api.types';

const t = ES_AR;

interface UseMPConnectOptions {
  enabled?: boolean;
  refetchInterval?: number | false;
}

function useDisconnectMutation(
  complexId: string,
  queryClient: ReturnType<typeof useQueryClient>,
  setShowDisconnect: (open: boolean) => void,
) {
  return useMutation({
    mutationFn: () => api.delete(`complexes/${complexId}/mp/connect`).json(),
    onSuccess: () => {
      setShowDisconnect(false);
      toast.success(t.mp.disconnectSuccess);
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.mpStatus(complexId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
    },
    onError: (error) => {
      if (error instanceof HTTPError && error.response.status === 409) {
        toast.error(t.mp.disconnectBlocked);
      } else {
        toast.error(t.mp.disconnectError);
      }
      setShowDisconnect(false);
    },
  });
}

export function useMPConnect(complexId: string, options?: UseMPConnectOptions) {
  const queryClient = useQueryClient();
  const [showDisconnect, setShowDisconnect] = useState(false);
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [pkceVerifier, setPkceVerifier] = useState<string | null>(null);

  const {
    data: mpStatus,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: queryKeys.complexes.mpStatus(complexId),
    queryFn: ({ signal }): Promise<MPStatusResponse> =>
      api
        .get(`complexes/${complexId}/mp/status`, withSignal(signal))
        .json()
        .then(parseWith(mpStatusResponseSchema, 'useMPConnect.mpStatus')),
    // A connection is made or broken by an explicit action on this card, each
    // of which invalidates this key itself; callers polling for the moment the
    // OAuth callback lands pass their own `refetchInterval`.
    staleTime: 60 * 1000,
    // TanStack Query's `enabled`/`refetchInterval` options aren't typed with
    // an explicit `| undefined`; under `exactOptionalPropertyTypes`, passing
    // them through as `options?.enabled` (present-but-possibly-`undefined`)
    // doesn't satisfy that, even though omitting the key entirely — the
    // library's own default — does. Only include each key when the caller
    // actually set it.
    ...(options?.enabled !== undefined ? { enabled: options.enabled } : {}),
    ...(options?.refetchInterval !== undefined ? { refetchInterval: options.refetchInterval } : {}),
  });

  // Pre-compute PKCE + auth URL so the <a> tag has a real href on tap. Waits
  // on mpStatus so the URL can carry its app_id — the app id the API can
  // actually exchange a code with — rather than build one from the
  // build-time env var and possibly send the seller through the wrong app.
  useEffect(() => {
    if (!mpStatus) return;
    let cancelled = false;
    void generatePKCE().then((pkce) => {
      if (cancelled) return;
      setPkceVerifier(pkce.verifier);
      setAuthUrl(buildMPAuthUrl(complexId, pkce, mpStatus.app_id));
    });
    return () => {
      cancelled = true;
    };
  }, [complexId, mpStatus]);

  const disconnectMutation = useDisconnectMutation(complexId, queryClient, setShowDisconnect);

  // Store PKCE verifier right before navigating (onClick fires before navigation)
  const handleConnectClick = () => {
    if (pkceVerifier) {
      safeSessionStorage.set(STORAGE_KEYS.MP_CODE_VERIFIER, pkceVerifier);
      safeSessionStorage.set(STORAGE_KEYS.MP_RETURN_PATH, window.location.pathname);
    }
  };

  return {
    isLoading,
    isError,
    refetch,
    connected: mpStatus?.connected ?? false,
    mpUserId: mpStatus?.mp_user_id,
    authUrl,
    handleConnectClick,
    showDisconnect,
    setShowDisconnect,
    disconnectMutation,
  };
}
