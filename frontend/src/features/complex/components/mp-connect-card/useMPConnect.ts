import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { HTTPError } from 'ky';
import api, { withSignal } from '@/shared/lib/ky';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';
import { buildMPAuthUrl, createMPAuthAttempt, type MPAuthAttempt } from '@/shared/lib/mpAuth';
import { saveMPOAuthSession } from '@/shared/lib/mpOAuthSession';
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
  const [prepared, setPrepared] = useState<{ attempt: MPAuthAttempt; authUrl: string } | null>(null);

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
    // The card renders `isError` with its own retry button.
    throwOnError: false,
    // TanStack Query's `enabled`/`refetchInterval` options aren't typed with
    // an explicit `| undefined`; under `exactOptionalPropertyTypes`, passing
    // them through as `options?.enabled` (present-but-possibly-`undefined`)
    // doesn't satisfy that, even though omitting the key entirely — the
    // library's own default — does. Only include each key when the caller
    // actually set it.
    ...(options?.enabled !== undefined ? { enabled: options.enabled } : {}),
    ...(options?.refetchInterval !== undefined ? { refetchInterval: options.refetchInterval } : {}),
  });

  // Prepare one attempt (PKCE pair + state nonce) and its auth URL so the <a>
  // tag has a real href on tap. Waits on mpStatus so the URL can carry its
  // app_id — the app id the API can actually exchange a code with — rather
  // than build one from the build-time env var and possibly send the seller
  // through the wrong app. Nothing is written to storage here: that happens
  // on click, so loading the card (or each status refetch) leaves no state
  // behind.
  const hasStatus = mpStatus !== undefined;
  const appId = mpStatus?.app_id;
  useEffect(() => {
    if (!hasStatus) return;
    let cancelled = false;
    void createMPAuthAttempt().then((attempt) => {
      if (cancelled) return;
      setPrepared({ attempt, authUrl: buildMPAuthUrl(attempt, appId) });
    });
    return () => {
      cancelled = true;
    };
  }, [hasStatus, appId]);

  const disconnectMutation = useDisconnectMutation(complexId, queryClient, setShowDisconnect);

  // Persist the attempt right before navigating (onClick fires before the
  // browser follows the link), bound to its own state nonce so a callback can
  // only ever pick up the verifier that matches it.
  const handleConnectClick = () => {
    if (!prepared) return;
    saveMPOAuthSession(prepared.attempt.state, {
      complexId,
      codeVerifier: prepared.attempt.verifier,
      returnPath: window.location.pathname,
    });
  };

  return {
    isLoading,
    isError,
    refetch,
    connected: mpStatus?.connected ?? false,
    mpUserId: mpStatus?.mp_user_id,
    authUrl: prepared?.authUrl ?? null,
    handleConnectClick,
    showDisconnect,
    setShowDisconnect,
    disconnectMutation,
  };
}
