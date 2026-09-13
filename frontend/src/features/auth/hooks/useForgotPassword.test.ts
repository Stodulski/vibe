import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { toast } from 'sonner';
import { createWrapper } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';

vi.mock('@/features/auth/api/auth.api', () => ({
  authApi: {
    forgotPassword: vi.fn(),
  },
}));

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

/** Mutates once with a rejected `authApi.forgotPassword` and waits for the error state. */
async function triggerForgotPasswordError(backendError: unknown, options?: { resetTurnstile?: () => void }) {
  const { authApi } = await import('@/features/auth/api/auth.api');
  vi.mocked(authApi.forgotPassword).mockRejectedValueOnce(backendError);

  const { useForgotPassword } = await import('./useForgotPassword');
  const { result } = renderHook(() => useForgotPassword(options), { wrapper: createWrapper(['/forgot-password']) });

  result.current.mutate({ email: 'juan@test.com' });

  await waitFor(() => {
    expect(result.current.isPending).toBe(false);
  });
  return result;
}

describe('useForgotPassword — Turnstile 422 handling', () => {
  it('shows the turnstile-specific message and does not read as "sent"', async () => {
    const result = await triggerForgotPasswordError(
      await makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'turnstile_token', message: 'required' }],
      }),
    );

    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileRequired);
    // Every other failure reads as "sent" (email enumeration) — a turnstile
    // failure is the one exception: the person must see it and retry.
    expect(result.current.sent).toBe(false);
  });

  it('translates every documented turnstile_token code', async () => {
    await triggerForgotPasswordError(
      await makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'turnstile_token', message: 'invalid' }],
      }),
    );
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileInvalid);

    await triggerForgotPasswordError(
      await makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'turnstile_token', message: 'unavailable' }],
      }),
    );
    expect(toast.error).toHaveBeenCalledWith(ES_AR.auth.turnstileUnavailable);
  });

  it('resets the widget on a turnstile failure — tokens are single-use', async () => {
    const resetTurnstile = vi.fn();

    await triggerForgotPasswordError(
      await makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'turnstile_token', message: 'invalid' }],
      }),
      {
        resetTurnstile,
      },
    );

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  it('also resets the widget on an unrelated failure — a solved token is burned by any failed submit', async () => {
    const resetTurnstile = vi.fn();

    await triggerForgotPasswordError(await makeConsumedHttpError(404, {}), { resetTurnstile });

    expect(resetTurnstile).toHaveBeenCalledTimes(1);
  });

  it('still reads as "sent" for a non-turnstile failure, unaffected by the turnstile check', async () => {
    const result = await triggerForgotPasswordError(await makeConsumedHttpError(404, {}));

    expect(result.current.sent).toBe(true);
    expect(toast.error).not.toHaveBeenCalled();
  });

  afterEach(async () => {
    const { authApi } = await import('@/features/auth/api/auth.api');
    vi.mocked(authApi.forgotPassword).mockReset();
  });
});
