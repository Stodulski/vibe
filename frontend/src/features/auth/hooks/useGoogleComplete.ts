import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useAuthSuccessHandler } from './authSuccess';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getHttpStatus, getHttpErrorMessage } from '@/shared/lib/utils';
import type { GoogleCompleteRequest } from '@/shared/types/api.types';

/**
 * Handles `POST /auth/google/complete` — the second step of Google sign-up,
 * submitted from `GoogleCompletePage`. Success behaves exactly like a login
 * (same store updates, same redirect logic as `useLogin`/`useGoogleExchange`).
 *
 * A 422 field-validation error is deliberately left un-toasted here: the
 * page itself applies it directly onto the form via a per-call `onError`
 * passed to `mutate()` (same pattern as `useComplexForm`), so the person
 * sees the error under the field instead of only in a toast.
 */
export interface UseGoogleCompleteOptions {
  /**
   * Runs the moment the account exists on the server, before any state
   * update. It lives in `mutationFn` and not in an `onSuccess` on purpose:
   * the hook-level `onSuccess` below navigates away, and a mutate-level
   * `onSuccess` is skipped by TanStack Query once the observer has
   * unmounted, so a flag set there could be lost in exactly the case it
   * exists for (see `useAbandonedGoogleSignupLead`).
   */
  onAccountCreated?: () => void;
}

export function useGoogleComplete(options: UseGoogleCompleteOptions = {}) {
  const t = ES_AR;
  const handleAuthSuccess = useAuthSuccessHandler();
  const { onAccountCreated } = options;

  return useMutation({
    mutationFn: async (data: GoogleCompleteRequest) => {
      const response = await authApi.googleComplete(data);
      onAccountCreated?.();
      return response;
    },
    // Wrapped for the reason `useLogin` states: React Query would otherwise
    // pass this mutation's variables as the handler's options bag. The
    // destination a Google sign-in was carrying reaches the handler through
    // `location.state.from` here, put there by `useGoogleExchange`.
    onSuccess: (data) => {
      handleAuthSuccess(data);
    },
    onError: (error: unknown) => {
      const status = getHttpStatus(error);

      if (status === 429) {
        toast.error(t.auth.rateLimitError);
        return;
      }
      if (status === 401) {
        toast.error(t.auth.googleSessionExpired);
        return;
      }
      if (status === 409) {
        toast.error(t.auth.googleAccountExists);
        return;
      }
      if (status === 422) return;

      toast.error(getHttpErrorMessage(error, t.auth.googleSignInError));
    },
  });
}
