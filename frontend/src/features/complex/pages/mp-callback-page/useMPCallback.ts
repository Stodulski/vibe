import { useEffect, useRef, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { HTTPError } from 'ky';
import { queryKeys } from '@/shared/lib/queryKeys';
import { mpRedirectUri } from '@/shared/lib/mpAuth';
import { consumeMPOAuthSession, DEFAULT_RETURN_PATH, type MPOAuthSession } from '@/shared/lib/mpOAuthSession';
import { complexApi } from '../../api/complex.api';

export type MPCallbackStatus = 'processing' | 'success' | 'error';

/**
 * Why the connection failed, for the copy the page shows:
 * - `conflict`: the API refused (409) — a different MercadoPago account while
 *   the complex has active bookings. Retrying cannot help until they are gone.
 * - `denied`: the owner cancelled on MercadoPago (`?error=access_denied`, no
 *   code). Nothing went wrong; they can simply start again.
 * - `expired`: the attempt cannot be completed — unknown or already-used
 *   state (reload, cleared storage, another browser) or the API rejected the
 *   code (other 4xx). The only way forward is to start the connection again.
 * - `failed`: anything else (network, 5xx).
 */
export type MPCallbackErrorReason = 'conflict' | 'denied' | 'expired' | 'failed';

interface OAuthParams {
  code: string | null;
  session: MPOAuthSession | null;
  denied: boolean;
}

// Reads the OAuth redirect params and consumes the attempt bound to `state`,
// synchronously and exactly once. Called from a lazy `useState` initializer —
// this is the officially-recommended way to compute a value once "when the
// component is first created" without an effect
// (react.dev/learn/you-might-not-need-an-effect), which avoids the
// `react-hooks/set-state-in-effect` violation an effect-driven `setStatus`
// would trigger for this synchronous, one-time check. The entry is deleted
// here, before anything is sent, so reloading the page cannot replay the
// single-use code. It is consumed even when `code` is missing (MercadoPago
// answers `?error=access_denied&state=…` if the owner declines).
function readOAuthParams(searchParams: URLSearchParams): OAuthParams {
  const code = searchParams.get('code');
  const state = searchParams.get('state');
  return {
    code,
    session: state ? consumeMPOAuthSession(state) : null,
    denied: code === null && searchParams.get('error') === 'access_denied',
  };
}

function errorReasonFor(error: unknown): MPCallbackErrorReason {
  if (!(error instanceof HTTPError)) return 'failed';
  const { status } = error.response;
  if (status === 409) return 'conflict';
  if (status >= 400 && status < 500) return 'expired';
  return 'failed';
}

function useConnectMutation({
  queryClient,
  setStatus,
  setErrorReason,
  redirectTimeoutRef,
  navigate,
  returnPath,
}: {
  queryClient: ReturnType<typeof useQueryClient>;
  setStatus: (status: MPCallbackStatus) => void;
  setErrorReason: (reason: MPCallbackErrorReason) => void;
  redirectTimeoutRef: React.RefObject<ReturnType<typeof setTimeout> | null>;
  navigate: ReturnType<typeof useNavigate>;
  returnPath: string;
}) {
  return useMutation({
    mutationFn: (params: { complexId: string; code: string; codeVerifier: string }) =>
      complexApi.connectMP(params.complexId, {
        code: params.code,
        redirect_uri: mpRedirectUri(),
        code_verifier: params.codeVerifier,
      }),
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.complexes.mpStatus(variables.complexId),
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.all });
      setStatus('success');
      redirectTimeoutRef.current = setTimeout(() => {
        void navigate(returnPath, { replace: true });
      }, 1500);
    },
    // No auto-redirect on failure: the code is single-use, so the page stays
    // and offers the way back to where the connect action lives.
    onError: (error) => {
      setErrorReason(errorReasonFor(error));
      setStatus('error');
    },
  });
}

export function useMPCallback() {
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const redirectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const submittedRef = useRef(false);

  useEffect(
    () => () => {
      if (redirectTimeoutRef.current) clearTimeout(redirectTimeoutRef.current);
    },
    [],
  );

  const [oauthParams] = useState(() => readOAuthParams(searchParams));
  const returnPath = oauthParams.session?.returnPath ?? DEFAULT_RETURN_PATH;
  const canExchange = oauthParams.code !== null && oauthParams.session !== null;
  const [status, setStatus] = useState<MPCallbackStatus>(canExchange ? 'processing' : 'error');
  const [errorReason, setErrorReason] = useState<MPCallbackErrorReason>(oauthParams.denied ? 'denied' : 'expired');

  const connectMutation = useConnectMutation({
    queryClient,
    setStatus,
    setErrorReason,
    redirectTimeoutRef,
    navigate,
    returnPath,
  });

  // `oauthParams` never changes after mount (read once, above) and `mutate`
  // is a stable function identity, so this effect's deps never change. The
  // ref covers what deps cannot: StrictMode runs the effect twice in
  // development, and the authorization code is single-use.
  const { mutate } = connectMutation;
  useEffect(() => {
    const { code, session } = oauthParams;
    if (!code || !session || submittedRef.current) return;
    submittedRef.current = true;
    mutate({ complexId: session.complexId, code, codeVerifier: session.codeVerifier });
  }, [oauthParams, mutate]);

  return { status, errorReason, returnPath };
}
