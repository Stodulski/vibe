import { useEffect, useRef, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useStore } from '@/shared/stores';
import { queryKeys } from '@/shared/lib/queryKeys';
import { STORAGE_KEYS } from '@/shared/lib/storageKeys';
import { complexApi } from '../../api/complex.api';
import { safeSessionStorage } from '@/shared/lib/safeStorage';

interface OAuthParams {
  code: string | null;
  complexId: string | null;
}

// Reads (and, for the nonce-keyed complexId lookup, consumes) the OAuth
// redirect params synchronously. Called once from a lazy `useState`
// initializer — this is the officially-recommended way to compute a value
// once "when the component is first created" without an effect
// (react.dev/learn/you-might-not-need-an-effect), which avoids the
// `react-hooks/set-state-in-effect` violation an effect-driven `setStatus`
// would trigger for this synchronous, one-time check.
function readOAuthParams(searchParams: URLSearchParams): OAuthParams {
  const code = searchParams.get('code');
  const stateNonce = searchParams.get('state');
  const complexId = stateNonce ? safeSessionStorage.get('mp_oauth_complex_' + stateNonce) : null;
  if (stateNonce) safeSessionStorage.remove('mp_oauth_complex_' + stateNonce);
  return { code, complexId };
}

function useConnectMutation({
  queryClient,
  setSelectedComplexId,
  setStatus,
  redirectTimeoutRef,
  navigate,
  returnPath,
}: {
  queryClient: ReturnType<typeof useQueryClient>;
  setSelectedComplexId: (id: string) => void;
  setStatus: (status: 'processing' | 'success' | 'error') => void;
  redirectTimeoutRef: React.RefObject<ReturnType<typeof setTimeout> | null>;
  navigate: ReturnType<typeof useNavigate>;
  returnPath: string;
}) {
  return useMutation({
    mutationFn: (params: { complexId: string; code: string; codeVerifier?: string | undefined }) =>
      complexApi.connectMP(params.complexId, {
        code: params.code,
        redirect_uri: `${window.location.origin}/settings/mp/callback`,
        ...(params.codeVerifier ? { code_verifier: params.codeVerifier } : {}),
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.complexes.mpStatus(variables.complexId),
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      setSelectedComplexId(variables.complexId);
      setStatus('success');
      safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
      redirectTimeoutRef.current = setTimeout(() => {
        void navigate(returnPath, { replace: true });
      }, 1500);
    },
    onError: () => {
      setStatus('error');
      safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
      redirectTimeoutRef.current = setTimeout(() => {
        void navigate(returnPath, { replace: true });
      }, 2500);
    },
  });
}

export function useMPCallback() {
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { setSelectedComplexId } = useStore();
  const redirectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (redirectTimeoutRef.current) clearTimeout(redirectTimeoutRef.current);
    },
    [],
  );

  const [oauthParams] = useState(() => readOAuthParams(searchParams));
  // Read once, the same way as `oauthParams` above and for the same reason:
  // the mutation removes this key from `sessionStorage` on settle, and
  // re-reading it fresh on every render would make the effect below see a
  // "changed" dependency after that removal and want to re-run.
  const [returnPath] = useState(() => safeSessionStorage.get(STORAGE_KEYS.MP_RETURN_PATH) ?? '/settings');
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>(() =>
    !oauthParams.code || !oauthParams.complexId ? 'error' : 'processing',
  );

  const connectMutation = useConnectMutation({
    queryClient,
    setSelectedComplexId,
    setStatus,
    redirectTimeoutRef,
    navigate,
    returnPath,
  });

  // `oauthParams` and `returnPath` never change after mount (both are read
  // once, above), and `mutate` is a stable function identity — so unlike a
  // dependency on the `connectMutation` object itself (a new identity every
  // render), this effect's deps never change after the first run and it
  // needs no `processed` ref to guard against re-submitting the OAuth code.
  const { mutate } = connectMutation;
  useEffect(() => {
    if (!oauthParams.code || !oauthParams.complexId) {
      safeSessionStorage.remove(STORAGE_KEYS.MP_RETURN_PATH);
      const timer = setTimeout(() => {
        void navigate(returnPath, { replace: true });
      }, 2500);
      return () => {
        clearTimeout(timer);
      };
    }

    const codeVerifier = safeSessionStorage.get(STORAGE_KEYS.MP_CODE_VERIFIER) ?? undefined;
    safeSessionStorage.remove(STORAGE_KEYS.MP_CODE_VERIFIER);

    mutate({ complexId: oauthParams.complexId, code: oauthParams.code, codeVerifier });
  }, [oauthParams, mutate, navigate, returnPath]);

  return { status, returnPath };
}
