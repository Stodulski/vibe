import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { authApi } from '../api/auth.api';
import { useAuthSuccessHandler } from './authSuccess';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { LoginRequest } from '@/shared/types/api.types';
import { getHttpStatus } from '@/shared/lib/utils';
import { getTurnstileError } from '@/shared/lib/serverErrors';

interface UseLoginOptions {
  /** Called on every failed submit — Turnstile tokens are single-use, so the widget must reset before the next attempt. */
  resetTurnstile?: () => void;
}

export function useLogin(options?: UseLoginOptions) {
  const t = ES_AR;
  const handleAuthSuccess = useAuthSuccessHandler();

  return useMutation({
    mutationFn: (data: LoginRequest) => authApi.login(data),
    // Wrapped, not passed by reference: React Query calls `onSuccess` with
    // `(data, variables, …)`, and the handler's second parameter is now an
    // options bag — handing it this mutation's variables would be nonsense
    // that happens to be harmless. Only the Google redirect flow has a
    // destination the current location cannot supply.
    onSuccess: (data) => {
      handleAuthSuccess(data);
    },
    onError: (error: unknown) => {
      // Turnstile tokens are single-use: any failed submit must get a fresh
      // one before the next attempt, not only a Turnstile-specific failure.
      options?.resetTurnstile?.();

      if (getHttpStatus(error) === 429) {
        toast.error(t.auth.rateLimitError);
        return;
      }

      // Unlike the credentials check below, a failed Turnstile challenge
      // says nothing about whether the account exists — safe to name.
      const turnstileError = getTurnstileError(error);
      if (turnstileError) {
        toast.error(turnstileError);
        return;
      }

      // Generic error for all auth failures (prevents account enumeration —
      // the backend answers "wrong password" and "unverified email" the
      // same way, on purpose). The hint applies either way without telling
      // an attacker which case it actually was.
      toast.error(t.auth.invalidCredentials, { description: t.auth.checkInboxIfUnverified });
    },
  });
}
