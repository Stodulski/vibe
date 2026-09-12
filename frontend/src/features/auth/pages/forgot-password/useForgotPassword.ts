import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { HTTPError } from 'ky';
import { authApi } from '@/features/auth';
import { getHttpStatus } from '@/shared/lib/utils';
import { getTurnstileError } from '@/shared/lib/serverErrors';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** ky consumes the body before throwing, so the payload lives on `.data`. */
function errorBody(error: unknown): unknown {
  return error instanceof HTTPError ? error.data : null;
}

interface ForgotPasswordVariables {
  email: string;
  turnstile_token?: string;
}

interface UseForgotPasswordOptions {
  /** Called on every failed submit — Turnstile tokens are single-use, so the widget must reset before the next attempt. */
  resetTurnstile?: () => void;
}

export function useForgotPassword(options?: UseForgotPasswordOptions) {
  const mutation = useMutation({
    // `turnstile_token` is passed as a second positional argument only when
    // present — always passing it (even as `undefined`) would change the
    // recorded call shape from `forgotPassword(email)` to
    // `forgotPassword(email, undefined)`, which callers asserting the call
    // via `toHaveBeenCalledWith(email)` would no longer match.
    mutationFn: ({ email, turnstile_token }: ForgotPasswordVariables) =>
      turnstile_token ? authApi.forgotPassword(email, turnstile_token) : authApi.forgotPassword(email),
    onError: (error: unknown) => {
      options?.resetTurnstile?.();

      const turnstileError = getTurnstileError(errorBody(error));
      if (turnstileError) {
        toast.error(turnstileError);
        return;
      }

      if (getHttpStatus(error) === 429) {
        toast.error(t.auth.rateLimitError);
      }
      // Any other failure still reads as "sent" below — never confirm
      // whether an email address is registered (email enumeration).
    },
  });

  const turnstileError = mutation.isError ? getTurnstileError(errorBody(mutation.error)) : undefined;

  return {
    mutate: mutation.mutate,
    isPending: mutation.isPending,
    // A rate limit or a failed Turnstile challenge are the failures worth
    // surfacing to the user; every other outcome — success included —
    // reads as "sent" (never confirm whether an email address is
    // registered — email enumeration).
    sent: mutation.isSuccess || (mutation.isError && getHttpStatus(mutation.error) !== 429 && !turnstileError),
  };
}
