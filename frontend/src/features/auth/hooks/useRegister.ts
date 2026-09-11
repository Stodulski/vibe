import { useMutation } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { toast } from 'sonner';
import { HTTPError } from 'ky';
import { authApi } from '../api/auth.api';
import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';
import { getTurnstileError } from '@/shared/lib/serverErrors';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { RegisterRequest } from '@/shared/types/api.types';

/** ky consumes the body before throwing, so the payload lives on `.data`. */
function errorBody(error: unknown): unknown {
  return error instanceof HTTPError ? error.data : null;
}

interface UseRegisterOptions {
  /** Called on every failed submit — Turnstile tokens are single-use, so the widget must reset before the next attempt. */
  resetTurnstile?: () => void;
  /**
   * Runs the moment the account exists on the server, before any state
   * update. It lives in `mutationFn` and not in an `onSuccess` on purpose:
   * the hook-level `onSuccess` below navigates to `/verify-email-sent`, and
   * a mutate-level `onSuccess` is skipped by TanStack Query once the
   * observer has unmounted, so a flag set there could be lost in exactly
   * the case it exists for (see `useAbandonedRegistrationLead`).
   */
  onRegistered?: () => void;
}

export function useRegister(options?: UseRegisterOptions) {
  const t = ES_AR;
  const navigate = useNavigate();

  return useMutation({
    mutationFn: async (data: RegisterRequest) => {
      const response = await authApi.register(data);
      options?.onRegistered?.();
      return response;
    },
    onSuccess: (_data, variables) => {
      void navigate('/verify-email-sent', { replace: true, state: { email: variables.email } });
    },
    onError: (error: unknown) => {
      options?.resetTurnstile?.();

      if (getHttpStatus(error) === 429) {
        toast.error(t.auth.rateLimitError);
        return;
      }

      const turnstileError = getTurnstileError(errorBody(error));
      if (turnstileError) {
        toast.error(turnstileError);
        return;
      }

      toast.error(getHttpErrorMessage(error, t.auth.registerError));
    },
  });
}
