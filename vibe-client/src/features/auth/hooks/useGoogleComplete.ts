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
 * (same store updates, same redirect logic as `useLogin`/`useGoogleSignIn`).
 *
 * A 422 field-validation error is deliberately left un-toasted here: the
 * page itself applies it directly onto the form via a per-call `onError`
 * passed to `mutate()` (same pattern as `useComplexForm`), so the person
 * sees the error under the field instead of only in a toast.
 */
export function useGoogleComplete() {
  const t = ES_AR;
  const handleAuthSuccess = useAuthSuccessHandler();

  return useMutation({
    mutationFn: (data: GoogleCompleteRequest) => authApi.googleComplete(data),
    onSuccess: handleAuthSuccess,
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
