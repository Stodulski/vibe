import { useEffect, useRef } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { authApi } from '../api/auth.api';
import { useStore } from '@/shared/stores';
import { getProblem } from '@/shared/lib/ApiError';

export type ConfirmEmailChangeStatus = 'invalid' | 'idle' | 'loading' | 'success' | 'taken' | 'rate_limited' | 'error';

/**
 * Maps a failed confirmation to the status its screen shows.
 *
 * 400 means the link itself is bad — unknown, expired or already used — and
 * there is nothing to retry. 422 means the token was fine but the address it
 * named was claimed since the request was made. 429 and anything else
 * (network failure, 5xx, an unrecognized body) are both transient: the link
 * may still work, so the caller keeps the confirm button and can try again.
 */
function mapConfirmEmailChangeError(error: unknown): 'invalid' | 'taken' | 'rate_limited' | 'error' {
  const status = getProblem(error)?.status;
  if (status === 400) return 'invalid';
  if (status === 422) return 'taken';
  if (status === 429) return 'rate_limited';
  return 'error';
}

/**
 * Confirms a pending email change from the token in `?token=`, but only on
 * an explicit click of `confirm()` — the token is single-use, so firing the
 * request on page load spent it before the person had even read what the
 * link does. `useMutation` is the right tool since nothing fires
 * automatically: `firedRef` is this hook's own single-fire guard, closed
 * synchronously on click so two fast clicks before the button disables
 * cannot both call `mutate()`, and reopened once the attempt settles so a
 * retryable failure (`rate_limited`, `error`) can be retried.
 *
 * On success the backend has already changed the address, marked it
 * unverified and revoked every session for the account — so this tab ends
 * its own session the same way the 401 handler in `ky.ts` does, instead of
 * going on rendering one the server no longer honours.
 */
export function useConfirmEmailChange() {
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const logout = useStore((s) => s.logout);
  const token = searchParams.get('token');
  const firedRef = useRef(false);

  const mutation = useMutation({
    mutationFn: () => authApi.confirmEmailChange(token ?? ''),
  });

  useEffect(() => {
    if (!mutation.isSuccess) return;
    logout();
    queryClient.clear();
  }, [mutation.isSuccess, logout, queryClient]);

  function confirm() {
    if (!token || firedRef.current) return;
    firedRef.current = true;
    mutation.mutate(undefined, {
      onSettled: () => {
        firedRef.current = false;
      },
    });
  }

  const status = resolveStatus(token, mutation.isPending, mutation.isSuccess, mutation.isError, mutation.error);

  return { status, confirm };
}

/**
 * Priority order: invalid, loading, success, error, idle — an invalid link
 * wins even mid-request, a settled success wins over a stale error, and
 * anything else falls through to idle.
 */
function resolveStatus(
  token: string | null,
  isPending: boolean,
  isSuccess: boolean,
  isError: boolean,
  error: unknown,
): ConfirmEmailChangeStatus {
  if (!token) return 'invalid';
  if (isPending) return 'loading';
  if (isSuccess) return 'success';
  if (isError) return mapConfirmEmailChangeError(error);
  return 'idle';
}
